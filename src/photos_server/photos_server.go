package photos_server

import (
	"encoding/json"
	"fmt"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"github.com/jotitan/photos_server/security"
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
	foldersManager *FoldersManager
	resources string
	// Exist only if garbage exist
	securityAccess *security.SecurityAccess
	pathRoutes map[string]func(w http.ResponseWriter,r *http.Request)
}

// Create security access from good provider
func (s *Server)setSecurityAccess(conf *config.Config) {
	if s.foldersManager.garbageManager != nil {
		s.securityAccess = security.NewSecurityAccess(conf.Security.MaskForAdmin, []byte(conf.Security.HS256SecretKey))
		// Check if basic or provider is enabled
		if !strings.EqualFold("", conf.Security.BasicConfig.Username) {
			s.securityAccess.SetAccessProvider(security.NewBasicProvider(conf.Security.BasicConfig.Username, conf.Security.BasicConfig.Password))
		} else {
			if !strings.EqualFold("", conf.Security.OAuth2Config.Provider) {
				if provider := security.NewProvider(conf.Security.OAuth2Config); provider != nil {
					s.securityAccess.SetAccessProvider(security.NewOAuth2AccessProvider(provider, conf.Security.OAuth2Config.AuthorizedEmails,conf.Security.OAuth2Config.AdminEmails))
				}
			}
		}
	}
}

func NewPhotosServerFromConfig(conf *config.Config)Server{
	s := Server{
		foldersManager:NewFoldersManager(conf.CacheFolder,conf.Garbage,conf.Security.MaskForAdmin,conf.UploadedFolder,conf.OverrideUploadFolder),
		resources:conf.WebResources,
	}
	s.setSecurityAccess(conf)
	s.loadPathRoutes()
	return s
}

func (s Server)canAccessAdmin(r * http.Request)bool{
	return s.securityAccess!= nil && s.securityAccess.CheckJWTTokenAdminAccess(r)
}

func (s Server)canAccessUser(r * http.Request)bool{
	return s.securityAccess!= nil && s.securityAccess.CheckJWTTokenAccess(r)
}

func (s Server)updateExifOfDate(w http.ResponseWriter,r * http.Request){
	if updates,err := s.foldersManager.updateExifOfDate(r.FormValue("date")) ; err != nil {
		http.Error(w,err.Error(),400)
	}else{
		w.Write([]byte(fmt.Sprintf("Update %d exif dates",updates)))
	}
}

func (s Server)getSecurityConfig(w http.ResponseWriter,r * http.Request){
	if s.securityAccess != nil {
		w.Write([]byte(s.securityAccess.GetTypeSecurity()))
	}	else{
		w.Write([]byte("{\"name\":\"none\"}"))
	}
}

func (s Server)canAdmin(w http.ResponseWriter,r * http.Request){
	header(w)
	if !s.canAccessAdmin(r) {
		http.Error(w, "acess denied, only admin", 403)
	}
}

// Can access is enable if oauth2 is configured, otherwise, only admin is checked
func (s Server)canAccess(w http.ResponseWriter,r * http.Request){
	header(w)
	if s.securityAccess != nil{
		if !s.securityAccess.CheckJWTTokenAccess(r) {
			http.Error(w,"access denied",401)
		}
	}
}

func (s Server)connect(w http.ResponseWriter,r * http.Request){
	if ! s.securityAccess.Connect(w,r) {
		http.Error(w,"impossible to connect",401)
	}
}

func (s Server)flushTags(w http.ResponseWriter,r * http.Request){
	s.foldersManager.tagManger.flush()
}

func header(w http.ResponseWriter){
	w.Header().Set("Access-Control-Allow-Origin","*")
	w.Header().Set("Content-type","application/json")
}

func (s Server)filterTagsFolder(w http.ResponseWriter,r * http.Request){
	s.filterByFct(w,r,s.foldersManager.tagManger.FilterFolder)
}

func (s Server)filterTagsDate(w http.ResponseWriter,r * http.Request){
	s.filterByFct(w,r,s.foldersManager.tagManger.FilterDate)
}

func (s Server)filterByFct(w http.ResponseWriter,r * http.Request,filter func(string)[]string){
	folders := filter(r.FormValue("value"))
	header(w)
	if data,err := json.Marshal(folders) ; err == nil {
		w.Write(data)
	}
}

func (s Server)getPhotosByDate(w http.ResponseWriter,r * http.Request){
	if date, err := time.Parse("20060102",r.FormValue("date")) ; err == nil {
		if photos,exist := s.foldersManager.GetPhotosByDate()[date] ; exist {
			converts := s.convertPaths(photos,false)
			response := imagesResponse{Files:converts,Tags:s.foldersManager.tagManger.GetTagsByDate(r.FormValue("date"))}
			header(w)
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
	header(w)
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
		header(w)
		w.Write(data)
	}
}

func (s Server)error403(w http.ResponseWriter,r * http.Request){
	logger.GetLogger2().Info("Try to action by",r.Referer())
	http.Error(w,"You can't execute this action",403)
}

func (s Server)addFolder(w http.ResponseWriter,r * http.Request){
	folder := r.FormValue("folder")
	forceRotate := r.FormValue("forceRotate") == "true"
	logger.GetLogger2().Info("Add folder",folder,"and forceRotate :",forceRotate)
	p := s.foldersManager.uploadProgressManager.addUploader(0)
	s.foldersManager.AddFolder(folder,forceRotate,p)
}

func (s Server)delete(w http.ResponseWriter,r * http.Request){
	header(w)
	if s.foldersManager.garbageManager != nil{
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
	nodes := FoldersManager{}.Analyse("",folder)
	if data,err := json.Marshal(nodes) ; err == nil {
		w.Header().Set("Content-type","application/json")
		w.Write(data)
	}
}

func (s Server)image(w http.ResponseWriter,r * http.Request){
	path := r.URL.Path[7:]
	if !s.canReadPath(getCleanPath(path),r){
		s.error403(w,r)
		return
	}
	s.writeImage(w,filepath.Join(s.foldersManager.reducer.cache,path))
}

func (s Server)canReadPath(path string,r * http.Request)bool{
	return s.canAccessUser(r) || s.securityAccess.ShareFolders.CanRead(s.securityAccess.GetUserId(r),path)
}

func getCleanPath(path string)string{
	if !strings.Contains(path,"/") {
		return path
	}
	return path[:strings.LastIndex(path,"/")]
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
	header(w)
	if s.foldersManager.garbageManager != nil{
		s.error403(w,r)
		return
	}
	path := r.URL.Path[12:]
	if err := s.foldersManager.RemoveNode(path) ; err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	logger.GetLogger2().Info("Remove node",path)
	w.Write([]byte("success"))
}

// Return original image
func (s Server)imageHD(w http.ResponseWriter,r * http.Request){
	path := r.URL.Path[9:]
	if !s.canReadPath(getCleanPath(path),r){
		s.error403(w,r)
		return
	}
	// Find absolute path based on first folder
	if node,_,err := s.foldersManager.FindNode(path) ; err != nil {
		http.Error(w,"Impossible to find image",404)
	}else{
		s.writeImage(w,node.AbsolutePath)
	}
}

func (s Server)update(w http.ResponseWriter,r * http.Request){
	logger.GetLogger2().Info("Launch update")
	if err := s.foldersManager.Update() ; err != nil {
		logger.GetLogger2().Error(err.Error())
	}
}


// Update a specific folder, faster than all folders
// Return an id to monitor upload
func (s Server)uploadFolder(w http.ResponseWriter,r * http.Request){
	w.Header().Set("Access-Control-Allow-Origin","*")

	pathFolder := r.FormValue("path")
	addToFolder := strings.EqualFold(r.FormValue("addToFolder"),"true")
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
	if progresser,err := s.foldersManager.UploadFolder(pathFolder,files,names,addToFolder) ; err != nil {
		http.Error(w,"Bad request " + err.Error(),400)
		logger.GetLogger2().Error("Impossible to upload folder : ",err.Error())
	}else{
		w.Write([]byte(fmt.Sprintf("{\"status\":\"running\",\"id\":\"%s\"}",progresser.id)))
	}
}

func (s Server)statUploadRT(w http.ResponseWriter,r * http.Request){
	id := r.FormValue("id")
	if sse,err := s.foldersManager.uploadProgressManager.addSSE(id,w,r) ; err == nil {
		// Block to write messages
		sse.watch()
		logger.GetLogger2().Info("End watch")
	}else{
		logger.GetLogger2().Info("Impossible to watch upload",err.Error())
		http.Error(w,err.Error(),404)
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

func (s Server)updateExifFolder(w http.ResponseWriter,r * http.Request){
	w.Header().Set("Access-Control-Allow-Origin","*")
	folder := r.FormValue("folder")
	logger.GetLogger2().Info("Launch update exif folder :",folder)
	if err := s.foldersManager.UpdateExif(folder) ; err != nil {
		logger.GetLogger2().Error(err.Error())
	}else{
		logger.GetLogger2().Info("End update folder",folder)
		w.Write([]byte("success"))
	}
}

// Update a specific folder, faster than all folders
func (s Server)updateFolder(w http.ResponseWriter,r * http.Request){
	w.Header().Set("Access-Control-Allow-Origin","*")
	folder := r.FormValue("folder")
	logger.GetLogger2().Info("Launch update folder :",folder)
	up := s.foldersManager.uploadProgressManager.addUploader(0)
	if err := s.foldersManager.UpdateFolder(folder,up) ; err != nil {
		logger.GetLogger2().Error(err.Error())
	}else{
		logger.GetLogger2().Info("End update folder",folder)
		w.Write([]byte("success"))
	}
}

// Index an existing folder
func (s Server)indexFolder(w http.ResponseWriter,r * http.Request){
	w.Header().Set("Access-Control-Allow-Origin","*")
	path := r.FormValue("path")
	folder := r.FormValue("folder")
	logger.GetLogger2().Info("Index :",folder,"with path",path)
	if err := s.foldersManager.IndexFolder(path,folder) ; err != nil {
		logger.GetLogger2().Error(err.Error())
	}else{
		logger.GetLogger2().Info("End update folder",folder)
		w.Write([]byte("success"))
	}
}

func (s Server)getSharesFolder(w http.ResponseWriter,r * http.Request){
	id := s.securityAccess.GetUserId(r)
	if shares,err := s.securityAccess.ShareFolders.Get(id); err != nil {
		s.error403(w,r)
	}else{
		header(w)
		logger.GetLogger2().Info("Get shares for",id)
		sharesNode := make([]*Node,0,len(shares))
		for _,share := range shares {
			if node,_,err := s.foldersManager.FindNode(share) ; err == nil {
				sharesNode = append(sharesNode,node)
			}
		}
		root := folderRestFul{Name:"Racine",Link:"",Children:s.convertPaths(sharesNode,true)}
		if data,err := json.Marshal(root) ; err == nil {
			w.Write(data)
		}
	}
}

func (s Server)getRootFolders(w http.ResponseWriter,r * http.Request){
	if ! s.securityAccess.CheckJWTTokenAccess(r){
	// If no access, try addShare
	s.getSharesFolder(w,r)
	return
	}

	logger.GetLogger2().Info("Get root folders")
	header(w)
	nodes := make([]*Node,0,len(s.foldersManager.Folders))
	for _,node := range s.foldersManager.Folders {
		nodes = append(nodes,node)
	}
	root := folderRestFul{Name:"Racine",Link:"",Children:s.convertPaths(nodes,true)}
	if data,err := json.Marshal(root) ; err == nil {
		w.Write(data)
	}
}

func (s Server)browseRestful(w http.ResponseWriter,r * http.Request){
	// Check if user can access, if not, check if is invited
	if ! s.securityAccess.CheckJWTTokenAccess(r){
		s.error403(w,r)
		return
	}

	// Return all tree
	header(w)
	path := r.URL.Path[9:]
	if !s.canReadPath(path[1:],r){
		s.error403(w,r)
		return
	}
	logger.GetLogger2().Info("Browse restfull receive request",path)
	if files,err := s.foldersManager.Browse(path) ; err == nil {
		formatedFiles := s.convertPaths(files,false)
		tags :=s.foldersManager.tagManger.GetTagsByFolder(path[1:])
		imgResponse := imagesResponse{Files:formatedFiles,UpdateExifUrl:"/updateExifFolder?folder=" + path[1:],UpdateUrl:"/updateFolder?folder=" + path[1:],FolderPath:path[1:],Tags:tags}
		if s.canAccessAdmin(r){
			imgResponse.RemoveFolderUrl="/removeNode" + path
		}
		if data,err := json.Marshal(imgResponse) ; err == nil {
			w.Write(data)
		}
	}else{
		logger.GetLogger2().Info("Impossible to browse",path,err.Error())
		http.Error(w,err.Error(),400)
	}
}

func (s Server)checkNodeExist(path string)bool{
	_,_,err := s.foldersManager.FindNode(path)
	return err == nil
}

// Return who an access the addShare
func (s Server)getShares(w http.ResponseWriter,r * http.Request){
	users := s.securityAccess.ShareFolders.GetUsersOfPath(r.FormValue("path"))
	if data,err := json.Marshal(users) ; err == nil {
		w.Write(data)
	}else{
		http.Error(w,"impossible to get users for path",404)
	}
}

func (s Server) addShare(w http.ResponseWriter,r * http.Request){
	if err := s.securityAccess.ShareFolders.Add(r.FormValue("user"),r.FormValue("path"),s.checkNodeExist); err != nil {
		http.Error(w,err.Error(),400)
	}
}

func (s Server) manageShare(w http.ResponseWriter,r * http.Request){
	switch r.Method {
	case http.MethodPost:
		s.addShare(w,r)
		break
	case http.MethodGet:
		s.getShares(w,r)
		break
	case http.MethodDelete:
		s.removeShare(w,r)
		break
	}
}

func (s Server) removeShare(w http.ResponseWriter,r * http.Request){
	if err := s.securityAccess.ShareFolders.Remove(r.FormValue("user"),r.FormValue("path"),s.checkNodeExist); err != nil {
		http.Error(w,err.Error(),400)
	}
}

type imagesResponse struct {
	Files []interface{}
	UpdateUrl string
	UpdateExifUrl string
	// Only if rights for user and folder empty
	RemoveFolderUrl string
	FolderPath string
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

type folderRestFul struct {
	Name string
	Link string
	// Link to update tags
	LinkTags string
	// Means that folder also have images to display
	HasImages bool
	Children  []interface{}
	Path      string
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
				Path:node.RelativePath,
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
		"/browserf":s.buildHandler(s.needConnected,s.browseRestful),
		"/imagehd":s.buildHandler(s.needConnected,s.imageHD),
		"/image":s.buildHandler(s.needConnected,s.image),
		"/removeNode":s.buildHandler(s.needAdmin,s.removeNode),
		"/tagsByFolder":s.buildHandler(s.needAdmin,s.updateTagsByFolder),
		"/tagsByDate":s.buildHandler(s.needAdmin,s.updateTagsByDate),
	}
}

func (s Server)defaultHandle(w http.ResponseWriter,r * http.Request){
	path := r.URL.Path[:strings.Index(r.URL.Path[1:],"/")+1]
	if fct,exist := s.pathRoutes[path] ; exist {
		fct(w,r)
	}else {
		// If ? exist, cut before
		pos := len(r.RequestURI)
		if posQMark := strings.Index(r.RequestURI,"?");posQMark != -1 {
			pos = posQMark
		}
		http.ServeFile(w, r, filepath.Join(s.resources, r.RequestURI[1:pos]))
	}
}

func (s Server)Launch(conf *config.Config){
	server := http.ServeMux{}
	server.HandleFunc("/rootFolders",s.getRootFolders)

	server.HandleFunc("/analyse",s.buildHandler(s.needAdmin,s.analyse))
	server.HandleFunc("/delete",s.buildHandler(s.needAdmin,s.delete))
	server.HandleFunc("/addFolder",s.buildHandler(s.needAdmin,s.addFolder))
	server.HandleFunc("/update",s.buildHandler(s.needAdmin,s.update))
	server.HandleFunc("/updateFolder",s.buildHandler(s.needAdmin,s.updateFolder))
	server.HandleFunc("/updateExifFolder",s.buildHandler(s.needAdmin,s.updateExifFolder))
	//server.HandleFunc("/indexFolder",s.indexFolder)
	server.HandleFunc("/uploadFolder",s.buildHandler(s.needAdmin,s.uploadFolder))
	server.HandleFunc("/statUploadRT",s.buildHandler(s.needAdmin,s.statUploadRT))
	server.HandleFunc("/updateExifOfDate",s.buildHandler(s.needAdmin,s.updateExifOfDate))

	server.HandleFunc("/listFolders",s.buildHandler(s.needUser,s.listFolders))
	server.HandleFunc("/count",s.count)
	// By date
	server.HandleFunc("/allDates",s.buildHandler(s.needUser,s.getAllDates))
	server.HandleFunc("/getByDate",s.buildHandler(s.needUser,s.getPhotosByDate))
	server.HandleFunc("/flushTags",s.buildHandler(s.needAdmin,s.flushTags))
	server.HandleFunc("/filterTagsFolder",s.buildHandler(s.needUser,s.filterTagsFolder))
	server.HandleFunc("/filterTagsDate",s.buildHandler(s.needUser,s.filterTagsDate))

	// Share
	server.HandleFunc("/share",s.buildHandler(s.needAdmin,s.manageShare))

	// Security
	server.HandleFunc("/canAdmin",s.buildHandler(s.needNoAccess,s.canAdmin))
	server.HandleFunc("/canAccess",s.buildHandler(s.needNoAccess,s.canAccess))
	server.HandleFunc("/connect",s.buildHandler(s.needNoAccess,s.connect))
	server.HandleFunc("/securityConfig",s.buildHandler(s.needNoAccess,s.getSecurityConfig))

	server.HandleFunc("/",s.buildHandler(s.needNoAccess,s.defaultHandle))

	logger.GetLogger2().Info("Start server on port " + conf.Port)
	err := http.ListenAndServe(":" + conf.Port,&server)
	logger.GetLogger2().Error("Server stopped cause",err)
}

func (s Server)needNoAccess(_ * http.Request)bool{
	return true
}

func (s Server)needAdmin(r * http.Request)bool{
	return s.canAccessAdmin(r)
}

func (s Server)needUser(r * http.Request)bool{
	return s.canAccessUser(r)
}

func (s Server)needShare(r * http.Request)bool{
	id := s.securityAccess.GetUserId(r)
	return s.securityAccess.ShareFolders.Exist(id)
}

func (s Server)needConnected(r * http.Request)bool{
	return s.needUser(r) || s.needShare(r)
}

func (s Server) buildHandler(checkAccess func(r *http.Request)bool,handler func(w http.ResponseWriter,r * http.Request))func(w http.ResponseWriter,r * http.Request){
	return func(w http.ResponseWriter,r * http.Request){
		if !checkAccess(r) {
			s.error403(w,r)
			return
		}
		handler(w,r)
	}
}