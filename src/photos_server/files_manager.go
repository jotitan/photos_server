package photos_server

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type INode interface {
	IsDir()bool
}

type Files map[string]*Node

type Node struct {
	AbsolutePath string
	// Path of node relative to head
	RelativePath string
	Width int
	Height int
	Date time.Time
	Name string
	IsFolder bool
	// Store files in a map with name
	Files Files
	ImagesResized bool
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
type foldersManager struct{
	Folders map[string]*Node
	PhotosByDate map[time.Time][]*Node
	garbageManager * GarbageManager
	reducer Reducer
	// Path of folder where to upload files
	UploadedFolder string
	// When upload file, override first folder in tree (to force to be in a specific one)
	overrideUploadFolder string
	tagManger * TagManager
	uploadProgressManager *uploadProgressManager
}

func NewFoldersManager(cache,garbageFolder,maskAdmin, uploadedFolder,overrideUploadFolder string)*foldersManager{
	fm := &foldersManager{Folders:make(map[string]*Node,0),reducer:NewReducer(cache,[]uint{1080,250}),
		UploadedFolder:uploadedFolder,overrideUploadFolder:overrideUploadFolder,
	uploadProgressManager:newUploadProgressManager()}
	fm.load()
	fm.garbageManager = NewGarbageManager(garbageFolder,maskAdmin,fm)
	fm.tagManger = NewTagManager(fm)
	return fm
}

type photosByDate struct {
	Date time.Time
	Nb int
}

func (fm * foldersManager)GetAllDates()[]photosByDate{
	byDate := fm.GetPhotosByDate()
	dates := make([]photosByDate,0,len(byDate))
	for date,nodes := range byDate {
		dates = append(dates,photosByDate{Date:date,Nb:len(nodes)})
	}
	return dates
}

func (fm * foldersManager)resetPhotosByDate(){
	fm.PhotosByDate = nil
}

// Update exif of all photos of a specific date
func (fm * foldersManager)updateExifOfDate(date string)(int,error){
	if parseDate,err := time.Parse("20060102",date) ; err != nil {
		return 0,err
	}else {
		midnightDate := getMidnightDate(parseDate)
		nodes,exist := fm.GetPhotosByDate()[midnightDate]
		logger.GetLogger2().Info("Found",len(nodes),"to update exif for",midnightDate)
		if !exist {
			return 0,errors.New("impossible to find images for this date")
		}
		for _,node := range nodes {
			// extract again exif date and update node
			node.Date,_ = GetExif(node.AbsolutePath)
			logger.GetLogger2().Info("Found date",node.Date,"for path",node.AbsolutePath)
		}
		fm.save()
		return len(nodes),nil
	}
}


func (fm * foldersManager)GetPhotosByDate()map[time.Time][]*Node{
	if fm.PhotosByDate == nil {
		fm.PhotosByDate = fm.computePhotosByDate(fm.Folders)
	}
	return fm.PhotosByDate
}

func (fm foldersManager) GetSmallImageName(node Node)string{
	return fm.reducer.createJpegFile(filepath.Dir(node.RelativePath),node.RelativePath,fm.reducer.sizes[1])
}

func (fm foldersManager) GetMiddleImageName(node Node)string{
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
func (fm * foldersManager)UpdateFolder(path string,progress *uploadProgress)error{
	if node,_,err := fm.FindNode(path) ; err != nil {
		return err
	}else {
		rootFolder := node.AbsolutePath[:len(node.AbsolutePath)-len(node.RelativePath)]
		files := fm.Analyse(rootFolder, node.AbsolutePath)
		// Take the specific folder
		files = files[filepath.Base(path)].Files
		fm.compareAndCleanFolder(files,path,make(map[string]*Node),progress)
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

func (fm * foldersManager)UpdateExif(path string)error {
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
func (fm * foldersManager)compareAndCleanFolder(files Files,folderPath string,newFolders map[string]*Node, progress *uploadProgress){

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
		progress.enableWaiter()
		progress.Add(len(delta))
		for _, node := range delta {
			logger.GetLogger2().Info("Launch update image resize", node.AbsolutePath)
			fm.reducer.AddImage(node.AbsolutePath, node.RelativePath, "",node, progress,make(map[string]struct{}),false)
		}
		progress.Wait()
		logger.GetLogger2().Info("All pictures have been resized")
	}

	// remove deletions in cache
	fm.removeFiles(deletions)
	for name, f := range files {
		newFolders[name] = f
	}
	progress.end()
}

//Update : compare structure in memory and folder to detect changes
func (fm * foldersManager)Update()error{
	// Have to block to avoid second update in same time
	// A lock is blocking, so use a chanel tiomeout to check if locker is still block
	updateWaiter := sync.WaitGroup{}
	updateWaiter.Add(1)
	begin := time.Now()
	chanelRunning := make(chan struct{},1)
	up := fm.uploadProgressManager.addUploader(0)
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

	// Detect if an update is already running. Is true (after one secon), return error, otherwise, wait for end of update
	select {
	case <- chanelRunning :
		updateWaiter.Wait()
		return nil
	case <- time.NewTimer(time.Millisecond*100).C:
		return errors.New("an update is already running")
	}
}

// Only remove the node in tree, not the file
func (fm * foldersManager)RemoveNode(path string)error{
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

func (fm foldersManager)FindNode(path string)(*Node,map[string]*Node,error){
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
	if current != nil {

	}
	return nil,nil,errors.New("Bad path " + path)
}

func (fm foldersManager)removeFiles(files map[string]*Node){
	for _,node := range files {
		fm.removeFilesNode(node)
	}
}

func (fm foldersManager)removeFilesNode( node * Node)error{
	if err := fm.removeFile(filepath.Join(fm.reducer.cache,fm.GetSmallImageName(*node))) ; err == nil {
		return fm.removeFile(filepath.Join(fm.reducer.cache,fm.GetMiddleImageName(*node)))
	}else{
		return err
	}
}

func (fm foldersManager)removeFile(path string)error{
	logger.GetLogger2().Info("Remove file",path)
	return os.Remove(path)
}

// used when upload
// @overrideOutput override default output folder by adding inside a path folder
// @forceRelativePath is true, use relativePath as real relative of new node
func (fm * foldersManager)AddFolderToNode(folderPath,relativePath,overrideOutput string,forceRotate,forceRelativePath bool,progress *uploadProgress)error{
	// Compute relative path
	rootFolder := filepath.Dir(relativePath)
	if strings.EqualFold("",rootFolder) || strings.EqualFold(".",rootFolder) {
		// Add folder as usual (new one)
		fm.AddFolder(folderPath,forceRotate,progress)
		return nil
	}
	// Find the node of root folder
	if node,_,err := fm.FindNode(rootFolder) ; err == nil {
		if forceRelativePath {
			// Override rootFolder
			root := folderPath[0:len(folderPath)-len(relativePath)]
			fm.AddFolderWithNode(node.Files,root,folderPath,overrideOutput,forceRotate,progress)
		}else {
			fm.AddFolderWithNode(node.Files, fm.UploadedFolder, folderPath, overrideOutput, forceRotate,progress)
		}
	}else{
		// Add the parent folder (which is recursive)
		return fm.AddFolderToNode(filepath.Dir(folderPath),rootFolder,overrideOutput,forceRotate,forceRelativePath,progress)
	}
	return nil
}

func (fm * foldersManager)AddFolder(folderPath string,forceRotate bool,progress *uploadProgress){
	fm.AddFolderWithNode(fm.Folders,"",folderPath,"",forceRotate,progress)
}

func (fm * foldersManager)AddFolderWithNode(files Files,rootFolder,folderPath,overrideOutput string,forceRotate bool,progress *uploadProgress){
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
	//globalWaiter := sync.WaitGroup{}
	progress.enableWaiter()
	//globalWaiter.Add(len(node))
	files[name] = node
	fm.launchImageResize(node,strings.Replace(folderPath,name,"",-1),overrideOutput,progress,existings,forceRotate)

	go func(){
		progress.Wait()
		progress.end()
		logger.GetLogger2().Info("End of resize folder",node.Name)
		node.ImagesResized=true
	}()
	//globalWaiter.Wait()
	fm.save()
}


func (fm * foldersManager) searchExistingReducedImages(folderPath string)map[string]struct{}{
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

func (fm * foldersManager)load(){
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

type uploadProgress struct {
	id        string
	chanel    chan struct{}
	total     int
	totalDone int
	// SSE connexion
	sses    []*sse
	waiter  *sync.WaitGroup
	manager *uploadProgressManager
}

// Add a waitergroup to manage Done / wait
func (up * uploadProgress)enableWaiter(){
	up.waiter = &sync.WaitGroup{}
}

func (up * uploadProgress)Add(size int){
	if up.waiter != nil {
		up.waiter.Add(size)
	}
}

func (up * uploadProgress)run(){
	go func(){
		for {
			if _,more := <-up.chanel ; more {
				up.totalDone++
				// Send notif if sse exist
				for _, s := range up.sses {
					s.done(stat{up.totalDone, up.total})
				}
			}else{
				// close chanel, send end message to all
				logger.GetLogger2().Info("Close chanel")
				for _, s := range up.sses {
					s.end()
				}
				break
			}
		}
	}()
}

func (up * uploadProgress)end(){
	close(up.chanel)
	up.manager.remove(up.id)
}

func (up * uploadProgress) Done(){
	if up.waiter != nil {
		up.waiter.Done()
	}
	up.chanel<-struct{}{}
}

func (up * uploadProgress)Wait(){
	if up.waiter != nil {
		up.waiter.Wait()
	}
}

func (up *uploadProgress) error(e error) {
	// Send message to sse and remove from manager
	for _, s := range up.sses {
		s.error(e)
	}
}

// Manage uploads progression
type uploadProgressManager struct{
	uploads map[string]*uploadProgress
	count int
}

func newUploadProgressManager()*uploadProgressManager{
	return &uploadProgressManager{make(map[string]*uploadProgress),0}
}

func (upm * uploadProgressManager)getStatUpload(id string)(stat,error){
	if up,ok := upm.uploads[id] ; ok {
		return stat{up.totalDone,up.total},nil
	}
	return stat{},errors.New("unknown upload id")
}

func (upm * uploadProgressManager)addSSE(id string, w http.ResponseWriter,r * http.Request)(*sse,error){
	if up,ok := upm.uploads[id] ; ok {
		sse := newSse(w,r)
		up.sses = append(up.sses,sse)
		return sse,nil
	}
	return nil,errors.New("unknown upload id")
}

// return unique id representing upload
func (upm * uploadProgressManager)addUploader(total int)*uploadProgress{
	id := upm.generateNewID()
	uploader := &uploadProgress{chanel:make(chan struct{},10),total:total*2,id:id,manager:upm}
	uploader.run()
	upm.uploads[id] = uploader
	return uploader
}

func (upm * uploadProgressManager)generateNewID()string{
	upm.count++
	h := md5.New()
	h.Write([]byte{byte(upm.count)})
	id := h.Sum([]byte{})
	return base64.StdEncoding.EncodeToString(id)
}

func (upm *uploadProgressManager) remove(id string) {
	delete(upm.uploads,id)
}

// folder must be a relative path
// addToFolder, if true, can add photos in existing folder
func (fm * foldersManager)UploadFolder(folder string, files []multipart.File,names []string,addToFolder bool)(*uploadProgress,error){
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
	progresser := fm.uploadProgressManager.addUploader(len(files))
	go fm.doUploadFolder(folder,outputFolder,names,files,addToFolder,progresser)
	return progresser,nil
}

func (fm * foldersManager)doUploadFolder( folder,outputFolder string,names []string,files []multipart.File,addToFolder bool,progress *uploadProgress){
	// Copy files on filer
	for i,file := range files {
		if imageFile,err := os.OpenFile(filepath.Join(outputFolder,names[i]),os.O_CREATE|os.O_RDWR,os.ModePerm); err == nil {
			if _,err := io.Copy(imageFile,file) ; err != nil {
				// Send error to progresser and stop
				progress.error(err)
				return
			}
			imageFile.Close()
			progress.Done()
		}else{
			progress.error(err)
			return
		}
	}
	// Use default source to add folder in a specific folder by default, not in root. Resize will be in default-source and path also
	logger.GetLogger2().Info("Folder",folder,"well uploaded with",len(files),"files")
	// If photos added in existing folder, update folder, otherwise, index
	if addToFolder {
		if err := fm.UpdateFolder(folder,progress) ; err != nil {
			progress.error(err)
		}
		return
	}
	// Launch add folder with input folder, node path
	if err :=  fm.AddFolderToNode(outputFolder,filepath.Join(fm.overrideUploadFolder,folder),fm.overrideUploadFolder,false,false,progress) ; err != nil {
		progress.error(err)
	}
}

func createFolderIfExistOrFail(path string)error {
	if _,err := os.Open(path) ; err == nil {
		return errors.New("folder already exist, must be new (" + path + ")")
	}
	return os.MkdirAll(path,os.ModePerm)
}

func (fm * foldersManager)save(){
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

func (fm * foldersManager)launchImageResize(folder *Node, rootFolder,overrideOutput string,progress *uploadProgress, existings map[string]struct{},forceRotate bool){
	folder.RelativePath = filepath.Join(overrideOutput,folder.RelativePath)
	folder.applyOnEach(rootFolder,func(path,relativePath string,node * Node){
		progress.Add(1)
		// Override relative path to include override output
		node.RelativePath = filepath.Join(overrideOutput,node.RelativePath)
		fm.reducer.AddImage(path,relativePath,overrideOutput,node,progress,existings,forceRotate)
	})
	go func(node *Node){
		progress.Wait()
		logger.GetLogger2().Info("End of resize folder",folder.Name)
		node.ImagesResized=true
	}(folder)
}

func (fm foldersManager)AnalyseAsOne(rootFolder,path string)(string,*Node){
	files := fm.Analyse(rootFolder,path)
	if len(files) == 1 {
		for name,node := range files {
			return name,node
		}
	}
	return "",nil
}

//Analyse a cache and detect all files of types images
func (fm foldersManager)Analyse(rootFolder,path string)Files{
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

func (fm foldersManager)treatImage(rootFolder,path,name string)map[string]*Node{
	// Test if is image
	if isImage(name){
		return createSimpleMap(name,NewImage(rootFolder,path, name))
	}
	return Files{}
}

func (fm foldersManager)treatFolder (rootFolder,path,name string,file *os.File)map[string]*Node{
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

func (fm foldersManager)List()[]*Node{
	nodes := make([]*Node,0,len(fm.Folders))
	for name,folder := range fm.Folders{
		nodes = append(nodes,NewFolder("",name,name,nil,folder.ImagesResized))
	}
	return nodes
}

func (fm *foldersManager) Browse(path string) ([]*Node,error){
	if len(path) < 2 {
		// Return list
		return fm.List(),nil

	}else{
		node,err:= fm.browsePaths(path)
		if err != nil{
			return nil,err
		}
		// Parse file of nodes
		nodes := make([]*Node,0,len(node.Files))
		for _,file := range node.Files {
			nodes = append(nodes,file)
		}
		return nodes,nil
	}
}

func (fm * foldersManager)browsePaths(path string)(*Node,error){
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

func (fm *foldersManager) computePhotosByDate(files Files) map[time.Time][]*Node {
	byDate := make(map[time.Time][]*Node)
	// Browse all pictures and group by date
	for _,node := range files {
		if node.IsFolder {
			// Relaunch
			for date,nodes := range fm.computePhotosByDate(node.Files) {
				addInTimeMap(byDate,date,nodes)
			}
		}else{
			formatDate := getMidnightDate(node.Date)
			addInTimeMap(byDate,formatDate,[]*Node{node})
		}
	}
	return byDate
}

func (fm *foldersManager) Count() int{
	count := 0
	for _,nodes := range fm.GetPhotosByDate() {
		count+=len(nodes)
	}
	return count
}

func (fm *foldersManager) IndexFolder(path string, folder string) error {
	if _,_,err := fm.FindNode(path) ; err == nil {
		return errors.New("path already exist")
	}
	p := fm.uploadProgressManager.addUploader(0)
	return fm.AddFolderToNode(folder,path,"",false,true,p)
}

func addInTimeMap(byDate map[time.Time][]*Node,date time.Time,nodes []*Node){
	if list,exist := byDate[date] ; !exist {
		byDate[date] = nodes
	}else{
		byDate[date] = append(list,nodes...)
	}
}

func getMidnightDate(date time.Time)time.Time {
	if format,err := time.Parse("2006-01-02",date.Format("2006-01-02")) ; err == nil {
		return format
	}
	return date
}

func isImage(name string)bool{
	for _,suffix := range extensions {
		if  strings.HasSuffix(strings.ToLower(name),suffix){
			return true
		}
	}
	return false
}