package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jotitan/photos_server/video"
)

var ffmpegPath string

func main() {
	// Get path of ffmpeg from parameter
	if len(os.Args) < 2 {
		panic("Impossible, need to specify mmpeg path in parameter")
	}
	ffmpegPath = os.Args[1]
	s := http.NewServeMux()
	s.HandleFunc("/convert", convert)
	s.HandleFunc("/status", status)
	log.Println("Start server on 9014")
	http.ListenAndServe(":9014", s)
}

func status(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("up"))
}

func convert(w http.ResponseWriter, r *http.Request) {
	path := r.FormValue("path")
	output := r.FormValue("output")
	sizes := strings.Split(r.FormValue("sizes"), ",")
	bitrates := strings.Split(r.FormValue("bitrates"), ",")
	log.Println("Receive request to convert video with parameters", path, output, r.FormValue("sizes"), r.FormValue("bitrates"))
	c := <-video.NewHSLLocalManager(ffmpegPath).Convert(path, output, sizes, bitrates)
	w.Write([]byte(fmt.Sprintf("%t", c)))
}
