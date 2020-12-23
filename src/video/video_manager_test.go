package video

import (
	"fmt"
	"os"
	"regexp"
	"testing"
)

func TestMap(t *testing.T){
	splitStream,_ := regexp.Compile("(/stream/?)")
	split := splitStream.Split("/video_stream/path1/path2/stream/v0/t21.ts", -1)
	fmt.Println(split)
}



func TestUpload(t *testing.T){
	vm := &VideoManager{exiftool:"C:\\Users\\jonathan.baranzini\\Downloads\\exiftool-12.12\\exiftool.exe",hlsUploadFolder:"C:\\Projets\\DATA\\upload-videos",Folders:make(map[string]*VideoNode)}
	filename := "C:\\Perso\\20200804\\test2.mp4"
	filename2 := "C:\\Perso\\20200804\\low_def.mp4"
	f,_ := os.Open(filename)
	f2,_ := os.Open(filename2)
	cover,_ := os.Open("C:\\Perso\\20200919\\IMG_7065.JPG")
	upm := newUploadProgressManager()
	up := upm.addUploader(1)
	vm.UploadVideo("test-up/toto",f,"test2.mp4",cover,"cover_image.jpg",up)

	f.Close()
	f2.Close()
	cover.Close()
}
