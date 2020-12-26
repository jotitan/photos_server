package video

import (
	"fmt"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type HLSManager interface {
	Convert(path,output string, sizes,bitrates []string)chan bool
}

type HSLLocalManager struct {
	ffmpegPath string
}

func GetHLSManager(conf config.Config)HLSManager {
	switch{
	case !strings.EqualFold("",conf.VideoConfig.FFMPEGPath):return newHSLLocalManager(conf.VideoConfig.FFMPEGPath)
	case !strings.EqualFold("",conf.VideoConfig.ConvertServer):return newHSLRemoteManager(conf.VideoConfig.ConvertServer)
	default: return nil
	}
}

func newHSLLocalManager(ffmpegPath string)HSLLocalManager{
	logger.GetLogger2().Info("Create local video converter")
	return HSLLocalManager{ffmpegPath}
}

func (hsll HSLLocalManager)Convert(path,output string, sizes,bitrates []string)chan bool{
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

	cmd := exec.Command(hsll.ffmpegPath,paramsArray...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	c := make(chan bool,1)
	go func(){
		if err := cmd.Run() ; err != nil {
			c <- false
		}else{
			c <- true
		}
	}()
	return c
}

type HSLRemoteManager struct {
	endpoint string
}

func newHSLRemoteManager(endpoint string)HSLRemoteManager{
	logger.GetLogger2().Info("Create remote video converter",endpoint)
	return HSLRemoteManager{endpoint}
}

func (hsrl HSLRemoteManager)Convert(path,output string, sizes,bitrates []string)chan bool{
	// Call url with parameters and a unique generated id
	url := fmt.Sprintf("%s?sizes=%s&bitrates=%s&path=%s&output=%s",hsrl.endpoint,
		strings.Join(sizes,","),
		strings.Join(bitrates,","),
		path,output)
	c := make(chan bool,1)
	go func() {
		if resp, err := http.DefaultClient.Get(url); err == nil {
			if data,err := ioutil.ReadAll(resp.Body) ; err == nil {
				c<-strings.EqualFold("true",string(data))
			}else {
				logger.GetLogger2().Error("impossible to launch",url,":",err)
				c <- false
			}
		}else{
			logger.GetLogger2().Error("impossible to launch",url,":",err)
			c<-false
		}
	}()
	return c
}