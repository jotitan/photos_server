package main

import (
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ffmpegPath string
func main(){
	// Get path of ffmpeg from parameter
	if len(os.Args) < 2 {
		panic("Impossible, need to specify mmpeg path in parameter")
	}
	ffmpegPath = os.Args[1]
	s := http.NewServeMux()
	s.HandleFunc("/convert",convert)
	s.HandleFunc("/status",status)
	log.Println("Start server on 9014")
	http.ListenAndServe(":9014",s)
}


func status(w http.ResponseWriter,_* http.Request){
	w.Write([]byte("up"))
}

func convert(w http.ResponseWriter,r * http.Request){
	path := r.FormValue("path")
	output := r.FormValue("output")
	sizes := strings.Split(r.FormValue("sizes"),",")
	bitrates := strings.Split(r.FormValue("bitrates"),",")
	log.Println("Receive request to convert video with parameters",path,output,r.FormValue("sizes"),r.FormValue("bitrates"))
	c := <- convertVideo(ffmpegPath,path,output,sizes,bitrates)
	w.Write([]byte(fmt.Sprintf("%t",c)))
}

func convertVideo(ffmpegPath,path,output string, sizes,bitrates []string)chan bool{
	paramsArray := strings.Split(fmt.Sprintf("-y -i %s -preset slow -g 48 -sc_threshold 0",path)," ")

	strmap := make([]string,len(sizes))
	for i,size := range sizes {
		paramsArray = append(paramsArray,strings.Split(fmt.Sprintf("-s:v:%d %s -c:v:%d libx264 -b:v:%d %sk",i,size,i,i,bitrates[i])," ")...)
		paramsArray = append(paramsArray,strings.Split("-map 0:0 -map 0:1"," ")...)
		strmap[i] = fmt.Sprintf("v:%d,a:%d",i,i)
	}
	paramsArray = append(paramsArray,strings.Split("-c:a copy -var_stream_map"," ")...)
	paramsArray = append(paramsArray,fmt.Sprintf("%s",strings.Join(strmap," ")))
	paths := fmt.Sprintf("-master_pl_name master.m3u8 -f hls -hls_time 6 -hls_list_size 0 -hls_segment_filename %s %s",
		filepath.Join(output,"v%v","fileSequence%d.ts"),
		filepath.Join(output,"v%v","prog_index.m3u8"))
	paramsArray = append(paramsArray,strings.Split(paths," ")...)

	cmd := exec.Command(ffmpegPath,paramsArray...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	c := make(chan bool,1)
	go func(){
		if err := cmd.Run() ; err != nil {
			logger.GetLogger2().Error("Impossible to convert",err)
			logger.GetLogger2().Info("Command was",strings.Join(paramsArray,","))
			c <- false
		}else{
			c <- true
		}
	}()
	return c
}
