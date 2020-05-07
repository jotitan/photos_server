package photos_server

import (
	"encoding/json"
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)


type Server struct {
	foldersManager *foldersManager
	resources string
	maskForAdmin string
	pathRoutes map[string]func(w http.ResponseWriter,r *http.Request)
}

func NewPhotosServer(cache,resources,garbage,maskForAdmin,uploadedFolder,overrideUploadFolder string)Server{
	s := Server{
		foldersManager:NewFoldersManager(cache,garbage,maskForAdmin,uploadedFolder,overrideUploadFolder),
		resources:resources,
		maskForAdmin:maskForAdmin,
	}
	s.loadPathRoutes()
	return s
}

func (s Server)canAccessAdmin(r * http.Request)bool{
	return !strings.EqualFold("",s.maskForAdmin) && (
		strings.Contains(r.Referer(),s.maskForAdmin) ||
			strings.Contains(r.RemoteAddr,s.maskForAdmin))
}

func (s Server)updateExifOfDate(w http.ResponseWriter,r * http.Request){
	if updates,err := s.foldersManager.updateExifOfDate(r.FormValue("date")) ; err != nil {
		http.Error(w,err.Error(),400)
	}else{
		w.Write([]byte(fmt.Sprintf("Update %d exif dates",updates)))
	}
}

func (s Server)canDelete(w http.ResponseWriter,r * http.Request){
	w.Header().Set("Access-Control-Allow-Origin","*")
	w.Header().Set("Content-type","application/json")
	w.Write([]byte(fmt.Sprintf("{\"can\":%t}",s.foldersManager.garbageManager!=nil && s.canAccessAdmin(r))))
}

func (s Server)flushTags(w http.ResponseWriter,r * http.Request){
	s.foldersManager.tagManger.flush()
}

func (s Server)getPhotosByDate(w http.ResponseWriter,r * http.Request){
	if date, err := time.Parse("20060102",r.FormValue("date")) ; err == nil {
		if photos,exist := s.foldersManager.GetPhotosByDate()[date] ; exist {
			converts := s.convertPaths(photos,false)
			response := imagesResponse{Files:converts,Tags:s.foldersManager.tagManger.GetTagsByDate(r.FormValue("date"))}
			w.Header().Set("Access-Control-Allow-Origin","*")
			w.Header().Set("Content-type","application/json")
			if data,err := json.Marshal(response) ; err == nil {
				w.Write(data)
			}
		}
	}
}

type tagDto struct{
	Value string
	Color string
	ToRemove bool
}

func (s Server)updateTagsByFolder(w http.ResponseWriter,r * http.Request){
	if r.Method != "POST"{
		http.Error(w,"Only post is allowed",405)
		return
	}
	s.updateTag(w,r,r.URL.Path[14:],s.foldersManager.tagManger.AddTagByFolder,s.foldersManager.tagManger.RemoveByFolder)
}

func (s Server)updateTagsByDate(w http.ResponseWriter,r * http.Request){
	if r.Method != "POST"{
		http.Error(w,"Only post is allowed",405)
		return
	}
	s.updateTag(w,r,r.URL.Path[12:],s.foldersManager.tagManger.AddTagByDate,s.foldersManager.tagManger.RemoveByDate)
}

func (s Server)updateTag(w http.ResponseWriter,r * http.Request,key string,updateTag func(string,string,string)error,removeTag func(string,string,string)){
	w.Header().Set("Access-Control-Allow-Origin","*")
	if r.Method != "POST"{
		http.Error(w,"Only post is allowed",405)
		return
	}
	if data,err := ioutil.ReadAll(r.Body) ; err == nil {
		tag := tagDto{}
		if json.Unmarshal(data,&tag) == nil {
			if tag.ToRemove {
				removeTag(key,tag.Value,tag.Color)
			}else {
				if err := updateTag(key, tag.Value, tag.Color); err != nil {
					http.Error(w, err.Error(), 400)
				}
			}
		}
	}
}

// Return all dates of photos
func (s Server)getAllDates(w http.ResponseWriter,r * http.Request){
	dates := s.foldersManager.GetAllDates()
	data,_ := json.Marshal(dates)
	w.Header().Set("Access-Control-Allow-Origin","*")
	w.Header().Set("Content-type","application/json")
	w.Write(data)
}

func (s Server)count(w http.ResponseWriter,r * http.Request){
	w.Header().Set("Access-Control-Allow-Origin","*")
	w.Write([]byte(fmt.Sprintf("%d",s.foldersManager.Count())))
}

func (s Server)listFolders(w http.ResponseWriter,r * http.Request){
	names := make([]string,0,len(s.foldersManager.Folders))
	for name := range s.foldersManager.Folders {
		names = append(names,name)
	}
	if data,err := json.Marshal(names) ; err == nil{
		w.Header().Set("Content-type","application/json")
		w.Write(data)
	}
}

func (s Server)error403(w http.ResponseWriter,r * http.Request){
	logger.GetLogger2().Info("Try to delete by",r.Referer())
	http.Error(w,"You can't execute this action",403)
}

func (s Server)addFolder(w http.ResponseWriter,r * http.Request){
	if !s.canAccessAdmin(r) {
		s.error403(w,r)
		return
	}
	folder := r.FormValue("folder")
	forceRotate := r.FormValue("forceRotate") == "true"
	logger.GetLogger2().Info("Add folder",folder,"and forceRotate :",forceRotate)
	s.foldersManager.AddFolder(folder,forceRotate)
}

func (s Server)delete(w http.ResponseWriter,r * http.Request){
	w.Header().Set("Access-Control-Allow-Origin","*")
	if !s.canAccessAdmin(r) && s.foldersManager.garbageManager != nil{
		s.error403(w,r)
		return
	}
	data,_ := ioutil.ReadAll(r.Body)
	deletions := make([]string,0)
	json.Unmarshal(data,&deletions)
	imagesPath := make([]string,len(deletions))
	for i,deletion := range deletions {
		imagesPath[i] = strings.Replace(deletion,"/imagehd/","",-1)
	}
	successDeletions := s.foldersManager.garbageManager.Remove(imagesPath)
	w.Write([]byte(fmt.Sprintf("{\"success\":%d,\"errors\":%d}",successDeletions,len(imagesPath)-successDeletions)))
}

func (s Server)analyse(w http.ResponseWriter,r * http.Request){
	folder := r.FormValue("folder")
	logger.GetLogger2().Info("Analyse",folder)
	nodes := foldersManager{}.Analyse("",folder)
	if data,err := json.Marshal(nodes) ; err == nil {
		w.Header().Set("Content-type","application/json")
		w.Write(data)
	}
}

func (s Server)image(w http.ResponseWriter,r * http.Request){
	path := r.URL.Path[7:]
	s.writeImage(w,filepath.Join(s.foldersManager.reducer.cache,path))
}

func (s Server)writeImage(w http.ResponseWriter,path string){
	w.Header().Set("Content-type","image/jpeg")
	if file,err := os.Open(path) ; err == nil {
		defer file.Close()
		if _,e := io.Copy(w,file) ; e != nil {
			http.Error(w,"Error during image rendering",404)
		}
	}else{
		http.Error(w,"Image not found",404)
	}
}

func (s Server)removeNode(w http.ResponseWriter,r * http.Request) {
	path := r.URL.Path[12:]
	if err := s.foldersManager.RemoveNode(path) ; err != nil {
		http.Error(w,err.Error(),400)
	}else{
		w.Write([]byte("success"))
	}
}

// Return original image
func (s Server)imageHD(w http.ResponseWriter,r * http.Request){
	path := r.URL.Path[9:]
	// Find absolute path based on first folder
	if node,_,err := s.foldersManager.FindNode(path) ; err != nil {
		http.Error(w,"Impossible to find image",404)
	}else{
		s.writeImage(w,node.AbsolutePath)
	}
}

func (s Server)browse(w http.ResponseWriter,r * http.Request){
	// Extract folder
	path := r.URL.Path[7:]
	logger.GetLogger2().Info("Browse receive request",path)
	if files,err := s.foldersManager.Browse(path) ; err == nil {
		if data,err := json.Marshal(files) ; err == nil {
			w.Header().Set("Content-type","application/json")
			w.Write(data)
		}
	}else{
		http.Error(w,err.Error(),400)
	}
}

func (s Server)update(w http.ResponseWriter,r * http.Request){
	if !s.canAccessAdmin(r) {
		s.error403(w,r)
		return
	}
	logger.GetLogger2().Info("Launch update")
	if err := s.foldersManager.Update() ; err != nil {
		logger.GetLogger2().Error(err.Error())
	}
}

// Update a specific folder, faster than all folders
func (s Server)uploadFolder(w http.ResponseWriter,r * http.Request){
	if !s.canAccessAdmin(r) {
		s.error403(w,r)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin","*")

	pathFolder := r.FormValue("path")
	if r.MultipartForm == nil {
		logger.GetLogger2().Error("impossible to upload photos in "+ pathFolder)
		http.Error(w,"impossible to upload photos in "+ pathFolder,400)
		return
	}
	files,names := extractFiles(r)
	logger.GetLogger2().Info("Launch upload folder :",pathFolder)
	if strings.EqualFold("",pathFolder) {
		http.Error(w,"need to specify a path",400)
		return
	}
	if err := s.foldersManager.UploadFolder(pathFolder,files,names) ; err != nil {
		http.Error(w,"Bad request " + err.Error(),400)
		logger.GetLogger2().Error("Impossible to upload folder : ",err.Error())
	}else{
		w.Write([]byte("success"))
	}
}

func extractFiles(r *http.Request)([]multipart.File,[]string){
	files := make([]multipart.File,0,len(r.MultipartForm.File))
	names := make([]string,0,len(r.MultipartForm.File))
	for _,headers := range r.MultipartForm.File {
		if len(headers) > 0 {
			header := headers[0]
			if file,err := header.Open() ; err == nil {
				files = append(files, file)
				names = append(names,filepath.Base(header.Filename))
			}
		}
	}
	return files,names
}

// Update a specific folder, faster than all folders
func (s Server)updateFolder(w http.ResponseWriter,r * http.Request){
	if !s.canAccessAdmin(r) {
		s.error403(w,r)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin","*")
	folder := r.FormValue("folder")
	logger.GetLogger2().Info("Launch update folder :",folder)
	if err := s.foldersManager.UpdateFolder(folder) ; err != nil {
		logger.GetLogger2().Error(err.Error())
	}else{
		logger.GetLogger2().Info("End update folder",folder)
		w.Write([]byte("success"))
	}
}

func (s Server)getRootFolders(w http.ResponseWriter,r * http.Request){
	logger.GetLogger2().Info("Get root folders")
	w.Header().Set("Access-Control-Allow-Origin","*")
	nodes := make([]*Node,0,len(s.foldersManager.Folders))
	for _,node := range s.foldersManager.Folders {
		nodes = append(nodes,node)
	}
	root := folderRestFul{Name:"Racine",Link:"",Children:s.convertPaths(nodes,true)}
	if data,err := json.Marshal(root) ; err == nil {
		w.Header().Set("Content-type","application/json")
		w.Write(data)
	}
}

func (s Server)browseRestful(w http.ResponseWriter,r * http.Request){
	// Extract folder
	w.Header().Set("Access-Control-Allow-Origin","*")
	path := r.URL.Path[9:]
	logger.GetLogger2().Info("Browse restfull receive request",path)
	if files,err := s.foldersManager.Browse(path) ; err == nil {
		formatedFiles := s.convertPaths(files,false)
		tags :=s.foldersManager.tagManger.GetTagsByFolder(path[1:])
		imgResponse := imagesResponse{Files:formatedFiles,UpdateUrl:"/updateFolder?folder=" + path[1:],Tags:tags}
		if data,err := json.Marshal(imgResponse) ; err == nil {
			w.Header().Set("Content-type","application/json")
			w.Write(data)
		}
	}else{
		logger.GetLogger2().Info("Impossible to browse",path,err.Error())
		http.Error(w,err.Error(),400)
	}
}

type imagesResponse struct {
	Files []interface{}
	UpdateUrl string
	Tags []*Tag
}

// Restful representation : real link instead real path
type imageRestFul struct{
	Name string
	ThumbnailLink string
	ImageLink string
	HdLink string
	Width int
	Height int
	Date time.Time
	Orientation int
}

type folderRestFul struct{
	Name string
	Link string
	// Link to update tags
	LinkTags string
	// Means that folder also have images to display
	HasImages bool
	Children []interface{}
}

func (s Server)newImageRestful(node *Node)imageRestFul{
	return imageRestFul{
		Name: node.Name, Width: node.Width, Height: node.Height,Date:node.Date,
		HdLink:filepath.ToSlash(filepath.Join("/imagehd",node.RelativePath)),
		ThumbnailLink: filepath.ToSlash(filepath.Join("/image", s.foldersManager.GetSmallImageName(*node))),
		ImageLink:     filepath.ToSlash(filepath.Join("/image", s.foldersManager.GetMiddleImageName(*node)))}
}

// Convert node to restful response
func (s Server)convertPaths(nodes []*Node,onlyFolders bool)[]interface{}{
	files := make([]interface{},0,len(nodes))
	for _,node := range nodes {
		if !node.IsFolder {
			if !onlyFolders {
				files = append(files, s.newImageRestful(node))
			}
		}else{
			folder := folderRestFul{Name:node.Name,
				Link:filepath.ToSlash(filepath.Join("/browserf",node.RelativePath)),
				LinkTags:filepath.ToSlash(filepath.Join("/tagsByFolder",node.RelativePath)),
			}
			if onlyFolders {
				s.convertSubFolders(node,&folder)
			}
			files = append(files,folder)
		}
	}
	return files
}

func (s Server)convertSubFolders(node *Node,folder *folderRestFul){
	// Relaunch on subfolders
	subNodes := make([]*Node,0,len(node.Files))
	hasImages := false
	for _,n := range node.Files {
		subNodes = append(subNodes,n)
		if !n.IsFolder {
			hasImages = true
		}
	}
	folder.Children = s.convertPaths(subNodes,true)
	folder.HasImages = hasImages
}



func (s * Server)loadPathRoutes(){
	s.pathRoutes= map[string]func(w http.ResponseWriter,r * http.Request){
		"/browserf":s.browseRestful,
		"/browse":s.browse,
		"/imagehd":s.imageHD,
		"/image":s.image,
		"/removeNode":s.removeNode,
		"/tagsByFolder":s.updateTagsByFolder,
		"/tagsByDate":s.updateTagsByDate,
	}
}

func (s Server)defaultHandle(w http.ResponseWriter,r * http.Request){
	path := r.URL.Path[:strings.Index(r.URL.Path[1:],"/")+1]
	if fct,exist := s.pathRoutes[path] ; exist {
		fct(w,r)
	}else {
		logger.GetLogger2().Info("Receive request", r.URL, r.URL.Path)
		http.ServeFile(w, r, filepath.Join(s.resources, r.RequestURI[1:]))
	}
}

func (s Server)Launch(port string){
	server := http.ServeMux{}
	server.HandleFunc("/analyse",s.analyse)
	server.HandleFunc("/delete",s.delete)
	server.HandleFunc("/addFolder",s.addFolder)
	server.HandleFunc("/rootFolders",s.getRootFolders)
	server.HandleFunc("/update",s.update)
	server.HandleFunc("/updateFolder",s.updateFolder)
	server.HandleFunc("/uploadFolder",s.uploadFolder)
	server.HandleFunc("/listFolders",s.listFolders)
	server.HandleFunc("/canDelete",s.canDelete)
	server.HandleFunc("/updateExifOfDate",s.updateExifOfDate)
	server.HandleFunc("/count",s.count)
	// By date
	server.HandleFunc("/allDates",s.getAllDates)
	server.HandleFunc("/getByDate",s.getPhotosByDate)
	server.HandleFunc("/flushTags",s.flushTags)
	server.HandleFunc("/",s.defaultHandle)

	logger.GetLogger2().Info("Start server on port " + port)
	err := http.ListenAndServe(":" + port,&server)
	logger.GetLogger2().Error("Server stopped cause",err)
}