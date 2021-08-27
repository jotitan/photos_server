package photos_server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jotitan/photos_server/common"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"github.com/jotitan/photos_server/progress"
	"io"
	"io/ioutil"
	"math"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)


type Files map[string]*Node

type Node struct {
	AbsolutePath string
	// Path of node relative to head
	RelativePath string
	Width        int
	Height       int
	Date         time.Time
	Name         string
	IsFolder     bool
	// Store files in a map with name
	Files         Files
	ImagesResized bool
	Id            int	`json:"id,omitempty"`
}

func (n Node)GetDate()time.Time{
	return n.Date
}

func (n Node)GetIsFolder()bool {
	return n.IsFolder
}

func (n Node)GetFiles()map[string]common.INode{
	nodes := make(map[string]common.INode,len(n.Files))
	for key,value := range n.Files {
		nodes[key] = value
	}
	return nodes
}

func (n Node)applyOnEach(rootFolder string, fct func(path,relativePath string, node * Node)){
	for _,file := range n.Files{
		if file.IsFolder{
			file.applyOnEach(rootFolder,fct)
		}else{
			fct(file.AbsolutePath,file.RelativePath,file)
		}
	}
}

func (n Node)String()string{
	return fmt.Sprintf("%s : %s : %t : %s : %s",n.AbsolutePath,n.RelativePath,n.ImagesResized,n.Name,n.Files)
}

func NewImage(rootFolder,path,name string)*Node{
	relativePath := strings.Replace(path,rootFolder,"",-1)
	return &Node{AbsolutePath:path,RelativePath:relativePath,Name:name,IsFolder:false,Files:nil,ImagesResized:false}
}

func NewFolder(rootFolder,path,name string,files Files, imageResized bool)*Node{
	relativePath := strings.Replace(strings.Replace(path,rootFolder,"",-1),"\\","/",-1)
	return &Node{AbsolutePath:path,RelativePath:relativePath,Name:name,IsFolder:true,Files:files,ImagesResized:imageResized}
}

// Store many folders
type FoldersManager struct{
	Folders map[string]*Node
	PhotosByDate map[time.Time][]common.INode
	garbageManager * GarbageManager
	reducer Reducer
	// Path of folder where to upload files
	UploadedFolder string
	// When upload file, override first folder in tree (to force to be in a specific one)
	overrideUploadFolder string
	tagManger * TagManager
	uploadProgressManager *progress.UploadProgressManager
	nextFolderId int
}

func NewFoldersManager(conf config.Config, uploadProgressManager*progress.UploadProgressManager)*FoldersManager {
	fm := &FoldersManager{Folders: make(map[string]*Node,0),reducer:NewReducer(conf,[]uint{1080,250}),
		UploadedFolder:conf.UploadedFolder,overrideUploadFolder:conf.OverrideUploadFolder,
		uploadProgressManager:uploadProgressManager}
	fm.load()
	fm.updateNextFolderId()
	fm.detectMissingFoldersId()
	fm.garbageManager = NewGarbageManager(conf.Garbage,conf.Security.MaskForAdmin,fm)
	fm.tagManger = NewTagManager(fm)
	return fm
}

func (fm *FoldersManager)updateNextFolderId(){
	id := 0
	for _,node := range fm.Folders {
		if node.IsFolder {
			id = int(math.Max(float64(node.Id),float64(id)))
		}
	}
	fm.nextFolderId = id+1
}

func (fm *FoldersManager)GetAllDates()[]common.NodeByDate {
	byDate := fm.GetPhotosByDate()
	dates := make([]common.NodeByDate,0,len(byDate))
	for date,nodes := range byDate {
		dates = append(dates, common.NodeByDate{Date: date,Nb:len(nodes)})
	}
	return dates
}

func (fm *FoldersManager)resetPhotosByDate(){
	fm.PhotosByDate = nil
}

// Update exif of all photos of a specific date
func (fm *FoldersManager)updateExifOfDate(date string)(int,error){
	if parseDate,err := time.Parse("20060102",date) ; err != nil {
		return 0,err
	}else {
		midnightDate := common.GetMidnightDate(parseDate)
		nodes,exist := fm.GetPhotosByDate()[midnightDate]
		logger.GetLogger2().Info("Found",len(nodes),"to update exif for",midnightDate)
		if !exist {
			return 0,errors.New("impossible to find images for this date")
		}
		for _,node := range nodes {
			// extract again exif date and update node
			node.(*Node).Date,_ = GetExif(node.(*Node).AbsolutePath)
			logger.GetLogger2().Info("Found date",node.(*Node).Date,"for path",node.(*Node).AbsolutePath)
		}
		fm.save()
		return len(nodes),nil
	}
}

func (fm *FoldersManager)GetVideosByDate()map[time.Time][]*Node{
	return nil
}

func (fm *FoldersManager)GetPhotosByDate()map[time.Time][]common.INode{
	if fm.PhotosByDate == nil {
		nodes := make(map[string]common.INode,len(fm.Folders))
		for key,value := range fm.Folders{
			nodes[key] = value
		}
		fm.PhotosByDate = common.ComputeNodeByDate(nodes)
	}
	return fm.PhotosByDate
}

func (fm FoldersManager) GetSmallImageName(node Node)string{
	return fm.reducer.createJpegFile(filepath.Dir(node.RelativePath),node.RelativePath,fm.reducer.sizes[1])
}

func (fm FoldersManager) GetMiddleImageName(node Node)string{
	return fm.reducer.createJpegFile(filepath.Dir(node.RelativePath),node.RelativePath,fm.reducer.sizes[0])
}

var extensions = []string{"jpg","jpeg","png"}

// Compare old and new version of folder
// For each files in new version : search if old version exist, if true, keep information, otherwise, store new node in separate list
// To detect deletion, create a copy at beginning and remove element at each turn
func (files Files)Compare(previousFiles Files)([]*Node,map[string]*Node,[]*Node){
	newNodes := make([]*Node,0)
	// Nodes without changes
	noChangesNodes := make([]*Node,0)
	nodesToDelete := make(map[string]*Node,0)
	// First recopy old version
	deletionMap := make(map[string]*Node,len(previousFiles))
	for name,node := range previousFiles {
		deletionMap[name] = node
	}
	for name,file := range files {
		if oldValue, exist := previousFiles[name]; exist {
			// Remove element from deletionMap
			delete(deletionMap,name)
			if !file.IsFolder {
				file.Height = oldValue.Height
				file.Width = oldValue.Width
				file.ImagesResized = oldValue.ImagesResized
				noChangesNodes = append(noChangesNodes,oldValue)
			}else{
				// Relaunch on folder
				delta,deletions,noChanges := file.Files.Compare(oldValue.Files)
				for _,node := range delta {
					newNodes = append(newNodes,node)
				}
				for name,node := range deletions {
					nodesToDelete[name] = node
				}
				for _,node := range noChanges {
					noChangesNodes = append(noChangesNodes,node)
				}
			}
		}else{
			// Treat folder
			if !file.IsFolder {
				newNodes = append(newNodes,file)
			}else{
				delta,deletions,noChanges := file.Files.Compare(Files{})
				for _,node := range delta {
					newNodes = append(newNodes,node)
				}
				for name,node := range deletions {
					nodesToDelete[name] = node
				}
				for _,node := range noChanges {
					noChangesNodes = append(noChangesNodes,node)
				}
			}
		}
	}
	// Add local nodes to delete with other
	for name,node := range deletionMap {
		nodesToDelete[name] = node
	}
	return newNodes,nodesToDelete,noChangesNodes
}

// Add a locker to check if an update is running
var updateLocker = sync.Mutex{}

// Only update one folder
func (fm *FoldersManager)UpdateFolder(path string,progresser *progress.UploadProgress)error{
	if node,_,err := fm.FindNode(path) ; err != nil {
		return err
	}else {
		rootFolder := node.AbsolutePath[:len(node.AbsolutePath)-len(node.RelativePath)]
		files := fm.Analyse(rootFolder, node.AbsolutePath)
		// Take the specific folder
		files = files[filepath.Base(path)].Files
		fm.compareAndCleanFolder(files,path,make(map[string]*Node),progresser)
		node.Files = files
		fm.save()
		return nil
	}
}

func getOnlyElementFromMap(files Files)*Node{
	if len(files) != 1 {
		return nil
	}
	for _,n := range files {
		return n
	}
	return nil
}

func (fm *FoldersManager)UpdateExif(path string)error {
	if node,_,err := fm.FindNode(path) ; err != nil {
		return err
	}else {
		rootFolder := node.AbsolutePath[:len(node.AbsolutePath)-len(node.RelativePath)]
		files := fm.Analyse(rootFolder, node.AbsolutePath)
		// Is first node is a folder, get files inside
		if folderNode := getOnlyElementFromMap(files) ; folderNode != nil && folderNode.IsFolder {
			_,_, noChanges := folderNode.Files.Compare(node.Files)
			for _,file := range noChanges {
				datePhoto,_ := GetExif(file.AbsolutePath)
				file.Date = datePhoto
			}
			fm.save()
			return nil
		}else{
			return errors.New("impossible to update exif")
		}
	}
}

// If folderpath not empty, compare only in this folder
func (fm *FoldersManager)compareAndCleanFolder(files Files,folderPath string,newFolders map[string]*Node, progresser *progress.UploadProgress){

	// Include dry run and full (compare length a nodes or compare always everything)
	folders := fm.Folders
	if !strings.EqualFold("",folderPath) {
		if node,_,err := fm.FindNode(folderPath) ; err == nil {
			folders = node.Files
		}
	}
	delta, deletions,noChanges := files.Compare(folders)
	logger.GetLogger2().Info("After update", len(delta), "new pictures and", len(deletions), "to remove and no changes",len(noChanges))
	// Launch indexation of new images,
	if len(delta) > 0 {
		progresser.EnableWaiter()
		progresser.Add(len(delta))
		for _, node := range delta {
			logger.GetLogger2().Info("Launch update image resize", node.AbsolutePath)
			fm.reducer.AddImage(node.AbsolutePath, node.RelativePath, "",node, progresser,make(map[string]struct{}),false)
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

//Update : compare structure in memory and folder to detect changes
func (fm *FoldersManager)Update()error{
	// Have to block to avoid second update in same time
	// A lock is blocking, so use a chanel tiomeout to check if locker is still block
	updateWaiter := sync.WaitGroup{}
	updateWaiter.Add(1)
	begin := time.Now()
	chanelRunning := make(chan struct{},1)
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
		newFolders := make(map[string]*Node, len(fm.Folders))
		for _, folder := range fm.Folders {
			rootFolder := filepath.Dir(folder.AbsolutePath)
			files := fm.Analyse(rootFolder, folder.AbsolutePath)
			fm.compareAndCleanFolder(files,"",newFolders,up)
		}
		fm.Folders = newFolders
		fm.save()
		updateWaiter.Done()
		updateLocker.Unlock()
	}()

	// Detect if an update is already running. Is true (after one secon), return Error, otherwise, wait for End of update
	select {
	case <- chanelRunning :
		updateWaiter.Wait()
		return nil
	case <- time.NewTimer(time.Millisecond*100).C:
		return errors.New("an update is already running")
	}
}

// Only remove the node in tree, not the file
func (fm *FoldersManager)RemoveNode(path string)error{
	if node, parent, err := fm.FindNode(path) ; err != nil {
		return err
	}else{
		// Remove only if folder is empty
		if len(node.Files) > 0{
			return errors.New("impossible to remove not empty folder")
		}
		delete(parent,node.Name)
		fm.save()
	}
	return nil
}

func (fm FoldersManager)FindNode(path string)(*Node,map[string]*Node,error){
	current := fm.Folders
	nbSub := strings.Count(path,"/")
	if nbSub == 0{
		if node,ok := fm.Folders[path] ; ok {
			return node,fm.Folders,nil
		}
		return nil,nil,errors.New("Impossible to find node " + path)
	}
	for pos,sub := range strings.Split(path,"/") {
		node := current[sub]
		if node == nil {
			return nil,nil,errors.New("Impossible to find node " + path)
		}
		if node.IsFolder {
			if pos == nbSub {
				return node,current,nil
			}
			current = current[sub].Files
		}else{
			// If not last element
			if pos == nbSub {
				// Last element, return it
				return node,current,nil
			}else{
				// Impossible to continue
				return nil,nil,errors.New("Impossible to found node " + path)
			}
		}
	}
	return nil,nil,errors.New("Bad path " + path)
}

func (fm FoldersManager)removeFiles(files map[string]*Node){
	for _,node := range files {
		fm.removeFilesNode(node)
	}
}

func (fm FoldersManager)removeFilesNode( node * Node)error{
	if err := fm.removeFile(filepath.Join(fm.reducer.cache,fm.GetSmallImageName(*node))) ; err == nil {
		return fm.removeFile(filepath.Join(fm.reducer.cache,fm.GetMiddleImageName(*node)))
	}else{
		return err
	}
}

func (fm FoldersManager)removeFile(path string)error{
	logger.GetLogger2().Info("Remove file",path)
	return os.Remove(path)
}

// used when upload
// @overrideOutput override default output folder by adding inside a path folder
// @forceRelativePath is true, use relativePath as real relative of new node
func (fm *FoldersManager)AddFolderToNode(folderPath,relativePath,overrideOutput string,forceRotate,forceRelativePath bool,p *progress.UploadProgress)error{
	// Compute relative path
	rootFolder := filepath.Dir(relativePath)
	if strings.EqualFold("",rootFolder) || strings.EqualFold(".",rootFolder) {
		// Add folder as usual (new one)
		fm.AddFolder(folderPath,forceRotate,p)
		return nil
	}
	// Find the node of root folder
	if node,_,err := fm.FindNode(rootFolder) ; err == nil {
		if forceRelativePath {
			// Override rootFolder
			root := folderPath[0:len(folderPath)-len(relativePath)]
			fm.AddFolderWithNode(node.Files,root,folderPath,overrideOutput,forceRotate,p)
		}else {
			fm.AddFolderWithNode(node.Files, fm.UploadedFolder, folderPath, overrideOutput, forceRotate,p)
		}
	}else{
		// Add the parent folder (which is recursive)
		return fm.AddFolderToNode(filepath.Dir(folderPath),rootFolder,overrideOutput,forceRotate,forceRelativePath,p)
	}
	return nil
}

func (fm *FoldersManager)AddFolder(folderPath string,forceRotate bool,p *progress.UploadProgress){
	fm.AddFolderWithNode(fm.Folders,"",folderPath,"",forceRotate,p)
}

func (fm *FoldersManager)AddFolderWithNode(files Files,rootFolder,folderPath,overrideOutput string,forceRotate bool,p *progress.UploadProgress){
	if strings.EqualFold("",rootFolder) {
		rootFolder = filepath.Dir(folderPath)
	}
	// Return always one node
	name,node := fm.AnalyseAsOne(rootFolder,folderPath)
	if node == nil {
		logger.GetLogger2().Error("Impossible to have more that one node")
		return
	}
	logger.GetLogger2().Info("Add folder",folderPath)
	// Check if images already exists to improve computing
	existings := fm.searchExistingReducedImages(folderPath)
	logger.GetLogger2().Info("Found existing",len(existings))
	p.EnableWaiter()
	files[name] = node
	fm.launchImageResize(node,strings.Replace(folderPath,name,"",-1),overrideOutput,p,existings,forceRotate)

	go func(){
		p.Wait()
		p.End()
		logger.GetLogger2().Info("End of resize folder",node.Name)
		node.ImagesResized=true
	}()
	fm.save()
}


func (fm *FoldersManager) searchExistingReducedImages(folderPath string)map[string]struct{}{
	// Find the folder in cache
	folder := filepath.Join(fm.reducer.cache,filepath.Base(folderPath))
	tree := fm.Analyse(fm.reducer.cache,folder)
	// Browse all files
	files := make(map[string]struct{})
	for _,node := range tree {
		for file,value := range extractImages(node) {
			files[file] = value
		}
	}
	return files
}

func extractImages(node *Node)map[string]struct{}{
	m := make(map[string]struct{})
	if node.IsFolder {
		for _,subNode := range node.Files {
			for file := range extractImages(subNode){
				m[file] = struct{}{}
			}
		}
	}else{
		m[node.AbsolutePath] = struct{}{}
	}
	return m
}

func (fm *FoldersManager)detectMissingFoldersIdOfFolder(folders map[string]*Node)int{
	counter := 0
	for _,folder := range folders {
		if folder.IsFolder {
			if folder.Id == 0 {
				// Need to define folder
				counter++
				folder.Id = fm.nextFolderId
				fm.nextFolderId++
			}
			// Browse subfolders
			counter += fm.detectMissingFoldersIdOfFolder(folder.Files)
		}
	}
	return counter
}

// Detect folders without id, create if necessary
func (fm *FoldersManager)detectMissingFoldersId(){
	counter := fm.detectMissingFoldersIdOfFolder(fm.Folders)
	if counter != 0 {
		// Save configuration to keep new ids
		logger.GetLogger2().Info("Save new ids of folder",counter,". Next folder id is",fm.nextFolderId)
		fm.save()
	}
}

func (fm *FoldersManager)load(){
	if f,err := os.Open(getSavePath()) ; err == nil {
		defer f.Close()
		data,_ := ioutil.ReadAll(f)
		folders := make(map[string]*Node,0)
		json.Unmarshal(data,&folders)
		fm.Folders = folders

	}else{
		logger.GetLogger2().Error("Impossible to read saved config",getSavePath(),err)
	}
}

func getSavePath()string{
	wd,_ := os.Getwd()
	return filepath.Join(wd,"save-images.json")
}

// folder must be a relative path
// addToFolder, if true, can add photos in existing folder
func (fm *FoldersManager)UploadFolder(folder string, files []multipart.File,names []string,addToFolder bool)(*progress.UploadProgress,error){
	if len(files) != len(names){
		return nil,errors.New("error during upload")
	}
	if !addToFolder && strings.EqualFold("",fm.UploadedFolder) {
		return nil,errors.New("impossible to upload file without folder defined")
	}
	// Check no double dots to move info tree
	if strings.Contains(folder,"..") {
		return nil,errors.New("too dangerous relative path folder with .. inside")
	}

	outputFolder := filepath.Join(fm.UploadedFolder,folder)
	if addToFolder {
		// Path is extract from existing node
		if node,_,err := fm.FindNode(folder); err != nil {
			return nil,err
		}else{
			outputFolder = node.AbsolutePath
		}
	}else {
		if err := createFolderIfExistOrFail(outputFolder); err != nil {
			return nil,err
		}
	}
	// Create work in go routine and return a progresser status
	progresser := fm.uploadProgressManager.AddUploader(len(files))
	go fm.doUploadFolder(folder,outputFolder,names,files,addToFolder,progresser)
	return progresser,nil
}

func (fm *FoldersManager)doUploadFolder( folder,outputFolder string,names []string,files []multipart.File,addToFolder bool,p *progress.UploadProgress){
	// Copy files on filer
	for i,file := range files {
		if imageFile,err := os.OpenFile(filepath.Join(outputFolder,names[i]),os.O_CREATE|os.O_RDWR,os.ModePerm); err == nil {
			if _,err := io.Copy(imageFile,file) ; err != nil {
				// Send Error to progresser and stop
				p.Error(err)
				return
			}
			imageFile.Close()
			p.Done()
		}else{
			p.Error(err)
			return
		}
	}
	// Use default source to add folder in a specific folder by default, not in root. Resize will be in default-source and path also
	logger.GetLogger2().Info("Folder",folder,"well uploaded with",len(files),"files")
	// If photos added in existing folder, update folder, otherwise, index
	if addToFolder {
		if err := fm.UpdateFolder(folder,p) ; err != nil {
			p.Error(err)
		}
		return
	}
	// Launch add folder with input folder, node path
	if err :=  fm.AddFolderToNode(outputFolder,filepath.Join(fm.overrideUploadFolder,folder),fm.overrideUploadFolder,false,false,p) ; err != nil {
		p.Error(err)
	}
}

func createFolderIfExistOrFail(path string)error {
	if _,err := os.Open(path) ; err == nil {
		return errors.New("folder already exist, must be new (" + path + ")")
	}
	return os.MkdirAll(path,os.ModePerm)
}

func (fm *FoldersManager)save(){
	fm.resetPhotosByDate()
	data,_ := json.Marshal(fm.Folders)
	if f,err := os.OpenFile(getSavePath(),os.O_TRUNC|os.O_CREATE|os.O_RDWR,os.ModePerm) ; err == nil {
		defer f.Close()
		f.Write(data)
		logger.GetLogger2().Info("Save tree in file",getSavePath())
	}else{
		logger.GetLogger2().Error("Impossible to save tree in file",getSavePath())
	}
}

func (fm *FoldersManager)launchImageResize(folder *Node, rootFolder,overrideOutput string,p *progress.UploadProgress, existings map[string]struct{},forceRotate bool){
	folder.RelativePath = filepath.Join(overrideOutput,folder.RelativePath)
	folder.applyOnEach(rootFolder,func(path,relativePath string,node * Node){
		p.Add(1)
		// Override relative path to include override output
		node.RelativePath = filepath.Join(overrideOutput,node.RelativePath)
		fm.reducer.AddImage(path,relativePath,overrideOutput,node,p,existings,forceRotate)
	})
	go func(node *Node){
		p.Wait()
		logger.GetLogger2().Info("End of resize folder",folder.Name)
		node.ImagesResized=true
	}(folder)
}

func (fm FoldersManager)AnalyseAsOne(rootFolder,path string)(string,*Node){
	files := fm.Analyse(rootFolder,path)
	if len(files) == 1 {
		for name,node := range files {
			return name,node
		}
	}
	return "",nil
}

//Analyse a cache and detect all files of types images
func (fm FoldersManager)Analyse(rootFolder,path string)Files{
	if file,err := os.Open(path) ; err == nil{
		defer file.Close()
		// If cache, create cache and go deep
		if stat,errStat := file.Stat() ; errStat == nil {
			if stat.IsDir() {
				return fm.treatFolder(rootFolder,path,stat.Name(),file)
			}else{
				return fm.treatImage(rootFolder,path,stat.Name())
			}
		}
	}else{
		logger.GetLogger2().Error(err.Error() + " : " + rootFolder + " ; " + path)
	}
	return Files{}
}

func (fm FoldersManager)treatImage(rootFolder,path,name string)map[string]*Node{
	// Test if is image
	if isImage(name){
		return createSimpleMap(name,NewImage(rootFolder,path, name))
	}
	return Files{}
}

func (fm FoldersManager)treatFolder (rootFolder,path,name string,file *os.File)map[string]*Node{
	files,_ := file.Readdirnames(-1)
	nodes := make(map[string]*Node,0)
	for _,file := range files {
		for name,node := range fm.Analyse(rootFolder,filepath.Join(path,file)){
			nodes[name] = node
		}
	}
	if len(nodes) > 0 {
		return createSimpleMap(name,NewFolder(rootFolder,path, name, nodes,false))
	}
	return Files{}
}
func createSimpleMap(name string,node *Node)map[string]*Node{
	r := make(map[string]*Node,0)
	r[name] = node
	return r
}

func (fm FoldersManager)List()[]*Node{
	nodes := make([]*Node,0,len(fm.Folders))
	for name,folder := range fm.Folders{
		nodes = append(nodes,NewFolder("",name,name,nil,folder.ImagesResized))
	}
	return nodes
}

func (fm *FoldersManager) Browse(path string) ([]*Node,int,error){
	if len(path) < 2 {
		// Return list
		return fm.List(),0,nil

	}else{
		node,err:= fm.browsePaths(path)
		if err != nil{
			return nil,0,err
		}
		// Parse file of nodes
		nodes := make([]*Node,0,len(node.Files))
		for _,file := range node.Files {
			nodes = append(nodes,file)
		}
		return nodes,node.Id,nil
	}
}

func (fm *FoldersManager)browsePaths(path string)(*Node,error){
	var node *Node
	var exist bool
	// Browse path
	for i,folder := range strings.Split(path[1:],"/") {
		if i == 0 {
			if node,exist = fm.Folders[folder] ; !exist {
				return nil,errors.New("Invalid path " + folder)
			}
		}else{
			if !strings.EqualFold("",strings.Trim(folder," ")) {
				if !node.IsFolder {
					return nil, errors.New("Not a valid cache " + folder)
				}
				node = node.Files[folder]
			}
		}
	}
	return node,nil
}

func (fm *FoldersManager) Count() int{
	count := 0
	for _,nodes := range fm.GetPhotosByDate() {
		count+=len(nodes)
	}
	return count
}

func (fm *FoldersManager) IndexFolder(path string, folder string) error {
	if _,_,err := fm.FindNode(path) ; err == nil {
		return errors.New("path already exist")
	}
	p := fm.uploadProgressManager.AddUploader(0)
	return fm.AddFolderToNode(folder,path,"",false,true,p)
}

func isImage(name string)bool{
	for _,suffix := range extensions {
		if  strings.HasSuffix(strings.ToLower(name),suffix){
			return true
		}
	}
	return false
}
