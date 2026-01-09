package photos_server

import (
	"encoding/json"
	"fmt"
	"github.com/jotitan/photos_server/common"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"github.com/jotitan/photos_server/people_tag"
	"github.com/jotitan/photos_server/progress"
	"github.com/jotitan/photos_server/remote_control"
	"github.com/jotitan/photos_server/security"
	"github.com/jotitan/photos_server/video"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	foldersManager        *FoldersManager
	videoManager          *video.VideoManager
	uploadProgressManager *progress.UploadProgressManager
	resources             string
	// Exist only if garbage exist
	securityAccess *security.SecurityAccess
	securityServer security.SecurityServer
	pathRoutes     map[string]func(w http.ResponseWriter, r *http.Request)
	remoteManager  remote_control.RemoteManager
}

// Create security access from good provider
func (s *Server) setSecurityAccess(conf *config.Config) {
	if s.foldersManager.garbageManager != nil {
		s.securityAccess = security.NewSecurityAccess(conf.Security, conf.Security.MaskForAdmin, []byte(conf.Security.HS256SecretKey))
		s.securityServer = security.NewSecurityServer(s.securityAccess)
		// Check if basic or provider is enabled
		s.securityAccess.SetAccessProvider(security.NewAccessProvider(conf.Security))
	}
}

func NewPhotosServerFromConfig(conf *config.Config) Server {
	uploadProgressManager := progress.NewUploadProgressManager()
	s := Server{
		foldersManager:        NewFoldersManager(*conf, uploadProgressManager),
		videoManager:          video.NewVideoManager(*conf),
		resources:             conf.WebResources,
		uploadProgressManager: uploadProgressManager,
		remoteManager:         remote_control.NewRemoteManager(),
	}
	if err := s.videoManager.Load(); err != nil {
		logger.GetLogger2().Error("Impossible to launch video manager", err)
	}
	s.setSecurityAccess(conf)
	s.loadPathRoutes()
	return s
}

func (s Server) updateExifOfDate(w http.ResponseWriter, r *http.Request) {
	if updates, err := s.foldersManager.updateExifOfDate(r.FormValue("date")); err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		write([]byte(fmt.Sprintf("Update %d exif dates", updates)), w)
	}
}

func (s Server) getSecurityConfig(w http.ResponseWriter, r *http.Request) {
	header(w)
	if s.securityAccess != nil {
		write([]byte(s.securityAccess.GetTypeSecurity()), w)
	} else {
		write([]byte("{\"name\":\"none\"}"), w)
	}
}

func (s Server) connect(w http.ResponseWriter, r *http.Request) {
	header(w)
	if strings.EqualFold(r.Method, http.MethodOptions) {
		w.Header().Set("Allow", "GET")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		return
	}
	if !s.securityAccess.Connect(w, r) {
		http.Error(w, "impossible to connect", 401)
	}
}

func (s Server) flushTags(w http.ResponseWriter, r *http.Request) {
	s.foldersManager.tagManger.flush()
}

func header(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-type", "application/json")
}

func (s Server) filterTagsFolder(w http.ResponseWriter, r *http.Request) {
	s.filterByFct(w, r, s.foldersManager.tagManger.FilterFolder)
}

func (s Server) filterTagsDate(w http.ResponseWriter, r *http.Request) {
	s.filterByFct(w, r, s.foldersManager.tagManger.FilterDate)
}

func (s Server) filterByFct(w http.ResponseWriter, r *http.Request, filter func(string) []string) {
	folders := filter(r.FormValue("value"))
	header(w)
	if data, err := json.Marshal(folders); err == nil {
		write(data, w)
	}
}

func (s Server) getPhotosByDate(w http.ResponseWriter, r *http.Request) {
	if date, err := time.Parse("20060102", r.FormValue("date")); err == nil {
		if photos, exist := s.foldersManager.GetPhotosByDate()[date]; exist {
			converts := s.convertPathsFromInterface(photos, false)
			response := imagesResponse{Files: converts, Tags: s.foldersManager.tagManger.GetTagsByDate(r.FormValue("date"))}
			header(w)
			if data, err := json.Marshal(response); err == nil {
				w.Write(data)
			}
		}
	}
}

func (s Server) searchVideos(w http.ResponseWriter, r *http.Request) {
	results := s.videoManager.Search(r.FormValue("query"))
	convertResults := s.convertVideoPaths(results, false)
	if data, err := json.Marshal(imagesResponse{Files: convertResults}); err == nil {
		w.Write(data)
	}
}

func (s Server) getVideosByDate(w http.ResponseWriter, r *http.Request) {
	if date, err := time.Parse("20060102", r.FormValue("date")); err == nil {
		if videos, exist := s.videoManager.GetVideosByDate()[date]; exist {
			converts := s.convertVideosPathsFromInterface(videos, false)
			response := imagesResponse{Files: converts, Tags: s.foldersManager.tagManger.GetTagsByDate(r.FormValue("date"))}
			header(w)
			if data, err := json.Marshal(response); err == nil {
				w.Write(data)
			}
		}
	}
}

type tagDto struct {
	Value    string
	Color    string
	ToRemove bool
}

func (s Server) updateTagsByFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only post is allowed", 405)
		return
	}
	s.updateTag(w, r, r.URL.Path[14:], s.foldersManager.tagManger.AddTagByFolder, s.foldersManager.tagManger.RemoveByFolder)
}

func (s Server) updateTagsByDate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only post is allowed", 405)
		return
	}
	s.updateTag(w, r, r.URL.Path[12:], s.foldersManager.tagManger.AddTagByDate, s.foldersManager.tagManger.RemoveByDate)
}

func (s Server) updateTag(w http.ResponseWriter, r *http.Request, key string, updateTag func(string, string, string) error, removeTag func(string, string, string)) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "POST" {
		http.Error(w, "Only post is allowed", 405)
		return
	}
	if data, err := ioutil.ReadAll(r.Body); err == nil {
		tag := tagDto{}
		if json.Unmarshal(data, &tag) == nil {
			if tag.ToRemove {
				removeTag(key, tag.Value, tag.Color)
			} else {
				if err := updateTag(key, tag.Value, tag.Color); err != nil {
					http.Error(w, err.Error(), 400)
				}
			}
		}
	}
}

// Return all dates of photos
func (s Server) getAllDates(w http.ResponseWriter, r *http.Request) {
	dates := s.foldersManager.GetAllDates()
	data, _ := json.Marshal(dates)
	header(w)
	write(data, w)
}

func (s Server) getAllVideosDates(w http.ResponseWriter, r *http.Request) {
	dates := s.videoManager.GetAllDates()
	data, _ := json.Marshal(dates)
	header(w)
	write(data, w)
}

func (s Server) count(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	write([]byte(fmt.Sprintf("{\"photos\":%d,\"videos\":%d}", s.foldersManager.Count(), s.videoManager.Count())), w)
}

type tagRequest struct {
	Tag     int      `json:"tag"`
	Folder  int      `json:"folder"`
	Paths   []string `json:"paths"`
	Deleted []string `json:"deleted"`
}

func getTagPath() string {
	wd, _ := os.Getwd()
	return wd
}

func (s Server) tagFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Bad method", http.StatusMethodNotAllowed)
		return
	}
	data, _ := ioutil.ReadAll(r.Body)
	tags := make([]tagRequest, 0)
	if err := json.Unmarshal(data, &tags); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ptm := people_tag.NewPeopleTagManager(getTagPath())
	for _, tag := range tags {
		ptm.Tag(tag.Folder, tag.Tag, tag.Paths, tag.Deleted)
	}
	ptm.Flush()
	w.Write([]byte("ok"))
}

func (s Server) getPeoples(w http.ResponseWriter, r *http.Request) {
	if peoples, err := people_tag.GetPeoplesAsByte(getTagPath()); err == nil {
		w.Write(peoples)
	} else {
		w.Write([]byte("[]"))
		//http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func (s Server) addPeopleTag(w http.ResponseWriter, r *http.Request) {
	if id, err := people_tag.AddPeopleTag(getTagPath(), r.FormValue("name")); err == nil {
		w.Write([]byte(fmt.Sprintf("%d", id)))
	} else {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

// Return all folder with specific tag
func (s Server) filterFolder(w http.ResponseWriter, r *http.Request) {
	idTag, err := strconv.Atoi(r.FormValue("tag"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	ptm := people_tag.NewPeopleTagManager(getTagPath())
	folders := ptm.SearchAllFolder(idTag)
	data, _ := json.Marshal(folders)
	w.Write(data)
}

func (s Server) searchTagsOfFolder(w http.ResponseWriter, r *http.Request) {
	idFolder, err := strconv.Atoi(r.FormValue("folder"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ptm := people_tag.NewPeopleTagManager(getTagPath())
	data, _ := json.Marshal(ptm.SearchFolder(idFolder))
	w.Write(data)

}

func (s Server) searchTag(w http.ResponseWriter, r *http.Request) {
	idFolder, err1 := strconv.Atoi(r.FormValue("folder"))
	idTag, err2 := strconv.Atoi(r.FormValue("tag"))
	if err1 != nil || err2 != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	logger.GetLogger2().Info("Search tag folder", idFolder, idTag)
	ptm := people_tag.NewPeopleTagManager(getTagPath())
	results := ptm.Search(idFolder, idTag)
	data, _ := json.Marshal(results)
	w.Write(data)
}

func error403(w http.ResponseWriter, r *http.Request) {
	logger.GetLogger2().Info("Try to action by", r.Referer(), r.URL)
	http.Error(w, "You can't execute this action", 403)
}

func error404(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Unknown resource", 404)
}

func (s Server) checkPhotoResizer(w http.ResponseWriter, r *http.Request) {
	if s.foldersManager.reducer.CheckResizer() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s Server) getFoldersDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, _ := io.ReadAll(r.Body)
	var folders []string
	json.Unmarshal(data, &folders)
	if data, err := json.Marshal(s.foldersManager.FindNodes(folders)); err == nil {
		w.Write(data)
	} else {
		http.Error(w, "Bad", http.StatusBadRequest)
	}
}

func (s Server) challengeCode(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.getChallenges(w, r)
	}
	if r.Method == http.MethodPost {
		s.createChallenge(w, r)
	}
}

func (s Server) getChallenges(w http.ResponseWriter, _ *http.Request) {
	list := s.remoteManager.ListChallenges()
	data, _ := json.Marshal(list)
	w.Write(data)
}

func (s Server) createChallenge(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	name := r.FormValue("name")
	ch, err := s.remoteManager.CreateChallenge(code, name)
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		return
	}
	// Wait response from challenge for 1 minute max
	timer := time.NewTimer(time.Minute)
	select {
	case <-timer.C:
		s.remoteManager.DeleteChallenge(name)
		w.WriteHeader(http.StatusRequestTimeout)
	case result := <-ch.Chan:
		switch result.Status {
		case remote_control.ChallengeOK:
			http.SetCookie(w, &http.Cookie{
				Name:     "token",
				Value:    result.Token,
				SameSite: http.SameSiteLaxMode,
				HttpOnly: true,
				Path:     "/",
			})
			w.WriteHeader(http.StatusOK)
		case remote_control.ChallengeCancel:
			w.WriteHeader(http.StatusUnauthorized)
		case remote_control.ChallengeBadCode:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func (s Server) answerChallenge(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	name := r.FormValue("name")
	abort := r.FormValue("abort") == "true"

	if s.remoteManager.AnswerChallenge(abort, code, name, getCookieContent(r)) != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
}

func getCookieContent(r *http.Request) string {
	c, err := r.Cookie("token")
	if err != nil {
		return ""
	}
	return c.Value
}

func (s Server) connectRemoteControl(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if _, exists := s.remoteManager.Get(name); exists {
		http.Error(w, "remote already exists", http.StatusBadRequest)
		return
	}
	if r.FormValue("browser") == "true" {
		s.connectSSERemoteControl(w, r)
	} else {
		s.connectRestRemoteControl(w, r)
	}
}

func (s Server) connectSSERemoteControl(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	c := remote_control.NewSSERemoteControler(name, w)
	s.remoteManager.Set(name, c)
	detectEnd := make(chan struct{}, 1)
	go func() {
		<-r.Context().Done()
		logger.GetLogger2().Info("Remove controllable device", name)
		detectEnd <- struct{}{}
	}()
	c.Connect(detectEnd)
	// End connection, remove from map
	logger.GetLogger2().Info("End of connection")
	s.remoteManager.Delete(name)
}

func (s Server) connectRestRemoteControl(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	c := remote_control.NewRestRemoteControler(name, r.FormValue("url"))
	s.remoteManager.Set(name, c)
	detectEnd := make(chan struct{}, 1)
	c.Connect(detectEnd)
	go func() {
		<-detectEnd
		s.remoteManager.Delete(name)
	}()
}

// remoteHeartbeat is called when remote rest client send heartbeat notification
func (s Server) remoteHeartbeat(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if c, exists := s.remoteManager.Get(name); exists {
		c.Heartbeat()
	} else {
		http.Error(w, "impossible to find "+name, http.StatusNotFound)
	}
}

func (s Server) listRemoteControl(w http.ResponseWriter, r *http.Request) {
	data, _ := json.Marshal(s.remoteManager.List())
	w.Write(data)
}

func (s Server) getStatusRemote(w http.ResponseWriter, r *http.Request) {
	c, exists := s.getRemote(w, r)

	if exists {
		c.ReceiveCommand("status", "")
		// Wait ?
		time.Sleep(500 * time.Millisecond)
		data, _ := json.Marshal(c.GetStatus())
		w.Write(data)
	}
}

func (s Server) statusRemote(w http.ResponseWriter, r *http.Request) {
	c, exists := s.getRemote(w, r)
	if exists {
		c.SetStatus(r.FormValue("source"), r.FormValue("current"), r.FormValue("size"))
	}
}

func (s Server) receiveRemoteControl(w http.ResponseWriter, r *http.Request) {
	/*if r.Method != http.MethodPost {
		http.Error(w, "unknown", http.StatusMethodNotAllowed)
		return
	}*/
	c, exists := s.getRemote(w, r)
	if exists {
		c.ReceiveCommand(r.FormValue("event"), r.FormValue("data"))
	}
}

func (s Server) getRemote(w http.ResponseWriter, r *http.Request) (remote_control.RemoteControler, bool) {
	name := r.FormValue("name")
	c, exists := s.remoteManager.Get(name)
	if !exists {
		http.Error(w, "unknown", http.StatusBadRequest)
		return nil, false
	}
	return c, true
}

// @Deprecated
func (s Server) addFolder(w http.ResponseWriter, r *http.Request) {
	/*folder := r.FormValue("folder")
	*source := r.FormValue("source")
	src, err := s.foldersManager.Sources.getSource(source)
	if err != nil {
		writeError(w, err.Error())
		return
	}*
	forceRotate := r.FormValue("forceRotate") == "true"
	logger.GetLogger2().Info("Add folder", folder, "and forceRotate :", forceRotate)
	p := s.foldersManager.uploadProgressManager.AddUploader(0)
	//s.foldersManager.AddFolderToSource(folder, src, forceRotate, detailUploadFolder{}, p)
	s.foldersManager.AddFolder(folder, forceRotate, detailUploadFolder{}, p)*/
}

func (s Server) video(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		s.deleteVideo(w, r)
		break
	case http.MethodPost:
		s.uploadVideo(w, r)
		break

	}
}

func (s Server) videoFolder(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		s.deleteVideoFolder(w, r)
		break
	case http.MethodGet:
		s.getRootVideoFolders(w, r)
		break
	}
}

func (s Server) deleteVideoFolder(w http.ResponseWriter, r *http.Request) {
	header(w)
	path := r.FormValue("path")
	logger.GetLogger2().Info("Delete folder", path)
	if err := s.videoManager.RemoveFolder(path); err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		write([]byte("{\"success\":true}"), w)
	}
}

func (s Server) updateVideoFolderExif(w http.ResponseWriter, r *http.Request) {
	header(w)
	path := r.FormValue("path")
	if err := s.videoManager.UpdateExifFolder(path); err == nil {
		write([]byte("{\"success\":true}"), w)
	} else {
		http.Error(w, err.Error(), 400)
	}
}

func (s Server) deleteVideo(w http.ResponseWriter, r *http.Request) {
	header(w)
	if s.foldersManager.garbageManager == nil {
		error403(w, r)
		return
	}
	path := r.FormValue("path")
	if err := s.videoManager.Delete(path, s.foldersManager.garbageManager.MoveOriginalFileFromPath); err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		write([]byte("{\"success\":true}"), w)
	}
}

func (s Server) delete(w http.ResponseWriter, r *http.Request) {
	// TODO add delete METHOD
	header(w)
	if s.foldersManager.garbageManager == nil {
		error403(w, r)
		return
	}
	data, _ := io.ReadAll(r.Body)
	deletions := make([]string, 0)
	if json.Unmarshal(data, &deletions) == nil {
		imagesPath := make([]string, len(deletions))
		for i, deletion := range deletions {
			imagesPath[i] = strings.Replace(deletion, "/imagehd/", "", -1)
		}
		successDeletions := s.foldersManager.garbageManager.Remove(imagesPath)
		write([]byte(fmt.Sprintf("{\"success\":%d,\"errors\":%d}", successDeletions, len(imagesPath)-successDeletions)), w)
	}
}

func (s Server) analyse(w http.ResponseWriter, r *http.Request) {
	folder := r.FormValue("folder")
	logger.GetLogger2().Info("Analyse", folder)
	nodes := FoldersManager{}.Analyse("", folder)
	if data, err := json.Marshal(nodes); err == nil {
		w.Header().Set("Content-type", "application/json")
		write(data, w)
	}
}

var splitStream, _ = regexp.Compile("(/stream/?)")

// Use HLS to stream video
func (s Server) getVideoStream(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[14:]
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// Path can be
	// /video_stream/path/stream => root of video path, return master.m3u8, which contains versions
	// /video_stream/path/stream/v1/file => subversion and segment file

	// search for "/stream/" sequence
	splits := splitStream.Split(path, -1)
	if len(splits) != 2 {
		http.Error(w, "impossible to read path", 404)
		return
	}
	if strings.EqualFold("", splits[1]) {
		// Serve master.m3u8
		if file, err := s.videoManager.GetVideoMaster(splits[0]); err == nil {
			logger.GetLogger2().Info("Video master", splits[0])
			http.ServeFile(w, r, file)
		} else {
			http.Error(w, "impossible to find", 404)
		}

	} else {
		if file, err := s.videoManager.GetVideoSegment(splits[0], splits[1]); err == nil {
			http.ServeFile(w, r, file)
		} else {
			http.Error(w, "impossible to find", 404)
		}
	}
}

func (s Server) getCover(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[7:]
	if file, err := s.videoManager.GetCover(path); err == nil {
		defer file.Close()
		if _, err := io.Copy(w, file); err != nil {
			// Image not found
			error404(w, r)
		}
		return
	}
	error404(w, r)
}

func (s Server) image(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[7:]
	if !s.securityServer.CanReadPath(getCleanPath(path), r) {
		error403(w, r)
		return
	}
	s.writeImage(w, filepath.Join(s.foldersManager.reducer.GetCache(), path))
}

func getCleanPath(path string) string {
	if !strings.Contains(path, "/") {
		return path
	}
	return path[:strings.LastIndex(path, "/")]
}

func (s Server) writeImage(w http.ResponseWriter, path string) {
	w.Header().Set("Content-type", "image/jpeg")
	if file, err := os.Open(path); err == nil {
		defer file.Close()
		if _, e := io.Copy(w, file); e != nil {
			http.Error(w, "Error during image rendering", 404)
		}
	} else {
		http.Error(w, "Image not found", 404)
	}
}

func (s Server) removeNode(w http.ResponseWriter, r *http.Request) {
	header(w)
	if s.foldersManager.garbageManager == nil {
		error403(w, r)
		return
	}
	path := r.URL.Path[12:]
	if err := s.foldersManager.RemoveNode(path); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	logger.GetLogger2().Info("Remove node", path)
	write([]byte("success"), w)
}

// Return original image
func (s Server) imageHD(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[9:]
	if !s.securityServer.CanReadPath(getCleanPath(path), r) {
		error403(w, r)
		return
	}
	// Find absolute path based on first folder
	if node, _, err := s.foldersManager.FindNode(path); err != nil {
		http.Error(w, "Impossible to find image", 404)
	} else {
		s.writeImage(w, node.GetAbsolutePath(s.foldersManager.Sources))
	}
}

func (s Server) update(w http.ResponseWriter, r *http.Request) {
	logger.GetLogger2().Info("Launch update")
	if err := s.foldersManager.Update(); err != nil {
		logger.GetLogger2().Error(err.Error())
	}
}

func (s Server) uploadVideo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	pathFolder := r.FormValue("path")
	if r.MultipartForm == nil {
		logger.GetLogger2().Error("impossible to upload videos in " + pathFolder)
		http.Error(w, "impossible to upload videos in "+pathFolder, 400)
		return
	}
	if strings.EqualFold("", pathFolder) {
		http.Error(w, "need to specify a path", 400)
		return
	}
	videoFile, videoName, cover, coverName := extractVideos(r)
	logger.GetLogger2().Info("Launch upload video", videoName, "in folder", pathFolder, "with cover", coverName)
	if progresser, err := s.videoManager.UploadVideoGlobal(pathFolder, videoFile, videoName, cover, coverName, s.foldersManager.uploadProgressManager); err != nil {
		http.Error(w, "Bad request "+err.Error(), 400)
		logger.GetLogger2().Error("Impossible to upload video : ", err.Error())
	} else {
		write([]byte(fmt.Sprintf("{\"status\":\"running\",\"id\":\"%s\"}", progresser.GetId())), w)
	}
}

func (s Server) getSources(w http.ResponseWriter, r *http.Request) {
	sources := s.foldersManager.Sources.getSources()
	data, _ := json.Marshal(sources)
	w.Write(data)
}

// Update a specific folder, faster than all folders
// Return an id to monitor upload
func (s Server) uploadFolder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != http.MethodPost {
		http.Error(w, "only POST method is allowed", 405)
		return
	}

	details := detailUploadFolder{r.FormValue("source"), r.FormValue("path"), r.FormValue("title"), r.FormValue("description")}
	addToFolder := strings.EqualFold(r.FormValue("addToFolder"), "true")
	if r.MultipartForm == nil {
		logger.GetLogger2().Error("impossible to upload photos in " + details.path)
		http.Error(w, "impossible to upload photos in "+details.path, 400)
		return
	}
	if strings.EqualFold("", details.path) {
		http.Error(w, "need to specify a path", 400)
		return
	}
	files, names := extractFiles(r)
	logger.GetLogger2().Info("Launch upload folder :", details.path)

	if progresser, err := s.foldersManager.UploadFolder(details, files, names, addToFolder); err != nil {
		http.Error(w, "Bad request "+err.Error(), 400)
		logger.GetLogger2().Error("Impossible to upload folder : ", err.Error())
	} else {
		write([]byte(fmt.Sprintf("{\"status\":\"running\",\"id\":\"%s\"}", progresser.GetId())), w)
	}
}

func (s Server) statUploadRT(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if sse, err := s.uploadProgressManager.AddSSE(id, w, r); err == nil {
		// Block to write messages
		sse.Watch()
		logger.GetLogger2().Info("End watch")
	} else {
		logger.GetLogger2().Info("Impossible to watch upload", err.Error())
		http.Error(w, err.Error(), 404)
	}
}

func extractVideos(r *http.Request) (multipart.File, string, multipart.File, string) {
	files, names := extractFiles(r)
	if strings.EqualFold("true", r.FormValue("has_cover")) && len(files) == 2 {
		// Last files is cover
		return files[0], names[0], files[1], names[1]
	}
	return files[0], names[0], nil, ""
}

func extractFiles(r *http.Request) ([]multipart.File, []string) {
	files := make([]multipart.File, 0, len(r.MultipartForm.File))
	names := make([]string, 0, len(r.MultipartForm.File))
	for _, headers := range r.MultipartForm.File {
		if len(headers) > 0 {
			header := headers[0]
			if file, err := header.Open(); err == nil {
				files = append(files, file)
				names = append(names, filepath.Base(header.Filename))
			}
		}
	}
	return files, names
}

func (s Server) updateExifFolder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	folder := r.FormValue("folder")
	forceSize := r.FormValue("force") == "true"
	logger.GetLogger2().Info("Launch update exif folder :", folder)
	treatError(s.foldersManager.UpdateExif(folder, forceSize), folder, w)
}

func treatError(err error, folder string, w http.ResponseWriter) {
	if err != nil {
		logger.GetLogger2().Error(err.Error())
	} else {
		logger.GetLogger2().Info("End update folder", folder)
		write([]byte("success"), w)
	}
}

// Update a specific folder, faster than all folders
func (s Server) updateFolder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	folder := r.FormValue("folder")
	logger.GetLogger2().Info("Launch update folder :", folder)
	up := s.foldersManager.uploadProgressManager.AddUploader(0)
	treatError(s.foldersManager.UpdateFolder(folder, up), folder, w)
}

// Update a specific folder, faster than all folders
func (s Server) editDetails(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "POST" {
		http.Error(w, "Only post is allowed", http.StatusMethodNotAllowed)
		return
	}
	data, _ := io.ReadAll(r.Body)
	var details FolderDto
	json.Unmarshal(data, &details)
	treatError(s.foldersManager.UpdateDetails(details), details.Path, w)
}

// move a specific folder
func (s Server) moveFolder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	fromPath := r.FormValue("from")
	toPath := r.FormValue("to")
	logger.GetLogger2().Info("Launch move folder", fromPath, toPath)
	err := s.foldersManager.MoveFolder(fromPath, toPath)
	treatError(err, fromPath, w)
}

// Index an existing folder
/*func (s Server) indexFolder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	path := r.FormValue("path")
	folder := r.FormValue("folder")
	logger.GetLogger2().Info("Index :", folder, "with path", path)
	if err := s.foldersManager.IndexFolder(path, folder); err != nil {
		logger.GetLogger2().Error(err.Error())
	} else {
		logger.GetLogger2().Info("End update folder", folder)
		write([]byte("success"), w)
	}
}*/

func (s Server) getSharesFolder(w http.ResponseWriter, r *http.Request) {
	id := s.securityAccess.GetUserId(r)
	if shares, err := s.securityAccess.ShareFolders.Get(id); err != nil {
		error403(w, r)
	} else {
		header(w)
		logger.GetLogger2().Info("Get shares for", id)
		sharesNode := make([]*Node, 0, len(shares))
		for _, share := range shares {
			if node, _, err := s.foldersManager.FindNode(share); err == nil {
				sharesNode = append(sharesNode, node)
			}
		}
		root := folderRestFul{Name: "Racine", Link: "", Children: s.convertPaths(sharesNode, true)}
		if data, err := json.Marshal(root); err == nil {
			write(data, w)
		}
	}
}

func (s Server) getRootFolders(w http.ResponseWriter, r *http.Request) {
	// If guest, return share folder
	if s.securityAccess.IsGuest(r) {
		s.getSharesFolder(w, r)
		return
	}

	logger.GetLogger2().Info("Get root folders")
	header(w)
	/*nodes := make([]*Node, 0, len(s.foldersManager.Folders))
	for _, node := range s.foldersManager.Folders {
		nodes = append(nodes, node)
	}*/
	nodes := make([]*Node, 0, len(s.foldersManager.Sources))
	for _, src := range s.foldersManager.Sources {
		nodes = append(nodes, &Node{Name: src.Name, RelativePath: src.Name, Files: src.Files, IsFolder: true})
	}
	//root := folderRestFul{Name: "Racine", Link: "", Children: s.convertPaths(nodes, true)}
	//if data, err := json.Marshal(root); err == nil {
	if data, err := json.Marshal(s.convertPaths(nodes, true)); err == nil {
		write(data, w)
	}
}

func (s Server) getRootVideoFolders(w http.ResponseWriter, r *http.Request) {
	logger.GetLogger2().Info("Get root videos folders")
	header(w)
	nodes := s.videoManager.GetSortedFolders()
	//root := folderRestFul{Name: "Racine", Link: "", Children: s.convertVideoPaths(nodes, true)}
	if data, err := json.Marshal(s.convertVideoPaths(nodes, true)); err == nil {
		write(data, w)
	}
}

func (s Server) browseRestfulVideo(w http.ResponseWriter, r *http.Request) {
	header(w)
	path := r.URL.Path[17:]
	if node, _, err := s.videoManager.FindVideoNode(path[1:]); err == nil {
		nodes := make([]*video.VideoNode, 0, len(node.Files))
		for _, file := range node.Files {
			nodes = append(nodes, file)
		}
		folder := folderRestFul{Name: node.Name, Children: s.convertVideoPaths(nodes, false)}
		if s.securityServer.CanAccessAdmin(r) {
			folder.RemoveFolderUrl = fmt.Sprintf("/video/folder?path=%s", path[1:])
			folder.UpdateExifFolderUrl = fmt.Sprintf("/video/folder/exif?path=%s", path[1:])
		}
		if data, err := json.Marshal(folder); err == nil {
			write(data, w)
		}
	}
}

func (s Server) browseRestful(w http.ResponseWriter, r *http.Request) {
	// Check if user can access, if not, check if is invited
	if !s.securityAccess.CheckJWTTokenAccess(r) {
		error403(w, r)
		return
	}

	// Return all tree
	header(w)
	path := r.URL.Path[9:]
	if !s.securityServer.CanReadPath(path[1:], r) {
		error403(w, r)
		return
	}
	logger.GetLogger2().Info("Browse restfull receive request", path)
	if files, node, err := s.foldersManager.Browse(path); err == nil {
		formatedFiles := s.convertPaths(files, false)
		tags := s.foldersManager.tagManger.GetTagsByFolder(path[1:])
		folderResponse := imagesResponse{Id: node.Id,
			Files:         formatedFiles,
			UpdateExifUrl: fmt.Sprintf("/photo/folder/exif?folder=%s", path[1:]),
			UpdateUrl:     fmt.Sprintf("/photo/folder/update?folder=%s", path[1:]),
			FolderPath:    path[1:], Tags: tags,
			Title:       node.Title,
			Description: node.Description}
		if s.securityServer.CanAccessAdmin(r) {
			folderResponse.RemoveFolderUrl = "/removeNode" + path
		}
		if data, err := json.Marshal(folderResponse); err == nil {
			write(data, w)
		}
	} else {
		logger.GetLogger2().Info("Impossible to browse", path, err.Error())
		http.Error(w, err.Error(), 400)
	}
}

func (s Server) checkNodeExist(path string) bool {
	_, _, err := s.foldersManager.FindNode(path)
	return err == nil
}

// Return who an access the addShare
func (s Server) getShares(w http.ResponseWriter, r *http.Request) {
	users := s.securityAccess.ShareFolders.GetUsersOfPath(r.FormValue("path"))
	if data, err := json.Marshal(users); err == nil {
		write(data, w)
	} else {
		http.Error(w, "impossible to get users for path", 404)
	}
}

func (s Server) addShare(w http.ResponseWriter, r *http.Request) {
	if err := s.securityAccess.ShareFolders.Add(r.FormValue("user"), r.FormValue("path"), s.checkNodeExist); err != nil {
		http.Error(w, err.Error(), 400)
	}
}

func (s Server) manageShare(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.addShare(w, r)
		break
	case http.MethodGet:
		s.getShares(w, r)
		break
	case http.MethodDelete:
		s.removeShare(w, r)
		break
	}
}

func (s Server) removeShare(w http.ResponseWriter, r *http.Request) {
	if err := s.securityAccess.ShareFolders.Remove(r.FormValue("user"), r.FormValue("path"), s.checkNodeExist); err != nil {
		http.Error(w, err.Error(), 400)
	}
}

type imagesResponse struct {
	Files         []interface{}
	UpdateUrl     string
	UpdateExifUrl string
	// Only if rights for user and folder empty
	RemoveFolderUrl string
	FolderPath      string
	Tags            []*Tag
	Id              int
	Description     string
	Title           string
}

// Restful representation : real link instead real path
type imageRestFul struct {
	Name          string
	ThumbnailLink string
	ImageLink     string
	HdLink        string
	Width         int
	Height        int
	Date          time.Time
	Orientation   int
}

type folderRestFul struct {
	Name                string
	Link                string
	RemoveFolderUrl     string
	UpdateExifFolderUrl string
	// Link to update tags
	LinkTags string
	// Means that folder also have images to display
	HasImages bool
	Children  []interface{}
	Path      string
	Id        int
}

func (s Server) newImageRestful(node *Node) imageRestFul {
	return imageRestFul{
		Name: node.Name, Width: node.Width, Height: node.Height, Date: node.Date,
		HdLink:        filepath.ToSlash(filepath.Join("/imagehd", node.RelativePath)),
		ThumbnailLink: filepath.ToSlash(filepath.Join("/image", s.foldersManager.GetSmallImageName(*node))),
		ImageLink:     filepath.ToSlash(filepath.Join("/image", s.foldersManager.GetMiddleImageName(*node)))}
}

func (s Server) convertPathsFromInterface(nodes []common.INode, onlyFolders bool) []interface{} {
	formatNodes := make([]*Node, len(nodes))
	for i, n := range nodes {
		formatNodes[i] = n.(*Node)
	}
	return s.convertPaths(formatNodes, onlyFolders)
}

// Convert node to restful response
func (s Server) convertPaths(nodes []*Node, onlyFolders bool) []interface{} {
	files := make([]interface{}, 0, len(nodes))
	for _, node := range nodes {
		if !node.IsFolder {
			if !onlyFolders {
				files = append(files, s.newImageRestful(node))
			}
		} else {
			folder := folderRestFul{Name: node.Name,
				Path:     node.RelativePath,
				Id:       node.Id,
				Link:     filepath.ToSlash(filepath.Join("/browserf", node.RelativePath)),
				LinkTags: filepath.ToSlash(filepath.Join("/tagsByFolder", node.RelativePath)),
			}
			if onlyFolders {
				s.convertSubFolders(node, &folder)
			}
			files = append(files, folder)
		}
	}
	return files
}

func (s Server) convertVideosPathsFromInterface(nodes []common.INode, onlyFolders bool) []interface{} {
	formatNodes := make([]*video.VideoNode, len(nodes))
	for i, n := range nodes {
		formatNodes[i] = n.(*video.VideoNode)
	}
	return s.convertVideoPaths(formatNodes, onlyFolders)
}

// Convert node to restful response
func (s Server) convertVideoPaths(nodes []*video.VideoNode, onlyFolders bool) []interface{} {
	files := make([]interface{}, 0, len(nodes))
	for _, node := range nodes {
		if !node.IsFolder {
			if !onlyFolders {
				files = append(files, video.NewVideoNodeDto(*node))
			}
		} else {
			// If node contains subfolers, define childrens
			folder := folderRestFul{Name: node.Name,
				Path:     node.RelativePath,
				Children: s.getVideosChildren(node),
				Link:     filepath.ToSlash(filepath.Join("/browse_videos_rf", node.RelativePath)),
			}
			files = append(files, folder)
		}
	}
	return files
}

func (s Server) getVideosChildren(n *video.VideoNode) []interface{} {
	children := make([]*video.VideoNode, 0, len(n.Files))

	for _, f := range n.Files {
		children = append(children, f)
	}
	return s.convertVideoPaths(children, true)
}

func (s Server) convertSubFolders(node *Node, folder *folderRestFul) {
	// Relaunch on subfolders
	subNodes := make([]*Node, 0, len(node.Files))
	hasImages := false
	for _, n := range node.Files {
		subNodes = append(subNodes, n)
		if !n.IsFolder {
			hasImages = true
		}
	}
	folder.Children = s.convertPaths(subNodes, true)
	folder.HasImages = hasImages
}

func (s Server) defaultHandle(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[:strings.Index(r.URL.Path[1:], "/")+1]
	if fct, exist := s.pathRoutes[path]; exist {
		fct(w, r)
	} else {
		// If ? exist, cut before
		pos := len(r.RequestURI)
		if posQMark := strings.Index(r.RequestURI, "?"); posQMark != -1 {
			pos = posQMark
		}
		http.ServeFile(w, r, filepath.Join(s.resources, r.RequestURI[1:pos]))
	}
}

func (s Server) Launch(conf *config.Config) {
	server := http.ServeMux{}

	s.photoRoutes(&server)
	s.updateRoutes(&server)
	s.videoRoutes(&server)
	s.tagRoutes(&server)
	s.dateRoutes(&server)
	s.securityRoutes(&server)
	s.remoteRoutes(&server)

	server.HandleFunc("/share", s.buildHandler(s.securityServer.NeedAdmin, s.manageShare))
	server.HandleFunc("/", s.buildHandler(s.securityServer.NeedNoAccess, s.defaultHandle))

	logger.GetLogger2().Info("Start server on port " + conf.Port)
	err := http.ListenAndServe(":"+conf.Port, &server)
	logger.GetLogger2().Error("Server stopped cause", err)
}

func (s Server) buildHandler(checkAccess func(r *http.Request) bool, handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkAccess(r) {
			error403(w, r)
			return
		}
		handler(w, r)
	}
}

func (s *Server) HealthCheck(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(200)
}

func write(data []byte, w http.ResponseWriter) {
	if _, err := w.Write(data); err != nil {
		logger.GetLogger2().Error("Error when write data", err)
	}
}

func writeError(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(message))
}
