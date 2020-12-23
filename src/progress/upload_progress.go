package progress

import (
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"net/http"
	"sync"
)

/* Manage progresser of a task. Use SSE to notify */

type UploadProgress struct {
	id        string
	chanel    chan struct{}
	total     int
	totalDone int
	// SSE connexion
	sses    []*sse
	waiter  *sync.WaitGroup
	manager *UploadProgressManager
}

func (up *UploadProgress)GetId()string{
	return up.id
}

// Add a waitergroup to manage Done / wait
func (up *UploadProgress) EnableWaiter(){
	up.waiter = &sync.WaitGroup{}
}

func (up *UploadProgress)Add(size int){
	if up.waiter != nil {
		up.waiter.Add(size)
	}
}

func (up *UploadProgress)run(){
	go func(){
		for {
			if _,more := <-up.chanel ; more {
				up.totalDone++
				// Send notif if sse exist
				for _, s := range up.sses {
					s.done(stat{up.totalDone, up.total})
				}
			}else{
				// close chanel, send End message to all
				logger.GetLogger2().Info("Close chanel")
				for _, s := range up.sses {
					s.end()
				}
				break
			}
		}
	}()
}

func (up *UploadProgress) End(){
	close(up.chanel)
	up.manager.remove(up.id)
}

func (up *UploadProgress) Done(){
	if up.waiter != nil {
		up.waiter.Done()
	}
	up.chanel<-struct{}{}
}

func (up *UploadProgress)Wait(){
	if up.waiter != nil {
		up.waiter.Wait()
	}
}

func (up *UploadProgress) Error(e error) {
	// Send message to sse and remove from manager
	for _, s := range up.sses {
		s.error(e)
	}
}

// Manage uploads progression
type UploadProgressManager struct{
	uploads map[string]*UploadProgress
	count int
}

func NewUploadProgressManager()*UploadProgressManager {
	return &UploadProgressManager{make(map[string]*UploadProgress),0}
}

func (upm *UploadProgressManager)getStatUpload(id string)(stat,error){
	if up,ok := upm.uploads[id] ; ok {
		return stat{up.totalDone,up.total},nil
	}
	return stat{},errors.New("unknown upload id")
}

func (upm *UploadProgressManager) AddSSE(id string, w http.ResponseWriter,r * http.Request)(*sse,error){
	if up,ok := upm.uploads[id] ; ok {
		sse := newSse(w,r)
		up.sses = append(up.sses,sse)
		return sse,nil
	}
	return nil,errors.New("unknown upload id " + id + " for SSE (" + fmt.Sprintf("%d",len(upm.uploads)))
}

// return unique id representing upload
func (upm *UploadProgressManager) AddUploader(total int)*UploadProgress {
	id := upm.generateNewID()
	uploader := &UploadProgress{chanel: make(chan struct{},10),total:total*2,id:id,manager:upm}
	uploader.run()
	logger.GetLogger2().Info("Create uploader",id)
	upm.uploads[id] = uploader
	return uploader
}

func (upm *UploadProgressManager)generateNewID()string{
	upm.count++
	h := md5.New()
	h.Write([]byte{byte(upm.count)})
	id := h.Sum([]byte{})
	return base64.StdEncoding.EncodeToString(id)
}

func (upm *UploadProgressManager) remove(id string) {
	delete(upm.uploads,id)
}
