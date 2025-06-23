package video

import (
	"github.com/jotitan/photos_server/progress"
	"os"
	"testing"
)

func TestUpload(t *testing.T) {
	vm := &VideoManager{exiftool: "TODO_PATH", hlsUploadFolder: "hls_folder", Folders: make(map[string]*VideoNode)}
	filename := "video.mp4"
	f, _ := os.Open(filename)
	cover, _ := os.Open("cover.jpg")
	upm := progress.NewUploadProgressManager()
	up := upm.AddUploader(1)
	vm.UploadVideo("test-up/toto", f, "test2.mp4", cover, "cover_image.jpg", up)

	f.Close()
	cover.Close()
}
