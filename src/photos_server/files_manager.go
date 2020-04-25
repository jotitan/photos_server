package photos_server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"io/ioutil"
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
	relativePath := strings.Replace(path,rootFolder,"",-1)
	return &Node{AbsolutePath:path,RelativePath:relativePath,Name:name,IsFolder:true,Files:files,ImagesResized:imageResized}
}

// Store many folders
type foldersManager struct{
	Folders map[string]*Node
	garbageManager * GarbageManager
	reducer Reducer
}

func NewFoldersManager(cache,garbageFolder,maskAdmin string)*foldersManager{
	fm := &foldersManager{Folders:make(map[string]*Node,0),reducer:NewReducer(cache,[]uint{1080,250})}
	fm.load()
	fm.garbageManager = NewGarbageManager(garbageFolder,maskAdmin,fm)
	return fm
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
func (files Files)Compare(previousFiles Files)([]*Node,map[string]*Node){
	//fmt.Println("Compare",files,previousFiles)
	newNodes := make([]*Node,0)
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
			}else{
				// Relaunch on folder
				delta,deletions := file.Files.Compare(oldValue.Files)
				for _,node := range delta {
					newNodes = append(newNodes,node)
				}
				for name,node := range deletions {
					nodesToDelete[name] = node
				}

			}
		}else{
			// Treat folder
			if !file.IsFolder {
				newNodes = append(newNodes,file)
			}else{
				delta,deletions := file.Files.Compare(Files{})
				for _,node := range delta {
					newNodes = append(newNodes,node)
				}
				for name,node := range deletions {
					nodesToDelete[name] = node
				}
			}
		}
	}
	// Add local nodes to delete with other
	for name,node := range deletionMap {
		nodesToDelete[name] = node
	}
	return newNodes,nodesToDelete
}

// Add a locker to check if an update is running
var updateLocker = sync.Mutex{}

// Only update one folder
func (fm * foldersManager)UpdateFolder(path string)error{
	if node,_,err := fm.FindNode(path) ; err != nil {
		return err
	}else {
		files := fm.Analyse(filepath.Dir(node.AbsolutePath), node.AbsolutePath)
		// Take the specific folder
		files = files[filepath.Base(path)].Files
		fm.compareAndCleanFolder(files,make(map[string]*Node))
		node.Files = files
		fm.save()
		return nil
	}
}

func (fm * foldersManager)compareAndCleanFolder(files Files,newFolders map[string]*Node){

	// Include dry run and full (compare length a nodes or compare always everything)
	delta, deletions := files.Compare(fm.Folders)
	logger.GetLogger2().Info("After update", len(delta), "new pictures and", len(deletions), "to remove")
	// Launch indexation of new images,
	if len(delta) > 0 {
		waiter := &sync.WaitGroup{}
		for _, node := range delta {
			logger.GetLogger2().Info("Launch update image resize", node.AbsolutePath)
			waiter.Add(1)
			fm.reducer.AddImage(node.AbsolutePath, node.RelativePath, node, waiter,make(map[string]struct{}),false)
		}
		waiter.Wait()
		logger.GetLogger2().Info("All pictures have been resized")
	}

	// remove deletions in cache
	fm.removeFiles(deletions)
	for name, f := range files {
		newFolders[name] = f
	}
}

//Update : compare structure in memory and folder to detect changes
func (fm * foldersManager)Update()error{
	// Have to block to avoid second update in same time
	// A lock is blocking, so use a chanel tiomeout to check if locker is still block
	updateWaiter := sync.WaitGroup{}
	updateWaiter.Add(1)
	begin := time.Now()
	chanelRunning := make(chan struct{},1)
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
			fm.compareAndCleanFolder(files,newFolders)
		}
		fm.Folders = newFolders
		fm.save()
		updateWaiter.Done()
		updateLocker.Unlock()
	}()

	// Dectect if an update is already running. Is true (after one secon), return error, otherwise, wait for end of update
	select {
	case <- chanelRunning :
		updateWaiter.Wait()
		return nil
	case <- time.NewTimer(time.Millisecond*100).C:
		return errors.New("An update is already running")
	}
}

func (fm foldersManager)FindNode(path string)(*Node,map[string]*Node,error){
	current := fm.Folders
	nbSub := strings.Count(path,"/")
	if nbSub == 0{
		if node,ok := fm.Folders[path] ; ok {
			return node,fm.Folders,nil
		}
		return nil,nil,errors.New("Impossible to found node " + path)
	}
	for pos,sub := range strings.Split(path,"/") {
		node := current[sub]
		if node.IsFolder {
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

func (fm * foldersManager)AddFolder(folderPath string,forceRotate bool){
	rootFolder := filepath.Dir(folderPath)
	node := fm.Analyse(rootFolder,folderPath)
	logger.GetLogger2().Info("Add folder",folderPath,"with",len(node))
	// Check if images already exists to improve computing
	existings := fm.searchExistingImages(folderPath)
	logger.GetLogger2().Info("Found existing",len(existings))
	globalWaiter := sync.WaitGroup{}
	for name,folder := range node{
		fm.Folders[name] = folder
		fm.launchImageResize(folder,strings.Replace(folderPath,name,"",-1),&globalWaiter,existings,forceRotate)
	}
	globalWaiter.Wait()
	fm.save()
}

func (fm * foldersManager)searchExistingImages(folderPath string)map[string]struct{}{
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

func (fm foldersManager)save(){
	data,_ := json.Marshal(fm.Folders)
	if f,err := os.OpenFile(getSavePath(),os.O_TRUNC|os.O_CREATE|os.O_RDWR,os.ModePerm) ; err == nil {
		defer f.Close()
		f.Write(data)
		logger.GetLogger2().Info("Save tree in file",getSavePath())
	}else{
		logger.GetLogger2().Error("Impossible to save tree in file",getSavePath())
	}
}


func (fm * foldersManager)launchImageResize(folder *Node, rootFolder string,globalWaiter * sync.WaitGroup, existings map[string]struct{},forceRotate bool){
	globalWaiter.Add(1)
	waiter := &sync.WaitGroup{}
	folder.applyOnEach(rootFolder,func(path,relativePath string,node * Node){
		waiter.Add(1)
		fm.reducer.AddImage(path,relativePath,node,waiter,existings,forceRotate)
	})
	go func(w *sync.WaitGroup,node *Node){
		w.Wait()
		globalWaiter.Done()
		logger.GetLogger2().Info("End of resize folder",folder.Name)
		node.ImagesResized=true
	}(waiter,folder)
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
				fm.treatImage(rootFolder,path,stat.Name())
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

func isImage(name string)bool{
	for _,suffix := range extensions {
		if  strings.HasSuffix(strings.ToLower(name),suffix){
			return true
		}
	}
	return false
}