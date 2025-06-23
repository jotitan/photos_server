package photos_server

import "net/http"

func (s Server) updateRoutes(server *http.ServeMux) {
	// @Deprecated
	server.HandleFunc("/update", s.buildHandler(s.securityServer.NeedAdmin, s.update))
	server.HandleFunc("/photo/folder/update", s.buildHandler(s.securityServer.NeedAdmin, s.updateFolder))
	server.HandleFunc("/photo/folder/edit-details", s.buildHandler(s.securityServer.NeedAdmin, s.editDetails))
	server.HandleFunc("/photo/folder/move", s.buildHandler(s.securityServer.NeedAdmin, s.moveFolder))
	server.HandleFunc("/photo/folder/exif", s.buildHandler(s.securityServer.NeedAdmin, s.updateExifFolder))
	server.HandleFunc("/photo", s.buildHandler(s.securityServer.NeedAdmin, s.uploadFolder))
	server.HandleFunc("/updateExifOfDate", s.buildHandler(s.securityServer.NeedAdmin, s.updateExifOfDate))
	server.HandleFunc("/sources", s.buildHandler(s.securityServer.NeedAdmin, s.getSources))
}

func (s Server) photoRoutes(server *http.ServeMux) {
	server.HandleFunc("/rootFolders", s.buildHandler(s.securityServer.NeedConnected, s.getRootFolders))
	server.HandleFunc("/analyse", s.buildHandler(s.securityServer.NeedAdmin, s.analyse))
	server.HandleFunc("/delete", s.buildHandler(s.securityServer.NeedAdmin, s.delete))
	// @Deprecated
	server.HandleFunc("/addFolder", s.buildHandler(s.securityServer.NeedAdmin, s.addFolder))
	server.HandleFunc("/statUploadRT", s.buildHandler(s.securityServer.NeedAdmin, s.statUploadRT))
	server.HandleFunc("/getFoldersDetails", s.buildHandler(s.securityServer.NeedConnected, s.getFoldersDetails))
	server.HandleFunc("/count", s.count)
	//server.HandleFunc("/indexFolder",s.indexFolder)
}

func (s Server) videoRoutes(server *http.ServeMux) {
	server.HandleFunc("/video", s.buildHandler(s.securityServer.NeedAdmin, s.video))
	server.HandleFunc("/video/folder", s.buildHandler(s.securityServer.NeedAdmin, s.videoFolder))
	server.HandleFunc("/video/folder/exif", s.buildHandler(s.securityServer.NeedAdmin, s.updateVideoFolderExif))
	server.HandleFunc("/video/date", s.buildHandler(s.securityServer.NeedUser, s.getVideosByDate))
	server.HandleFunc("/video/search", s.buildHandler(s.securityServer.NeedUser, s.searchVideos))
}

func (s Server) tagRoutes(server *http.ServeMux) {
	server.HandleFunc("/tag/tag_folder", s.buildHandler(s.securityServer.NeedAdmin, s.tagFolder))
	server.HandleFunc("/tag/search", s.buildHandler(s.securityServer.NeedUser, s.searchTag))
	server.HandleFunc("/tag/filter_folder", s.buildHandler(s.securityServer.NeedUser, s.filterFolder))
	server.HandleFunc("/tag/search_folder", s.buildHandler(s.securityServer.NeedUser, s.searchTagsOfFolder))
	server.HandleFunc("/tag/peoples", s.buildHandler(s.securityServer.NeedUser, s.getPeoples))
	server.HandleFunc("/tag/add_people", s.buildHandler(s.securityServer.NeedAdmin, s.addPeopleTag))
}

func (s Server) dateRoutes(server *http.ServeMux) {
	server.HandleFunc("/allDates", s.buildHandler(s.securityServer.NeedUser, s.getAllDates))
	server.HandleFunc("/videos/allDates", s.buildHandler(s.securityServer.NeedUser, s.getAllVideosDates))
	server.HandleFunc("/getByDate", s.buildHandler(s.securityServer.NeedUser, s.getPhotosByDate))
	server.HandleFunc("/flushTags", s.buildHandler(s.securityServer.NeedAdmin, s.flushTags))
	server.HandleFunc("/filterTagsFolder", s.buildHandler(s.securityServer.NeedUser, s.filterTagsFolder))
	server.HandleFunc("/filterTagsDate", s.buildHandler(s.securityServer.NeedUser, s.filterTagsDate))
}

func (s Server) securityRoutes(server *http.ServeMux) {
	server.HandleFunc("/security/canAdmin", s.buildHandler(s.securityServer.NeedNoAccess, s.securityServer.CanAdmin))
	server.HandleFunc("/security/canAccess", s.buildHandler(s.securityServer.NeedNoAccess, s.securityServer.CanAccess))
	server.HandleFunc("/security/isGuest", s.buildHandler(s.securityServer.NeedNoAccess, s.securityServer.IsGuest))
	server.HandleFunc("/security/connect", s.buildHandler(s.securityServer.NeedNoAccess, s.connect))
	server.HandleFunc("/security/config", s.buildHandler(s.securityServer.NeedNoAccess, s.getSecurityConfig))
}

func (s *Server) loadPathRoutes() {
	s.pathRoutes = map[string]func(w http.ResponseWriter, r *http.Request){
		"/browserf":         s.buildHandler(s.securityServer.NeedConnected, s.browseRestful),
		"/imagehd":          s.buildHandler(s.securityServer.NeedConnected, s.imageHD),
		"/image":            s.buildHandler(s.securityServer.NeedConnected, s.image),
		"/removeNode":       s.buildHandler(s.securityServer.NeedAdmin, s.removeNode),
		"/tagsByFolder":     s.buildHandler(s.securityServer.NeedAdmin, s.updateTagsByFolder),
		"/tagsByDate":       s.buildHandler(s.securityServer.NeedAdmin, s.updateTagsByDate),
		"/browse_videos_rf": s.buildHandler(s.securityServer.NeedUser, s.browseRestfulVideo),
		"/video_stream":     s.buildHandler(s.securityServer.NeedUser, s.getVideoStream),
		"/cover":            s.buildHandler(s.securityServer.NeedUser, s.getCover),
	}
}
