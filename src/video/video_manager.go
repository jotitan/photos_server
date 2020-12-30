package video

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"github.com/jotitan/photos_server/progress"
	"io"
	"io/ioutil"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type VideoFiles map[string]*VideoNode

type Metadata struct {
	Title string
	Date time.Time
	Duration int	// in second
	Peoples []string
	Keywords []string
	Place []string
}

// Store the initial video and compute HLS
type VideoNode struct {
	// No store sizes, use HLS
	Name      string
	CoverPath string
	// Sub Folder with HLS video
	HLSFolder string
	// Path where original file is stored
	OriginalPath string
	// Path of node relative to head
	RelativePath string
	IsFolder     bool
	Metadata     Metadata
	// Store files in a map with name
	Files        VideoFiles
}

type VideoNodeDto struct {
	VideosPath string
	CoverPath string
	DeletePath string
	Metadata Metadata
}

func NewVideoNodeDto(node VideoNode)VideoNodeDto{
	return VideoNodeDto{
		Metadata:node.Metadata,
		CoverPath:"/cover/" + node.RelativePath,
		DeletePath:"/deleteVideo?path=" + node.RelativePath,
		VideosPath:fmt.Sprintf("/video_stream/%s/stream/",node.HLSFolder)}
}

type VideoManager struct {
	exiftool string
	// Folder where original video files are stored
	originalUploadFolder string
	// Folder when hls segment (and cover) are stored
	hlsUploadFolder string
	Folders VideoFiles
	hlsManager HLSManager
}

func NewVideoManager(conf config.Config)*VideoManager{
	if conf.VideoConfig.ExifTool == ""{
		return nil
	}
	return &VideoManager{
		exiftool:conf.VideoConfig.ExifTool,
		hlsUploadFolder:conf.VideoConfig.HLSUploadedFolder,
		originalUploadFolder:conf.VideoConfig.OriginalUploadedFolder,
		Folders:make(map[string]*VideoNode),
		hlsManager:GetHLSManager(conf)}
}


func getSaveVideoPath()string{
	wd,_ := os.Getwd()
	return filepath.Join(wd,"save-videos.json")
}

func (vm * VideoManager)Load()error{
	path := getSaveVideoPath()
	if data,err := ioutil.ReadFile(path) ; err == nil {
		vm.Folders = make(map[string]*VideoNode)
		return json.Unmarshal(data,&vm.Folders)
	}else{
		return err
	}
	return nil
}

func (vm * VideoManager)Save()error{
	path := getSaveVideoPath()
	if data,err := json.Marshal(vm.Folders) ; err == nil {
		if f, err := os.OpenFile(path,os.O_CREATE|os.O_TRUNC|os.O_RDWR,os.ModePerm) ; err == nil {
			defer f.Close()
			if _,err := f.Write(data) ; err != nil{
				return err
			}
		}
	}else{
		return err
	}
	return nil
}

func (vm VideoManager)FindVideoNode(path string)(*VideoNode,map[string]*VideoNode,error){
	current := vm.Folders
	nbSub := strings.Count(path,"/")
	if nbSub == 0{
		if node,ok := vm.Folders[path] ; ok {
			return node,vm.Folders,nil
		}
		return nil,nil,errors.New("Impossible to find node " + path)
	}
	for pos,sub := range strings.Split(path,"/") {
		node := current[sub]
		if node == nil {
			return nil,nil,errors.New("Impossible to find node " + path)
		}
		if node.IsFolder {
			if pos == nbSub {
				return node,current,nil
			}
			current = current[sub].Files
		}else{
			// If not last element
			if pos == nbSub {
				// Last element, return it
				return node,current,nil
			}else{
				// Impossible to continue
				return nil,nil,errors.New("Impossible to found node " + path)
			}
		}
	}
	if current != nil {

	}
	return nil,nil,errors.New("Bad path " + path)
}

// Return the master of HLS video
func (vm * VideoManager)GetVideoMaster(path string)(string,error){
	if node,_,err := vm.FindVideoNode(path) ; err == nil {
		return filepath.Join(vm.hlsUploadFolder,node.HLSFolder,"master.m3u8"),nil
	}else{
		return "",err
	}
}

// Return the master of HLS video
func (vm * VideoManager)GetVideoSegment(path,segment string)(string,error){
	if _,_,err := vm.FindVideoNode(path) ; err == nil {
		return filepath.Join(vm.hlsUploadFolder,path,segment),nil
	}else{
		return "",err
	}
}

func (vm * VideoManager)GetCover(path string)(*os.File,error){
	if node,_,err := vm.FindVideoNode(path) ; err == nil {
		return os.Open(filepath.Join(vm.hlsUploadFolder,node.HLSFolder,node.CoverPath))
	}else{
		return nil,errors.New("unknown path")
	}
}

func (vm * VideoManager)UploadVideoGlobal(folder string,video multipart.File,videoName string,cover multipart.File,coverName string, progressManager * progress.UploadProgressManager)(*progress.UploadProgress,error){
	progresser := progressManager.AddUploader(1)
	go vm.UploadVideo(folder,video,videoName,cover,coverName,progresser)
	return progresser,nil
}

func (vm * VideoManager)Delete(path string,moveFile func(string,string)bool)error{
	if node,parent,err := vm.FindVideoNode(path) ; err == nil {
		logger.GetLogger2().Info("Remove video file",node.Name)
		if err := os.RemoveAll(filepath.Join(vm.hlsUploadFolder,node.HLSFolder)) ; err != nil {
			return err
		}
		// Move original video
		if moveFile(filepath.Join(vm.originalUploadFolder,node.OriginalPath),node.RelativePath + ".mp4") {
			// Move cover
			moveFile(filepath.Join(vm.hlsUploadFolder,node.HLSFolder,node.CoverPath),node.RelativePath + filepath.Ext(node.CoverPath))
			delete(parent, node.Name)
			vm.Save()
		}else{
			return errors.New("impossible to move original file")
		}
		return nil
	}else{
		return errors.New("unknown path")
	}
}

func (vm * VideoManager)UploadVideo(folder string,video multipart.File,videoName string,cover multipart.File,coverName string, progresser * progress.UploadProgress)bool{
	// Create folders (hls folder and original) if necessary
	pathOriginal := filepath.Join(vm.originalUploadFolder,folder)
	cleanName := strings.TrimSuffix(videoName,filepath.Ext(videoName))
	if err := os.MkdirAll(pathOriginal,os.ModePerm) ; err != nil {
		return errorProgresser(progresser,err)
	}
	pathHls := filepath.Join(vm.hlsUploadFolder, folder, cleanName)
	if err := os.MkdirAll(pathHls,os.ModePerm) ; err != nil {
		return errorProgresser(progresser,err)
	}
	node := &VideoNode{HLSFolder:filepath.Join(folder, cleanName),IsFolder:false,OriginalPath:filepath.Join(folder,videoName)}

	// Copy video original
	filename := filepath.Join(vm.originalUploadFolder,folder,videoName)
	if f , err := os.OpenFile(filename,os.O_CREATE|os.O_TRUNC|os.O_RDWR,os.ModePerm) ; err == nil {
		if nb,err := io.Copy(f,video); err == nil && nb > 0{
			f.Close()
			// Extract exif
			properties := vm.getProperties(filename)
			node.Metadata = createMetadatas(properties)
			node.Name = cleanName
			progresser.Done()
		}else{
			return errorProgresser(progresser,err)
		}
	}else{
		return errorProgresser(progresser,err)
	}
	// If pathHls is not empty, not need to compute again
	if !isFolderEmpty(pathHls) {
		logger.GetLogger2().Info("Segments already exists for",pathHls)
		progresser.Done()
	}else {
		// use ffmpeg to create segments
		if !vm.createSegments(filename, pathHls, progresser) {
			logger.GetLogger2().Info("Impossible to create segments")
			progresser.Error(errors.New("impossible to create segments"))
			progresser.End()
			return false
		} else {
			progresser.Done()
		}
	}
	// Create cover
	vm.copyCover(node,cover,coverName)
	vm.addNode(folder,nil,node)
	vm.Save()
	progresser.End()
	return true
}

func isFolderEmpty(path string)bool {
	if f,err := os.Open(path) ; err == nil {
		if files,err := f.Readdirnames(-1) ; err == nil && len(files) > 0 {
			return false
		}
	}
	return true
}

func errorProgresser(progresser * progress.UploadProgress, err error)bool{
	progresser.Error(err)
	progresser.End()
	return false
}


// Command line : ffmpeg.exe -y -i my_video.mp4 -preset slow -g 48 -sc_threshold 0 -map 0:0 -map 0:1 -map 0:0 -map 0:1 -map 0:0 -map 0:1 -s:v:0 640x360 -c:v:0 libx264 -b:v:0 365k -s:v:1 960x540 -c:v:1 libx264 -b:v:1 2000k -s:v:2 1920x1080 -c:v:2 libx264 -b:v:2 6000k -c:a copy -var_stream_map "v:0,a:0 v:1,a:1 v:2,a:2" -master_pl_name master.m3u8 -f hls -hls_time 6 -hls_list_size 0 -hls_segment_filename "path/v%v/fileSequence%d.ts" path/v%v/prog_index.m3u8
func (vm * VideoManager)createSegments(videoPath,hlsFolder string,progresser * progress.UploadProgress)bool{
	// Call distant api or local
	return <- vm.hlsManager.Convert(videoPath,hlsFolder,[]string{"640x360","960x540","1920x1080"},[]string{"365","2000","6000"})
}

func (vm VideoManager)copyCover(node *VideoNode,cover multipart.File,coverName string){
	if cover != nil {
		filename := filepath.Join(vm.hlsUploadFolder, node.HLSFolder, coverName)
		if f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm); err == nil {
			if nb, err := io.Copy(f, cover); err == nil && nb > 0 {
				f.Close()
				node.CoverPath=coverName
			}
		}
	}
}

func (vm * VideoManager)addNode(folder string,parent,node * VideoNode){
	nodesToSearch := vm.Folders
	parentName := ""
	separator := ""
	if parent != nil {
		separator = "/"
		parentName+=parent.RelativePath
		nodesToSearch = parent.Files
	}
	if folder == "" {
		node.RelativePath=parentName+ separator + node.Name
		nodesToSearch[node.Name] = node
	}else{
		// Final folder
		if !strings.Contains(folder,"/") {
			currentParent := createFolderIfNecessary(parentName+separator,folder,nodesToSearch)
			vm.addNode("",currentParent,node)
		}else{
			// Split
			splits := strings.Split(folder,"/")
			currentParent := createFolderIfNecessary(parentName+separator,splits[0],nodesToSearch)
			vm.addNode(strings.Join(splits[1:],"/"),currentParent,node)
		}
	}
}

var splitRegexp,_ = regexp.Compile("(\r\n)|(\n)")

// return exif
func (vm VideoManager)getProperties(path string)map[string]string{
	cmd := exec.Command(vm.exiftool,path)
	data,_:=cmd.Output()
	properties := make(map[string]string)
	for _,line := range splitRegexp.Split(string(data),-1){
		splits := strings.Split(line," :")
		if len(splits) == 2 {
			properties[strings.ReplaceAll(strings.ToLower(strings.Trim(splits[0]," "))," ","_")] = strings.Trim(splits[1]," ")
		}
	}
	logger.GetLogger2().Info("Read properties",len(properties))
	return properties
}

func createFolderIfNecessary(parentName,folder string,nodesToSearch map[string]*VideoNode)*VideoNode{
	currentParent := nodesToSearch[folder]
	// If folder not exist, create it
	if currentParent == nil {
		// Create it
		currentParent = &VideoNode{IsFolder:true,Name:folder,Files:make(map[string]*VideoNode),RelativePath:parentName+folder}
		nodesToSearch[folder] = currentParent
	}
	return currentParent
}

func createMetadatas(properties map[string]string)Metadata{
	metadatas := Metadata{}
	metadatas.Date = formatDate(properties["subtitle"])
	metadatas.Keywords = strings.Split(properties["category"],",")
	metadatas.Peoples = strings.Split(properties["artist"],",")
	metadatas.Place = strings.Split(properties["producer"],",")
	metadatas.Title = properties["title"]
	metadatas.Duration = parseDuration(properties["duration"])

	return metadatas
}

func parseDuration(value string)int{
	if duration := parseDoublePoints(value) ; duration != 0{
		return duration
	}
	r,_ := regexp.Compile("([0-9]+\\.[0-9]+)")
	if duration,err := strconv.ParseFloat(r.FindString(value),32);err == nil {
		return int(duration)
	}
	return 0
}

func parseDoublePoints(value string)int{
	subs := strings.Split(value,":")
	if len(subs) == 0 {
		return 0
	}
	seconds := 0
	for i := 0 ; i < len(subs) ; i++ {
		if n,err := strconv.ParseInt(subs[len(subs) - i - 1],10,32) ; err == nil{
			seconds+=int(n)*getTimeInSec(i)
		}
	}
	return seconds
}

func getTimeInSec(pos int)int{
	switch pos {
	case 1:return 60
	case 2:return 3600
	default: return 1
	}
}

func formatDate(date string)time.Time{
	if t,err := time.Parse("2006-01-02 15:04:05",date) ; err == nil {
		return t
	}
	return time.Now()
}
