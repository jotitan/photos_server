package security

import (
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"net/http"
)

// Give method to manage security on server

type SecurityServer struct {
	access *SecurityAccess
}

func NewSecurityServer(access *SecurityAccess) SecurityServer {
	return SecurityServer{access}
}

func (s SecurityServer) CanAccessAdmin(r *http.Request) bool {
	return s.access != nil && s.access.CheckJWTTokenAdminAccess(r)
}

func (s SecurityServer) CanAccessUser(r *http.Request) bool {
	return s.access != nil && s.access.CheckJWTTokenRegularAccess(r)
}

func (s SecurityServer) CanAdmin(w http.ResponseWriter, r *http.Request) {
	header(w)
	if !s.CanAccessAdmin(r) {
		http.Error(w, "access denied, only admin", 403)
	}
}

// Can access is enable if oauth2 is configured, otherwise, only admin is checked
func (s SecurityServer) CanAccess(w http.ResponseWriter, r *http.Request) {
	header(w)
	if s.access != nil {
		if !s.access.CheckJWTTokenAccess(r) {
			http.Error(w, "access denied", 401)
		}
	}
}

func (s SecurityServer) IsGuest(w http.ResponseWriter, r *http.Request) {
	header(w)
	if s.access != nil {
		write([]byte(fmt.Sprintf("{\"guest\":%t}", s.access.IsGuest(r))), w)
	}
}

func (s SecurityServer) CanReadPath(path string, r *http.Request) bool {
	return s.CanAccessUser(r) || s.access.ShareFolders.CanRead(s.access.GetUserId(r), path)
}

func (s SecurityServer) NeedShare(r *http.Request) bool {
	id := s.access.GetUserId(r)
	return s.access.IsGuest(r) && s.access.ShareFolders.Exist(id)
}

func (s SecurityServer) NeedConnected(r *http.Request) bool {
	return s.NeedUser(r) || s.NeedShare(r)
}

func (s SecurityServer) NeedNoAccess(_ *http.Request) bool {
	return true
}

func (s SecurityServer) NeedAdmin(r *http.Request) bool {
	return s.CanAccessAdmin(r)
}

func (s SecurityServer) NeedUser(r *http.Request) bool {
	return s.CanAccessUser(r)
}

func write(data []byte, w http.ResponseWriter) {
	if _, err := w.Write(data); err != nil {
		logger.GetLogger2().Error("Error when write data", err)
	}
}

func header(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-type", "application/json")
}
