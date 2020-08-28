package photos_server

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jotitan/photos_server/config"
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

type SecurityAccess struct {
	maskForAdmin string
	// true only if username and password exist
	userAccessEnable bool
	username string
	password string
	hs256SecretKey []byte
}

func NewSecurityAccess(maskForAdmin,username,password string,hs256SecretKey []byte)*SecurityAccess{
	sa := SecurityAccess{maskForAdmin:maskForAdmin,userAccessEnable:false}
	if !strings.EqualFold("",username) && !strings.EqualFold("",password) && len(hs256SecretKey) > 0{
		sa.userAccessEnable = true
		sa.username = username
		sa.password = password
		sa.hs256SecretKey = hs256SecretKey
		logger.GetLogger2().Info("Use basic with JWT security mode")
	}else{
		logger.GetLogger2().Info("Use simple security mode")
	}
	return &sa
}

func (sa SecurityAccess)checkMaskAccess(r * http.Request)bool{
	return !strings.EqualFold("",sa.maskForAdmin) && (
		strings.Contains(r.Referer(),sa.maskForAdmin) ||
			strings.Contains(r.RemoteAddr,sa.maskForAdmin))
}

func (sa SecurityAccess)getJWTCookie(r * http.Request)*http.Cookie{
	for _,c := range r.Cookies() {
		if strings.EqualFold("token",c.Name) {
			return c
		}
	}
	return nil
}

func (sa SecurityAccess)checkAccess(r * http.Request)bool{
	// Two cases : on local network or with basic authent
	if sa.checkMaskAccess(r) {
		return true
	}
	// Check authorisation
	if sa.userAccessEnable {
		if username, password, ok := r.BasicAuth(); ok && !strings.EqualFold("", username) {
			success :=  strings.EqualFold(username, sa.username) && strings.EqualFold(password, sa.password)
			if success {
				logger.GetLogger2().Info(fmt.Sprintf("User %s connected",username))
			}else{
				logger.GetLogger2().Error(fmt.Sprintf("User %s try to connect but fail",username))
			}
			return success
		}
	}
	return false
}

func (sa SecurityAccess)checkJWTToken(r * http.Request)bool{
	// Check if jwt token exist in a cookie and is valid. Create by server during first connexion
	if token := sa.getJWTCookie(r); token != nil {
		if jwtToken,err :=jwt.Parse(token.Value,func(token *jwt.Token) (interface{}, error) {return sa.hs256SecretKey,nil});err == nil {
			if strings.EqualFold(sa.username,jwtToken.Claims.(jwt.MapClaims)["username"].(string)) {
				return true
			}
		}
	}
	return false
}


func (sa SecurityAccess)setJWTToken(username string,w http.ResponseWriter){
	// No expiracy
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,jwt.MapClaims{"username":username})
	if sToken,err := token.SignedString(sa.hs256SecretKey); err == nil {
		cookie := http.Cookie{}
		cookie.Name="token"
		cookie.Path="/"
		cookie.Value=sToken
		cookie.HttpOnly=true
		cookie.SameSite=http.SameSiteLaxMode
		http.SetCookie(w,&cookie)
	}
}

func (sa SecurityAccess) CheckAdmin(w http.ResponseWriter,r * http.Request)bool{
	// Check if jwt token exist and valid. Create by server during first connexion
	if sa.checkJWTToken(r){
		// If jwt already exist, don't create a new
		return true
	}
	if sa.checkAccess(r) {
		sa.setJWTToken(sa.username,w)
		return true
	}
	return false
}

type Server struct {
	foldersManager *foldersManager
	resources string
	// Exist only if garbage exist
	securityAccess *SecurityAccess
	pathRoutes map[string]func(w http.ResponseWriter,r *http.Request)
}

func (s *Server)setSecurityAccess(conf *config.Config){
	if s.foldersManager.garbageManager != nil {
		s.securityAccess = NewSecurityAccess(conf.Security.MaskForAdmin,conf.Security.Username,conf.Security.Password,[]byte(conf.Security.HS256SecretKey))
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

func (s Server)canAccessAdmin(w http.ResponseWriter,r * http.Request)bool{
	return s.securityAccess!= nil && s.securityAccess.CheckAdmin(w,r)
}

func (s Server)updateExifOfDate(w http.ResponseWriter,r * http.Request){
	if updates,err := s.foldersManager.updateExifOfDate(r.FormValue("date")) ; err != nil {
		http.Error(w,err.Error(),400)
	}else{
		w.Write([]byte(fmt.Sprintf("Update %d exif dates",updates)))
	}
}

func (s Server)canAdmin(w http.ResponseWriter,r * http.Request){
	header(w)
	w.Write([]byte(fmt.Sprintf("{\"can\":%t}",s.canAccessAdmin(w,r))))
}

func (s Server)flushTags(w http.ResponseWriter,r * http.Request){
	s.foldersManager.tagManger.flush()
}

func header(w http.ResponseWriter){
	w.Header().Set("Access-Control-Allow-Origin","*")
	w.Header().Set("Content-type","application/json")
}

func (s Server)filterTagsFolder(w http.ResponseWriter,r * http.Request){
	folders := s.foldersManager.tagManger.FilterFolder(r.FormValue("value"))
	header(w)
	if data,err := json.Marshal(folders) ; err == nil {
		w.Write(data)
	}
}

func (s Server)filterTagsDate(w http.ResponseWriter,r * http.Request){
	folders := s.foldersManager.tagManger.FilterDate(r.FormValue("value"))
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
	logger.GetLogger2().Info("Try to delete by",r.Referer())
	http.Error(w,"You can't execute this action",403)
}

func (s Server)addFolder(w http.ResponseWriter,r * http.Request){
	if !s.canAccessAdmin(w,r) {
		s.error403(w,r)
		return
	}
	folder := r.FormValue("folder")
	forceRotate := r.FormValue("forceRotate") == "true"
	logger.GetLogger2().Info("Add folder",folder,"and forceRotate :",forceRotate)
	p := s.foldersManager.uploadProgressManager.addUploader(0)
	s.foldersManager.AddFolder(folder,forceRotate,p)
}

func (s Server)delete(w http.ResponseWriter,r * http.Request){
	header(w)
	if !s.canAccessAdmin(w,r) && s.foldersManager.garbageManager != nil{
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
	header(w)
	if !s.canAccessAdmin(w,r) && s.foldersManager.garbageManager != nil{
		s.error403(w,r)
		return
	}
	path := r.URL.Path[12:]
	if err := s.foldersManager.RemoveNode(path) ; err != nil {
		http.Error(w,err.Error(),400)
	}else{
		logger.GetLogger2().Info("Remove node",path)
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
	if !s.canAccessAdmin(w,r) {
		s.error403(w,r)
		return
	}
	logger.GetLogger2().Info("Launch update")
	if err := s.foldersManager.Update() ; err != nil {
		logger.GetLogger2().Error(err.Error())
	}
}


// Update a specific folder, faster than all folders
// Return an id to monitor upload
func (s Server)uploadFolder(w http.ResponseWriter,r * http.Request){
	if !s.canAccessAdmin(w,r) {
		s.error403(w,r)
		return
	}
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
	if !s.canAccessAdmin(w,r) {
		s.error403(w,r)
		return
	}
	id := r.FormValue("id")
	if sse,err := s.foldersManager.uploadProgressManager.addSSE(id,w,r) ; err == nil {
		// Block to write messages
		sse.watch()
		logger.GetLogger2().Info("End watch")
	}else{
		http.Error(w,err.Error(),404)
	}
}

func (s Server)statUpload(w http.ResponseWriter,r * http.Request){
	if !s.canAccessAdmin(w,r) {
		s.error403(w,r)
		return
	}
	id := r.FormValue("id")
	if stat,err := s.foldersManager.uploadProgressManager.getStatUpload(id) ; err == nil {
		w.Write([]byte(fmt.Sprintf("{\"Done\":%d,\"total\":%d}",stat.done,stat.total)))
	}else{
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
	if !s.canAccessAdmin(w,r) {
		s.error403(w,r)
		return
	}
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
	if !s.canAccessAdmin(w,r) {
		s.error403(w,r)
		return
	}
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
	if !s.canAccessAdmin(w,r) {
		s.error403(w,r)
		return
	}
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

func (s Server)getRootFolders(w http.ResponseWriter,r * http.Request){
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
	// Extract folder
	header(w)
	path := r.URL.Path[9:]
	logger.GetLogger2().Info("Browse restfull receive request",path)
	if files,err := s.foldersManager.Browse(path) ; err == nil {
		formatedFiles := s.convertPaths(files,false)
		tags :=s.foldersManager.tagManger.GetTagsByFolder(path[1:])
		imgResponse := imagesResponse{Files:formatedFiles,UpdateExifUrl:"/updateExifFolder?folder=" + path[1:],UpdateUrl:"/updateFolder?folder=" + path[1:],FolderPath:path[1:],Tags:tags}
		if s.canAccessAdmin(w,r){
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
		//logger.GetLogger2().Info("Receive request", r.URL, r.URL.Path)
		http.ServeFile(w, r, filepath.Join(s.resources, r.RequestURI[1:]))
	}
}

func (s Server)Launch(conf *config.Config){
	server := http.ServeMux{}
	server.HandleFunc("/analyse",s.analyse)
	server.HandleFunc("/delete",s.delete)
	server.HandleFunc("/addFolder",s.addFolder)
	server.HandleFunc("/rootFolders",s.getRootFolders)
	server.HandleFunc("/update",s.update)
	server.HandleFunc("/updateFolder",s.updateFolder)
	server.HandleFunc("/updateExifFolder",s.updateExifFolder)
	//server.HandleFunc("/indexFolder",s.indexFolder)
	server.HandleFunc("/uploadFolder",s.uploadFolder)
	server.HandleFunc("/statUpload",s.statUpload)
	server.HandleFunc("/statUploadRT",s.statUploadRT)
	server.HandleFunc("/listFolders",s.listFolders)
	server.HandleFunc("/canAdmin",s.canAdmin)
	server.HandleFunc("/updateExifOfDate",s.updateExifOfDate)
	server.HandleFunc("/count",s.count)
	// By date
	server.HandleFunc("/allDates",s.getAllDates)
	server.HandleFunc("/getByDate",s.getPhotosByDate)
	server.HandleFunc("/flushTags",s.flushTags)
	server.HandleFunc("/filterTagsFolder",s.filterTagsFolder)
	server.HandleFunc("/filterTagsDate",s.filterTagsDate)
	server.HandleFunc("/",s.defaultHandle)

	logger.GetLogger2().Info("Start server on port " + conf.Port)
	err := http.ListenAndServe(":" + conf.Port,&server)
	logger.GetLogger2().Error("Server stopped cause",err)
}