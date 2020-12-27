package progress

import (
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"net/http"
	"time"
)

type sse struct {
	chanel chan stat
	w http.ResponseWriter
}

type stat struct {
	done int
	total int
}

func newSse(w http.ResponseWriter, r *http.Request)*sse{
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create chanel to communicate with
	chanelEvent := make(chan stat,10)

	// If connexion stop, close chanel
	s := &sse{chanel:chanelEvent,w:w}
	//watchEndSSE(r,chanelEvent)
	return s
}

func (s * sse)done(st stat){
	s.chanel <- st
}

func (s * sse)end(){
	logger.GetLogger2().Info("Write End")
	writeEnd(s.w)
	close(s.chanel)
}

func (s * sse) Watch(){
	for {
		if st, more  := <- s.chanel ; more {
			writeEvent(s.w,st)
		}else{
			logger.GetLogger2().Error("No more event")
			time.Sleep(time.Second)
			break
		}
	}
}

func (s *sse) error(err error) {
	writeError(s.w,err)
}

func writeEvent(w http.ResponseWriter, st stat){
	w.Write([]byte("event: stat\n"))
	w.Write([]byte(fmt.Sprintf("data: {\"done\":%d,\"total\":%d}\n\n",st.done,st.total)))
	w.(http.Flusher).Flush()
}

func writeEnd(w http.ResponseWriter){
	w.Write([]byte("event: End\n"))
	w.Write([]byte("data: {\"End\":true}\n\n"))
	w.(http.Flusher).Flush()
}

func writeError(w http.ResponseWriter,err error){
	w.Write([]byte("event: erro" +
		"r-message\n"))
	w.Write([]byte(fmt.Sprintf("data: {\"Error\":\"%s\"}\n\n",err.Error())))
	w.(http.Flusher).Flush()
}

func watchEndSSE(r * http.Request, chanelEvent chan stat){
   	go func(){
		<- r.Context().Done()
		logger.GetLogger2().Info("Stop connexion")
		close(chanelEvent)
	}()
}