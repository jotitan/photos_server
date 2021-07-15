package main

import (
	"encoding/json"
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"github.com/jotitan/photos_server/resize"
	"io/ioutil"
	"net/http"
)

var agor resize.AsyncGoResizer

func main(){
	// Get path of ffmpeg from parameter
	agor = resize.NewAsyncGoResize()
	s := http.NewServeMux()
	s.HandleFunc("/convert",convertPhoto)
	s.HandleFunc("/status",statusPhoto)
	logger.GetLogger2().Info("Start server on 9013")
	http.ListenAndServe(":9013",s)
}


func statusPhoto(w http.ResponseWriter,_* http.Request){
	w.Write([]byte("up"))
}

type conversionResponse struct{
	err error
	width uint
	height uint
	orientation int
}

func convertPhoto(w http.ResponseWriter,r * http.Request){
	data,_ := ioutil.ReadAll(r.Body)
	cr := resize.ConversionRequest{}
	if err := json.Unmarshal(data,&cr) ; err != nil {
		logger.GetLogger2().Error("Impossible de deserializer request",err)
		http.Error(w, err.Error(),400)
		return
	}
	logger.GetLogger2().Info("Receive request to convert photo with parameters", cr.Input)
	resp := <- doConversion(cr.Input,cr.Orientation,cr.Conversions)
	// When conversion end, return immedialty result, no async
	if resp.err != nil {
		http.Error(w, resp.err.Error(),400)
	}else{
		w.Write([]byte(fmt.Sprintf("{\"width\":%d,\"height\":%d,\"orientation\":%d}", resp.width, resp.height, resp.orientation)))
	}
}



func doConversion(from string,orientation int, conversions[] resize.ImageToResize)chan conversionResponse{
	c := make(chan conversionResponse,1)
	callback := func(err error,width,height uint,correctOrientation int){
		if err == nil {
			if height != 0 && width != 0 {
				c <- conversionResponse{height: height, width: width, orientation: correctOrientation}
			}
		}else{
			c<-conversionResponse{err:err}
		}
	}
	go agor.ResizeAsync(from,orientation,conversions,callback)
	return c
}
