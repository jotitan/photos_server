package photos_server

import (
	"encoding/json"
	"errors"
	"github.com/jotitan/photos_server/common"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"github.com/jotitan/photos_server/progress"
	"github.com/jotitan/photos_server/resize"
	"io"
	"log"
	"math"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Files map[string]*Node

type FolderDto struct {
	Path        string
	Title       string
	Description string
}

// FoldersManager Store many folders
type FoldersManager struct {
	olders         map[string]*Node
	Sources        SourceNodes
	PhotosByDate   map[time.Time][]common.INode
	garbageManager *GarbageManager
	reducer        Reducer
	// Path of folder where to upload files
	UploadedFolder string
	// When upload file, override first folder in tree (to force to be in a specific one)
	//overrideUploadFolder  string
	tagManger             *TagManager
	uploadProgressManager *progress.UploadProgressManager
	nextFolderId          int
	Mirroring             Mirroring
}

func NewFoldersManager(conf config.Config, uploadProgressManager *progress.UploadProgressManager) *FoldersManager {
	fm := &FoldersManager{reducer: NewReducer(conf, []uint{1080, 250}),
		UploadedFolder:        conf.UploadedFolder,
		uploadProgressManager: uploadProgressManager}
	fm.load(conf.Sources)
	fm.updateNextFolderId()
	logger.GetLogger2().Info("Next folder id", fm.nextFolderId)
	fm.detectMissingFoldersId()
	fm.garbageManager = NewGarbageManager(conf.Garbage, conf.Security.MaskForAdmin, fm)
	fm.tagManger = NewTagManager(fm)
	fm.Mirroring = newMirroring(conf.Mirroring)
	fm.save()
	return fm
}

/*MoveFolder move folder in directory :
* Move folder with original files
* Move resized image
* Move node in folders
 */
func (fm *FoldersManager) MoveFolder(pathFrom, pathTo string) error {
	node, siblings, err := fm.FindNode(pathFrom)
	if err != nil {
		return err
	}
	r := regexp.MustCompile("[/\\\\]")

	formatPathFrom := filepath.Join(r.Split(pathFrom, -1)...)
	formatPathTo := filepath.Join(r.Split(pathTo, -1)...)

	previousFolder := node.GetAbsolutePath(fm.Sources)
	nextPath := strings.Replace(previousFolder, formatPathFrom, pathTo, -1)
	node.RelativePath = "/" + pathTo
	node.Name = filepath.Base(pathTo)

	// Move each childs
	moveEachPaths(node, formatPathFrom, formatPathTo)

	// Change tree
	fm.moveNode(pathTo, node)
	delete(siblings, filepath.Base(pathFrom))

	fm.tagManger.UpdateExistingPath(pathFrom, pathTo)
	fm.tagManger.flush()

	fm.save()

	// Move folder really (original and cache)
	err = moveSourceFolder(previousFolder, nextPath)
	if err != nil {
		return err
	}
	cacheFrom := filepath.Join(append([]string{fm.reducer.GetCache()}, r.Split(pathFrom, -1)...)...)
	cacheTo := filepath.Join(append([]string{fm.reducer.GetCache()}, r.Split(pathTo, -1)...)...)
	return moveSourceFolder(cacheFrom, cacheTo)
}

func moveSourceFolder(from, to string) error {
	log.Println("Move folder", from, to)
	// Create parent
	if err := os.MkdirAll(filepath.Dir(to), os.ModePerm); err != nil {
		return err
	}
	return os.Rename(from, to)
}

func moveEachPaths(node *Node, pathFrom, pathTo string) {
	for _, sub := range node.Files {
		sub.RelativePath = strings.Replace(sub.RelativePath, pathFrom, pathTo, -1)
		if sub.IsFolder {
			moveEachPaths(sub, pathFrom, pathTo)
		}
	}
}

func (fm *FoldersManager) moveNode(path string, node *Node) *Node {
	parent := filepath.Dir(path)
	parentNode, _, err := fm.FindNode(strings.ReplaceAll(parent, "\\", "/"))
	if err == nil {
		parentNode.Files[filepath.Base(path)] = node
		return node
	}
	// Launch on parent and create missing folder
	folder := NewFolderWithRel(filepath.Dir(node.GetAbsolutePath(fm.Sources)), "/"+parent, filepath.Base(parent), map[string]*Node{filepath.Base(path): node}, false)
	return fm.moveNode(parent, folder)
}

func (fm *FoldersManager) updateNextFolderId() {
	fm.nextFolderId = 1 + computeMaxNodeIdFromEachSource(fm.Sources)
}

func computeMaxNodeIdFromEachSource(sources SourceNodes) int {
	id := float64(0)
	for _, src := range sources {
		id = math.Max(id, float64(computeMaxNodeId(src.Files)))
	}
	return int(id)
}

func computeMaxNodeId(files Files) int {
	id := 0
	for _, node := range files {
		if node.IsFolder {
			id = int(math.Max(float64(node.Id), float64(id)))
			id = int(math.Max(float64(computeMaxNodeId(node.Files)), float64(id)))
		}
	}
	return id
}

func (fm *FoldersManager) GetAllDates() []common.NodeByDate {
	byDate := fm.GetPhotosByDate()
	dates := make([]common.NodeByDate, 0, len(byDate))
	for date, nodes := range byDate {
		dates = append(dates, common.NodeByDate{Date: date, Nb: len(nodes)})
	}
	return dates
}

func (fm *FoldersManager) resetPhotosByDate() {
	fm.PhotosByDate = nil
}

// Update exif of all photos of a specific date
func (fm *FoldersManager) updateExifOfDate(date string) (int, error) {
	if parseDate, err := time.Parse("20060102", date); err != nil {
		return 0, err
	} else {
		midnightDate := common.GetMidnightDate(parseDate)
		nodes, exist := fm.GetPhotosByDate()[midnightDate]
		logger.GetLogger2().Info("Found", len(nodes), "to update exif for", midnightDate)
		if !exist {
			return 0, errors.New("impossible to find images for this date")
		}
		for _, node := range nodes {
			n := node.(*Node)
			// extract again exif date and update node
			path := n.GetAbsolutePath(fm.Sources)
			n.Date, _ = GetExif(path)
			if n.Width == 0 {
				path := filepath.Join(fm.reducer.GetCache(), fm.GetSmallImageName(*n))
				n.Width, n.Height = resize.GetSizeAsInt(path)
			}
			logger.GetLogger2().Info("Found date", n.Date, "for path", path)
		}
		fm.save()
		return len(nodes), nil
	}
}

func (fm *FoldersManager) GetVideosByDate() map[time.Time][]*Node {
	return nil
}

func (fm *FoldersManager) GetPhotosByDate() map[time.Time][]common.INode {
	if fm.PhotosByDate == nil {
		nodes := make(map[string]common.INode)
		for _, src := range fm.Sources {
			for key, value := range src.Files {
				nodes[src.Name+"-"+key] = value
			}
		}
		fm.PhotosByDate = common.ComputeNodeByDate(nodes)
	}
	return fm.PhotosByDate
}

func (fm FoldersManager) GetSmallImageName(node Node) string {
	return fm.reducer.CreateJpegFile(filepath.Dir(node.RelativePath), node.RelativePath, fm.reducer.GetSizes()[1])
}

func (fm FoldersManager) GetMiddleImageName(node Node) string {
	return fm.reducer.CreateJpegFile(filepath.Dir(node.RelativePath), node.RelativePath, fm.reducer.GetSizes()[0])
}

var extensions = []string{"jpg", "jpeg", "png"}

// Compare old and new version of folder
// For each files in new version : search if old version exist, if true, keep information, otherwise, store new node in separate list
// To detect deletion, create a copy at beginning and remove element at each turn
func (files Files) Compare(previousFiles Files) ([]*Node, map[string]*Node, []*Node) {
	newNodes := make([]*Node, 0)
	// Nodes without changes
	noChangesNodes := make([]*Node, 0)
	nodesToDelete := make(map[string]*Node, 0)
	// First recopy old version
	deletionMap := make(map[string]*Node, len(previousFiles))
	for name, node := range previousFiles {
		deletionMap[name] = node
	}
	for name, file := range files {
		if oldValue, exist := previousFiles[name]; exist {
			// Remove element from deletionMap
			delete(deletionMap, name)
			if !file.IsFolder {
				file.Height = oldValue.Height
				file.Width = oldValue.Width
				file.Date = oldValue.Date
				file.ImagesResized = oldValue.ImagesResized
				noChangesNodes = append(noChangesNodes, oldValue)
			} else {
				// Relaunch on folder
				delta, deletions, noChanges := file.Files.Compare(oldValue.Files)
				for _, node := range delta {
					newNodes = append(newNodes, node)
				}
				for name, node := range deletions {
					nodesToDelete[name] = node
				}
				for _, node := range noChanges {
					noChangesNodes = append(noChangesNodes, node)
				}
			}
		} else {
			// Treat folder
			if !file.IsFolder {
				newNodes = append(newNodes, file)
			} else {
				delta, deletions, noChanges := file.Files.Compare(Files{})
				for _, node := range delta {
					newNodes = append(newNodes, node)
				}
				for name, node := range deletions {
					nodesToDelete[name] = node
				}
				for _, node := range noChanges {
					noChangesNodes = append(noChangesNodes, node)
				}
			}
		}
	}
	// Add local nodes to delete with other
	for name, node := range deletionMap {
		nodesToDelete[name] = node
	}
	return newNodes, nodesToDelete, noChangesNodes
}

// Add a locker to check if an update is running
var updateLocker = sync.Mutex{}

// UpdateFolder : only update one folder
func (fm *FoldersManager) UpdateFolder(path string, progresser *progress.UploadProgress) error {
	if node, _, err := fm.FindNode(path); err != nil {
		return err
	} else {
		rootFolder := fm.Sources.getSourceFolder(node.RelativePath)
		files := fm.Analyse(rootFolder, node.GetAbsolutePath(fm.Sources))
		// Take the specific folder
		if t, exists := files[filepath.Base(path)]; exists {
			files = t.Files
		}
		fm.compareAndCleanFolder(files, path, make(map[string]*Node), progresser)
		node.Files = files
		fm.save()
		return nil
	}
}

func getOnlyElementFromMap(files Files) *Node {
	if len(files) != 1 {
		return nil
	}
	for _, n := range files {
		return n
	}
	return nil
}

func (fm *FoldersManager) UpdateExif(path string, forceSize bool) error {
	if node, _, err := fm.FindNode(path); err != nil {
		return err
	} else {
		rootFolder := fm.Sources.getSourceFolder(node.RelativePath)
		files := fm.Analyse(rootFolder, node.GetAbsolutePath(fm.Sources))
		// Is first node is a folder, get files inside
		if folderNode := getOnlyElementFromMap(files); folderNode != nil && folderNode.IsFolder {
			_, _, noChanges := folderNode.Files.Compare(node.Files)
			for _, file := range noChanges {
				datePhoto, _ := GetExif(file.GetAbsolutePath(fm.Sources))
				file.Date = datePhoto
				if forceSize || file.Width == 0 {
					path := filepath.Join(fm.reducer.GetCache(), fm.GetSmallImageName(*file))
					file.Width, file.Height = resize.GetSizeAsInt(path)
					logger.GetLogger2().Info("Update exif size", path, file.Width, file.Height)
				}
			}
			fm.save()
			return nil
		} else {
			return errors.New("impossible to update exif")
		}
	}
}

// If folderpath not empty, compare only in this folder
func (fm *FoldersManager) compareAndCleanFolder(files Files, folderPath string, newFolders map[string]*Node, progresser *progress.UploadProgress) {

	// Include dry run and full (compare length a nodes or compare always everything)
	folders := files
	if !strings.EqualFold("", folderPath) {
		if node, _, err := fm.FindNode(folderPath); err == nil {
			folders = node.Files
		}
	}
	delta, deletions, noChanges := files.Compare(folders)
	existings := fm.searchExistingReducedImages(folderPath)
	logger.GetLogger2().Info("After update", len(delta), "new pictures and", len(deletions), "to remove and no changes", len(noChanges), "(existings :", len(existings), ")")
	logger.GetLogger2().Info(existings)
	// Launch indexation of new images,
	if len(delta) > 0 {
		progresser.EnableWaiter()
		progresser.Add(len(delta))
		for _, node := range delta {
			absolutePath := node.GetAbsolutePath(fm.Sources)
			logger.GetLogger2().Info("Launch update image resize", absolutePath)
			fm.reducer.AddImage(absolutePath, node.RelativePath, node, progresser, existings, false)
		}
		progresser.Wait()
		logger.GetLogger2().Info("All pictures have been resized")
	}

	// remove deletions in cache
	fm.removeFiles(deletions)
	for name, f := range files {
		newFolders[name] = f
	}
	progresser.End()
}

// Update : compare structure in memory and folder to detect changes
func (fm *FoldersManager) Update() error {
	// Have to block to avoid second update in same time
	// A lock is blocking, so use a chanel tiomeout to check if locker is still block
	updateWaiter := sync.WaitGroup{}
	updateWaiter.Add(1)
	begin := time.Now()
	chanelRunning := make(chan struct{}, 1)
	up := fm.uploadProgressManager.AddUploader(0)
	go func() {
		// Is still block after one second, method exit and go routine is never executed
		updateLocker.Lock()
		chanelRunning <- struct{}{}
		// Stop the thread when get the lock after stop
		if time.Now().Sub(begin) > time.Duration(100)*time.Millisecond {
			return
		}
		time.Sleep(time.Second)
		// For each folder, launch an analyse and detect differences
		for _, src := range fm.Sources {
			newFolders := make(map[string]*Node, len(src.Files))
			for _, folder := range src.Files {
				absolutePath := folder.GetAbsolutePath(fm.Sources)
				rootFolder := filepath.Dir(absolutePath)
				files := fm.Analyse(rootFolder, absolutePath)
				fm.compareAndCleanFolder(files, "", newFolders, up)
			}
			src.Files = newFolders
		}
		fm.save()
		updateWaiter.Done()
		updateLocker.Unlock()
	}()

	// Detect if an update is already running. Is true (after one secon), return Error, otherwise, wait for End of update
	select {
	case <-chanelRunning:
		updateWaiter.Wait()
		return nil
	case <-time.NewTimer(time.Millisecond * 100).C:
		return errors.New("an update is already running")
	}
}

// RemoveNode : oOnly remove the node in tree, not the file
func (fm *FoldersManager) RemoveNode(path string) error {
	if node, parent, err := fm.FindNode(path); err != nil {
		return err
	} else {
		// Remove only if folder is empty
		if len(node.Files) > 0 {
			return errors.New("impossible to remove not empty folder")
		}
		delete(parent, node.Name)
		fm.save()
	}
	return nil
}

// FindNodes return details of folders
func (fm FoldersManager) FindNodes(paths []string) []FolderDto {
	results := make([]FolderDto, len(paths))
	for i, path := range paths {
		if node, _, err := fm.FindNode(path); err == nil {
			results[i] = FolderDto{Path: path, Title: node.Name, Description: node.Description}
		} else {
			results[i] = FolderDto{}
		}
	}
	return results
}

func (fm FoldersManager) FindOrCreateNode(path string) (*Node, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	source, subPath, err := fm.Sources.getSourceFromPath(path)
	// Source not found
	if err != nil {
		return nil, err
	}
	if node, _, err := findNodeFromList(source.Files, subPath); err == nil {
		return node, nil
	}
	// Search the node
	if strings.Index(subPath, "/") == -1 {
		// add to the source
		node := &Node{RelativePath: createRelativePath(source.Name, subPath), Name: subPath, Files: make(map[string]*Node), Id: fm.GetNextId()}
		source.Files[subPath] = node
		return node, nil
	}
	parent, _ := fm.FindOrCreateNode(filepath.Dir(path))
	var node = &Node{Name: filepath.Base(subPath), RelativePath: createRelativePath(source.Name, subPath), Id: fm.GetNextId(), Files: make(map[string]*Node)}
	parent.Files[node.Name] = node
	parent.IsFolder = true

	return node, nil
}

func createRelativePath(source, path string) string {
	return strings.ReplaceAll(source+"/"+path, "\\", "/")
}

func (fm FoldersManager) FindNode(path string) (*Node, map[string]*Node, error) {
	source, subPath, err := fm.Sources.getSourceFromPath(path)
	// Source not found
	if err != nil {
		return nil, nil, err
	}
	return findNodeFromList(source.Files, subPath)
}

func findNodeFromList(current map[string]*Node, path string) (*Node, map[string]*Node, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	nbSub := strings.Count(path, "/")
	if nbSub == 0 {
		if node, ok := current[path]; ok {
			return node, current, nil
		}
		return nil, nil, errors.New("Impossible to find node " + path)
	}
	for pos, sub := range strings.Split(path, "/") {
		node := current[sub]
		if node == nil {
			return nil, nil, errors.New("Impossible to find node " + path)
		}
		if node.IsFolder {
			if pos == nbSub {
				return node, current, nil
			}
			current = current[sub].Files
		} else {
			// If not last element
			if pos == nbSub {
				// Last element, return it
				return node, current, nil
			} else {
				// Impossible to continue
				return nil, nil, errors.New("Impossible to found node " + path)
			}
		}
	}
	return nil, nil, errors.New("Impossible to find path" + path)
}

func (fm FoldersManager) removeFiles(files map[string]*Node) {
	for _, node := range files {
		fm.removeFilesNode(node)
	}
}

func (fm FoldersManager) removeFilesNode(node *Node) error {
	if err := fm.removeFile(filepath.Join(fm.reducer.GetCache(), fm.GetSmallImageName(*node))); err == nil {
		return fm.removeFile(filepath.Join(fm.reducer.GetCache(), fm.GetMiddleImageName(*node)))
	} else {
		return err
	}
}

func (fm FoldersManager) removeFile(path string) error {
	logger.GetLogger2().Info("Remove file", path)
	return os.Remove(path)
}

func (fm *FoldersManager) AddFolderToNode(absolutePath, relativePath string, forceRotate bool, detail detailUploadFolder, p *progress.UploadProgress) error {
	// Get or create the good code inserted info tree
	node, err := fm.FindOrCreateNode(relativePath)
	if err != nil {
		return err
	}
	src, _, _ := fm.Sources.getSourceFromPath(relativePath)
	rootFolder := src.GetSourceFolder()
	// Return always one node
	_, nodeWithFiles := fm.AnalyseAsOne(rootFolder, absolutePath)
	if nodeWithFiles == nil {
		logger.GetLogger2().Error("Impossible to have more that one node")
		return errors.New("impossible to have more that one node")
	}
	// Update metadata
	node.Files = nodeWithFiles.Files
	node.IsFolder = true
	node.Title = detail.title
	node.Description = detail.description

	// Define id if not exist in subtree
	fm.detectMissingFoldersIdOfFolder(node.Files)

	logger.GetLogger2().Info("Add folder", relativePath)
	// Check if images already exists to improve computing
	existings := fm.searchExistingReducedImages(node.RelativePath)
	logger.GetLogger2().Info("Found existing", len(existings))
	p.EnableWaiter()

	fm.launchImageResize(node, detail.source, p, existings, forceRotate)

	fm.save()
	return nil
}

// GetNextId return the current id and increase it
func (fm *FoldersManager) GetNextId() int {
	id := fm.nextFolderId
	fm.nextFolderId++
	return id
}

func (fm *FoldersManager) searchExistingReducedImages(folderPath string) map[string]struct{} {
	// Find the folder in cache
	folder := filepath.Join(fm.reducer.GetCache(), folderPath)
	tree := fm.Analyse(fm.reducer.GetCache(), folder)
	// Browse all files
	files := make(map[string]struct{})
	for _, node := range tree {
		for file, value := range extractReducedImages(node, fm.reducer.GetCache()) {
			files[file] = value
		}
	}
	return files
}

func extractReducedImages(node *Node, cacheFolder string) map[string]struct{} {
	m := make(map[string]struct{})
	if node.IsFolder {
		for _, subNode := range node.Files {
			for file := range extractReducedImages(subNode, cacheFolder) {
				m[file] = struct{}{}
			}
		}
	} else {
		m[filepath.Join(cacheFolder, node.RelativePath)] = struct{}{}
	}
	return m
}

func extractImages(node *Node, sn SourceNodes) map[string]struct{} {
	m := make(map[string]struct{})
	if node.IsFolder {
		for _, subNode := range node.Files {
			for file := range extractImages(subNode, sn) {
				m[file] = struct{}{}
			}
		}
	} else {
		m[node.GetAbsolutePath(sn)] = struct{}{}
	}
	return m
}

func (fm *FoldersManager) detectMissingFoldersIdOfFolder(folders map[string]*Node) int {
	counter := 0
	for _, folder := range folders {
		if folder.IsFolder {
			if folder.Id == 0 {
				// Need to define folder
				counter++
				folder.Id = fm.GetNextId()
			}
			// Browse subfolders
			counter += fm.detectMissingFoldersIdOfFolder(folder.Files)
		}
	}
	return counter
}

// Detect folders without id, create if necessary
func (fm *FoldersManager) detectMissingFoldersId() {
	counter := 0
	for _, src := range fm.Sources {
		counter += fm.detectMissingFoldersIdOfFolder(src.Files)
	}
	if counter != 0 {
		// Save configuration to keep new ids
		logger.GetLogger2().Info("Save new ids of folder", counter, ". Next folder id is", fm.nextFolderId)
		fm.save()
	}
}

func (fm *FoldersManager) load(sources []config.Source) {
	folders := make(map[string]*SourceNode, 0)
	if f, err := os.Open(getSavePath()); err == nil {
		defer f.Close()
		data, _ := io.ReadAll(f)
		log.Println(json.Unmarshal(data, &folders))
		fm.Sources = folders
		// Check if new sources are available
		for _, source := range sources {
			if src, exists := fm.Sources[source.Name]; !exists {
				fm.Sources[source.Name] = &SourceNode{Name: source.Name, Folder: source.Folder, Files: make(map[string]*Node)}
			} else {
				if src.Files == nil {
					src.Files = make(map[string]*Node)
				}
			}
		}
	} else {
		logger.GetLogger2().Error("Impossible to read saved config", getSavePath(), err)
		// Initialize folders with sources if exists
		if len(sources) > 0 {
			for _, source := range sources {
				folders[source.Name] = &SourceNode{Name: source.Name, Folder: source.Folder, Files: make(map[string]*Node)}
			}
			fm.Sources = folders
		}
	}
}

func getSavePath() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "save-images.json")
}

type detailUploadFolder struct {
	source      string
	path        string
	title       string
	description string
}

// folder must be a relative path
// UploadFolder addToFolder, if true, can add photos in existing folder
func (fm *FoldersManager) UploadFolder(detail detailUploadFolder, files []multipart.File, names []string, addToFolder bool) (*progress.UploadProgress, error) {
	if len(files) != len(names) {
		return nil, errors.New("error during upload")
	}
	src, err := fm.Sources.getSource(detail.source)
	if err != nil {
		return nil, err
	}
	// Check no double dots to move info tree
	if strings.Contains(detail.path, "..") {
		return nil, errors.New("too dangerous relative path folder with .. inside")
	}

	outputFolder := filepath.Join(src.Folder, detail.path)
	if addToFolder {
		// if already exists, source if already in the path
		if node, _, err := fm.FindNode(detail.path); err != nil {
			return nil, err
		} else {
			outputFolder = node.GetAbsolutePath(fm.Sources)
		}
	} else {
		if err := createFolderIfExistOrFail(outputFolder); err != nil {
			return nil, err
		}
	}
	// Create work in go routine and return a progresser status
	progresser := fm.uploadProgressManager.AddUploader(len(files))
	go fm.doUploadFolder(detail, outputFolder, names, files, addToFolder, progresser)
	return progresser, nil
}

func (fm *FoldersManager) copyImagesInFolder(names []string, files []multipart.File, folder string, detail detailUploadFolder, p *progress.UploadProgress) error {
	for i, file := range files {
		imagePath := filepath.Join(folder, names[i])
		if imageFile, err := os.OpenFile(imagePath, os.O_CREATE|os.O_RDWR, os.ModePerm); err == nil {
			if _, err := io.Copy(imageFile, file); err != nil {
				// Send Error to progresser and stop
				return err
			}
			imageFile.Close()
			if err = fm.Mirroring.copy(imagePath, filepath.Join(detail.source, detail.path, names[i])); err != nil {
				return err
			}
			p.Done()
		} else {
			return err
		}
	}
	return nil
}

func (fm *FoldersManager) doUploadFolder(detail detailUploadFolder, outputFolder string, names []string, files []multipart.File, addToFolder bool, p *progress.UploadProgress) {
	// Copy files on filer
	if err := fm.copyImagesInFolder(names, files, outputFolder, detail, p); err != nil {
		p.Error(err)
		return
	}

	// Use default source to add folder in a specific folder by default, not in root. Resize will be in default-source and path also
	logger.GetLogger2().Info("Folder", detail.path, "well uploaded with", len(files), "files")
	// If photos added in existing folder, update folder, otherwise, index
	if addToFolder {
		if err := fm.UpdateFolder(detail.path, p); err != nil {
			p.Error(err)
		}
		return
	}
	// Launch add folder with input folder, node path
	if err := fm.AddFolderToNode(outputFolder, detail.source+"/"+detail.path, false, detail, p); err != nil {
		p.Error(err)
	}
}

func createFolderIfExistOrFail(path string) error {
	if d, err := os.Open(path); err == nil {
		d.Close()
		return errors.New("folder already exists, must be new (" + path + ")")
	}
	return os.MkdirAll(path, os.ModePerm)
}

func (fm *FoldersManager) save() {
	fm.resetPhotosByDate()
	data, _ := json.Marshal(fm.Sources)
	if f, err := os.OpenFile(getSavePath(), os.O_TRUNC|os.O_CREATE|os.O_RDWR, os.ModePerm); err == nil {
		defer f.Close()
		f.Write(data)
		logger.GetLogger2().Info("Save tree in file", getSavePath())
	} else {
		logger.GetLogger2().Error("Impossible to save tree in file", getSavePath())
	}
}

func (fm *FoldersManager) launchImageResize(folder *Node, source string, p *progress.UploadProgress, existings map[string]struct{}, forceRotate bool) {
	folder.applyOnEach(fm.Sources, func(absolutePath, relativePath string, node *Node) {
		p.Add(1)
		// Override relative path to include source
		fm.reducer.AddImage(absolutePath, node.RelativePath, node, p, existings, forceRotate)
	})
	go func(node *Node) {
		p.Wait()
		p.End()
		logger.GetLogger2().Info("End of resize folder", folder.Name)
		node.ImagesResized = true
	}(folder)
}

func (fm FoldersManager) AnalyseAsOne(rootFolder, path string) (string, *Node) {
	files := fm.Analyse(rootFolder, path)
	if len(files) == 1 {
		for name, node := range files {
			return name, node
		}
	}
	return "", nil
}

// Analyse a cache and detect all files of types images
func (fm FoldersManager) Analyse(rootFolder, path string) Files {
	if file, err := os.Open(path); err == nil {
		defer file.Close()
		// If cache, create cache and go deep
		if stat, errStat := file.Stat(); errStat == nil {
			if stat.IsDir() {
				return fm.treatFolder(rootFolder, path, stat.Name(), file)
			} else {
				return fm.treatImage(rootFolder, path, stat.Name())
			}
		}
	}
	return Files{}
}

func (fm FoldersManager) treatImage(rootFolder, path, name string) map[string]*Node {
	// Test if is image
	if isImage(name) {
		return createSimpleMap(name, NewImage(rootFolder, path, name))
	}
	return Files{}
}

func (fm FoldersManager) treatFolder(rootFolder, path, name string, file *os.File) map[string]*Node {
	files, _ := file.Readdirnames(-1)
	nodes := make(map[string]*Node, 0)
	for _, file := range files {
		for name, node := range fm.Analyse(rootFolder, filepath.Join(path, file)) {
			nodes[name] = node
		}
	}
	if len(nodes) > 0 {
		// If folder already exists, get informations from existing node (title, description...)
		//folder := NewFolder(rootFolder, path, name, nodes, false)
		//fm.searchAndImproveNode(folder)
		return createSimpleMap(name, NewFolder(rootFolder, path, name, nodes, false))
	}
	return Files{}
}

func createSimpleMap(name string, node *Node) map[string]*Node {
	return map[string]*Node{name: node}
}

func (fm FoldersManager) List() []*Node {
	nodes := make([]*Node, 0, fm.Sources.countFolders())
	for _, src := range fm.Sources {
		for name, folder := range src.Files {
			nodes = append(nodes, NewFolder("", name, name, nil, folder.ImagesResized))
		}
	}
	return nodes
}

func (fm *FoldersManager) Browse(path string) ([]*Node, *Node, error) {
	if len(path) < 2 {
		// Return list
		return fm.List(), nil, nil

	} else {
		node, err := fm.browsePaths(path)
		if err != nil {
			return nil, nil, err
		}
		// Parse file of nodes
		nodes := make([]*Node, 0, len(node.Files))
		for _, file := range node.Files {
			nodes = append(nodes, file)
		}
		return nodes, node, nil
	}
}

func (fm *FoldersManager) browsePaths(path string) (*Node, error) {
	var node *Node
	var exist bool
	// Browse path
	src, subPath, err := fm.Sources.getSourceFromPath(path)
	if err != nil {
		return nil, err
	}
	for i, folder := range strings.Split(subPath, "/") {
		if i == 0 {
			if node, exist = src.Files[folder]; !exist {
				return nil, errors.New("Invalid path " + folder)
			}
		} else {
			if !strings.EqualFold("", strings.Trim(folder, " ")) {
				if !node.IsFolder {
					return nil, errors.New("Not a valid cache " + folder)
				}
				node = node.Files[folder]
			}
		}
	}
	return node, nil
}

func (fm *FoldersManager) Count() int {
	count := 0
	for _, nodes := range fm.GetPhotosByDate() {
		count += len(nodes)
	}
	return count
}

func (fm *FoldersManager) UpdateDetails(details FolderDto) error {
	if node, _, err := fm.FindNode(details.Path); err == nil {
		node.Title = details.Title
		node.Description = details.Description
		fm.save()
		return nil
	} else {
		return err
	}
}

func isImage(name string) bool {
	for _, suffix := range extensions {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			return true
		}
	}
	return false
}
