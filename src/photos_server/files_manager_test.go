package photos_server

import (
	"fmt"
	"github.com/jotitan/photos_server/common"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/progress"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type EmptyReducer struct {
	cache string
}

func (e EmptyReducer) GetCache() string {
	return e.cache
}

func (e EmptyReducer) CheckResizer() bool {
	return true
}

func (e EmptyReducer) CreateJpegFile(folder, basePath string, size uint) string {
	return ""
}

func (e EmptyReducer) GetSizes() []uint {
	return []uint{200, 600}
}

func (e EmptyReducer) AddImage(path, relativePath string, node *Node, progresser *progress.UploadProgress, existings map[string]struct{}, forceRotate bool) {
	input, _ := os.Open(path)
	defer input.Close()
	outputPath := filepath.Join(e.cache, relativePath)
	os.MkdirAll(filepath.Dir(outputPath), os.ModePerm)

	output, _ := os.OpenFile(filepath.Join(e.cache, relativePath), os.O_RDWR|os.O_CREATE, os.ModePerm)
	defer output.Close()
	io.Copy(output, input)
	progresser.Done()
}

func TestFindSub(t *testing.T) {
	node := &Node{Files: map[string]*Node{
		"path1": {
			IsFolder: true,
			Files: map[string]*Node{
				"sub1": {
					IsFolder: true,
					Files: map[string]*Node{
						"subsub1": {},
						"subsub2": {},
						"subsub3": {},
					},
				},
				"sub2": {
					IsFolder: true,
					Files: map[string]*Node{
						"subsub1": {},
						"subsub4": {},
					},
				},
			},
		},
		"path2": {
			IsFolder: true,
			Files: map[string]*Node{
				"sub3": {
					IsFolder: true,
					Files: map[string]*Node{
						"subsub1": {},
						"subsub5": {},
						"subsub6": {},
					},
				},
			},
		},
	}}
	if _, _, err := findNodeFromList(node.Files, "path1/sub1/subsub2"); err != nil {
		t.Error("Should find node", err)
	}
}

func TestCompare(t *testing.T) {
	oldFiles := Files{}
	oldFiles["folder1"] = createFolderNode("/home/folder1")
	oldFiles["folder1-2"] = createFolderNode("/home/folder1/folder2")

	newFiles := Files{}
	newFiles["folder1"] = createFolderNode("/home/folder1")
	newFiles["folder1-2"] = createFolderNode("/home/folder1/folder2")

	nodes, deletions, _ := newFiles.Compare(oldFiles)
	if len(nodes) != 0 || len(deletions) != 0 {
		t.Error("Same structure must return 0 differences but find", len(nodes))
	}

	newFiles["image1"] = createImageNode("/home", "/home/folder1/image1.jpg")
	nodes, deletions, _ = newFiles.Compare(oldFiles)
	if len(nodes) != 1 || len(deletions) != 0 {
		t.Error("New image must be detected but find", len(nodes))
	}

	newFiles["folder1-3"] = createFolderNode("/home/folder1/folder3")
	nodes, deletions, _ = newFiles.Compare(oldFiles)
	if len(nodes) != 1 || len(deletions) != 0 {
		t.Error("New folder must not be return, only new images but find", len(nodes))
	}

	newFiles["image1-3"] = createImageNode("/home", "/home/folder1/folder3/image1-3.jpg")
	nodes, deletions, _ = newFiles.Compare(oldFiles)
	if len(nodes) != 2 || len(deletions) != 0 {
		t.Error("New image in subfolder must be found but find", len(nodes))
	}

	oldFiles["image1-2"] = createImageNode("/home", "/home/folder1/folder2/image1-2.jpg")
	nodes, deletions, _ = newFiles.Compare(oldFiles)
	if len(deletions) != 1 {
		t.Error("Old image must be deleted but find", len(deletions))
	}
}

func TestManager(t *testing.T) {
	fm := createStructure()
	if node, _, err := fm.FindNode("root/folder1/folder2/leaf2.jpg"); err != nil || node == nil {
		t.Error("Impossible to find node")
	}
}

func newImage(folder, path, name, date string) *Node {
	img := NewImage(folder, path, name)
	d, _ := time.Parse("20060102", date)
	img.Date = d
	return img
}

func TestFilePath(t *testing.T) {
	fmt.Println(filepath.Join("C:\\Projets\\PERSO\\DATA\\PHOTOS_SERVER\\", "/SOURCE/PENTECOTE/SAMEDI"))
}

func TestUploadFolder(t *testing.T) {
	// GIVEN
	fm, _, cache := createFakeStructure()
	upload, _ := os.MkdirTemp("", "upload-images")
	fm.reducer = EmptyReducer{cache: cache}
	createOriginalFile(upload, "", "file1.jpg", Files{})
	createOriginalFile(upload, "", "file2.jpg", Files{})
	f1, _ := os.Open(filepath.Join(upload, "file1.jpg"))
	f2, _ := os.Open(filepath.Join(upload, "file1.jpg"))
	files := []multipart.File{f1, f2}

	// WHEN
	_, err := fm.UploadFolder(detailUploadFolder{source: "root", path: "folder1/test-upload-new"}, files, []string{"file1.jpg", "file2.jpg"}, false)
	if err != nil {
		t.Error("Error during upload", err)
	}

	time.Sleep(time.Second)

	// THEN
	cacheDir, _ := os.Open(filepath.Join(cache, "root/folder1/test-upload-new"))
	list, _ := cacheDir.Readdirnames(-1)
	if len(list) != 2 {
		t.Error("Need 2 files in folder")
	}
}

func TestMoveFolder(t *testing.T) {
	fm, folder, cache := createFakeStructure()
	err := fm.MoveFolder("root/folder1", "root/move/folder1")
	if err != nil {
		t.Error("Error during copy", err)
	}
	foundNode, _, _ := fm.FindNode("root/move/folder1")
	if foundNode == nil {
		t.Error("Node can't be null")
	}
	foundNode, _, _ = fm.FindNode("root/folder2")
	if foundNode == nil {
		t.Error("Folder 2 must exists")
	}
	foundNode, _, _ = fm.FindNode("root/folder1")
	if foundNode != nil {
		t.Error("Folder 1 must not exists")
	}

	dfile, _ := os.Open(filepath.Join(folder, "root", "move"))
	res, err := dfile.Readdir(-1)
	if len(res) == 0 || err != nil {
		t.Error("Folder source move can't be empty", err, filepath.Join(folder, "root", "move"))
	}
	dfile, _ = os.Open(filepath.Join(cache, "root", "move"))
	res, err = dfile.Readdir(-1)
	if len(res) == 0 || err != nil {
		t.Error("Folder cache move can't be empty", err)
	}
}

func createFakeStructure() (*FoldersManager, string, string) {
	folder, _ := os.MkdirTemp("", "test")
	cache, _ := os.MkdirTemp("", "cache")

	folder1 := Files{}
	folder2 := Files{}
	createOriginalFile(folder, "root/folder1", "first.txt", folder1)
	createOriginalFile(folder, "root/folder1", "second.txt", folder1)
	createOriginalFile(folder, "root/folder1", "third.txt", folder1)
	createOriginalFile(folder, "root/folder2", "quater.txt", folder2)
	createOriginalFile(folder, "root/folder2", "fifth.txt", folder2)

	createSmallFile(cache, "root/folder1", "first_low.txt")
	createSmallFile(cache, "root/folder1", "first_med.txt")
	createSmallFile(cache, "root/folder1", "second_low.txt")
	createSmallFile(cache, "root/folder1", "second_med.txt")
	createSmallFile(cache, "root/folder1", "third_low.txt")
	createSmallFile(cache, "root/folder1", "third_med.txt")
	createSmallFile(cache, "root/folder2", "quater_low.txt")
	createSmallFile(cache, "root/folder2", "quater_med.txt")
	createSmallFile(cache, "root/folder2", "fifth_low.txt")
	createSmallFile(cache, "root/folder2", "fifth_med.txt")

	f1 := NewFolder(folder, filepath.Join(folder, "root", "folder1"), "folder1", folder1, false)
	f2 := NewFolder(folder, filepath.Join(folder, "root", "folder2"), "folder2", folder2, false)

	root := Files{}
	root["folder1"] = f1
	root["folder2"] = f2

	//r := NewFolder(folder, folder, filepath.Dir(folder), root, false)

	fm := NewFoldersManager(config.Config{Security: config.SecurityConfig{}, CacheFolder: cache}, progress.NewUploadProgressManager())
	fm.tagManger = NewTagManager(fm)
	//fm.Folders["root"] = r
	fm.Sources["root"] = &SourceNode{Folder: filepath.Join(folder, "root"), Files: root}
	return fm, folder, cache
}

func createOriginalFile(folder, sub, name string, files Files) string {
	return createFile(folder, sub, name, files, true)
}

func createSmallFile(folder, sub, name string) string {
	return createFile(folder, sub, name, Files{}, false)
}

func createFile(folder, sub, name string, files Files, addTree bool) string {
	path := filepath.Join(folder, sub, name)
	os.MkdirAll(filepath.Join(folder, sub), os.ModePerm)
	err := os.WriteFile(path, []byte("content : "+path), os.ModePerm)
	if err != nil {
		log.Println("ERR", err)
	}
	if addTree {
		files[name] = newImage(folder, path, name, "20200502")
	}
	return path
}

func createStructure() *FoldersManager {
	fm := NewFoldersManager(config.Config{Security: config.SecurityConfig{}}, nil)
	filesSub2 := Files{}
	filesSub2["leaf1.jpg"] = newImage("/home", "/home/folder1/folder2/leaf1.jpg", "leaf1.jpg", "20200502")
	filesSub2["leaf2.jpg"] = newImage("/home", "/home/folder1/folder2/leaf2.jpg", "leaf2.jpg", "20200502")
	filesSub2["leaf3.jpg"] = newImage("/home", "/home/folder1/folder2/leaf3.jpg", "leaf3.jpg", "20200506")
	filesSub2["leaf4.jpg"] = newImage("/home", "/home/folder1/folder2/leaf4.jpg", "leaf4.jpg", "20200506")
	sub2 := NewFolder("/home", "/home/folder1/folder2", "folder2", filesSub2, false)
	filesSub1 := Files{}
	filesSub1["folder2"] = sub2
	filesSub1["leaf5.jpg"] = newImage("/home", "/home/folder1/leaf5.jpg", "leaf5.jpg", "20200507")
	sub1 := NewFolder("/home", "/home/folder1", "folder1", filesSub1, false)
	filesRoot := Files{}
	filesRoot["folder1"] = sub1
	fm.Sources["root"] = &SourceNode{Folder: "root", Files: filesRoot}
	//fm.Folders["root"] = NewFolder("/home", "/home/folder1", "folder1", filesRoot, false)
	return fm
}

func TestTimer(t *testing.T) {
	ti := time.NewTimer(2000)
	ti.Stop()
	go func() {
		for {
			value := <-ti.C
			log.Println("Receive", value)
		}
	}()

	log.Println("Start wait 5s")
	time.Sleep(time.Second * 5)
	log.Println("End wait")

	ti.Reset(time.Second)
	log.Println("End reset")
	time.Sleep(time.Second * 2)
	ti.Reset(time.Second)
	time.Sleep(time.Second * 4)
	log.Println("End sleep")

}

func Test(t *testing.T) {
	// GIVEN
	fm := FoldersManager{Sources: SourceNodes{
		"SOURCE1": &SourceNode{Name: "SOURCE1", Files: make(map[string]*Node)},
		"SOURCE2": &SourceNode{Name: "SOURCE2", Files: make(map[string]*Node)},
	}}
	//WHEN-THEN
	if n, err := fm.FindOrCreateNode("SOURCE3/TEST"); err == nil || n != nil {
		t.Error("SOURCE3/TEST must fail, no source")
	}

	if n, err := fm.FindOrCreateNode("SOURCE1/TEST1/SUBTEST1/SUBSUBTEST1"); err != nil || n == nil {
		t.Error("SOURCE1/TEST1/SUBTEST1/SUBSUBTEST1 must succeded")
	}
	if _, _, err := fm.FindNode("SOURCE1/TEST1"); err != nil {
		t.Error("SOURCE1/TEST1 must exists")
	}

	if n, err := fm.FindOrCreateNode("SOURCE2/TEST2/SUBTEST2/SUBSUBTEST2"); err != nil || n == nil {
		t.Error("SOURCE2/TEST2/SUBTEST2/SUBSUBTEST2 must succeded")
	}

	if _, _, err := fm.FindNode("SOURCE2/TEST2/SUBTEST2/SUBSUBTEST2"); err != nil {
		t.Error("SOURCE2/TEST2/SUBTEST2/SUBSUBTEST2 must exists")
	}

	if n, err := fm.FindOrCreateNode("SOURCE2/TEST3"); err != nil || n == nil {
		t.Error("SOURCE2/TEST3 must succeded")
	}
	if n, err := fm.FindOrCreateNode("SOURCE2/TEST3/SUBTEST3"); err != nil || n == nil {
		t.Error("SOURCE2/TEST3/SUBTEST3 must succeded")
	}

	if _, _, err := fm.FindNode("SOURCE2/TEST3/SUBTEST3"); err != nil {
		t.Error("SOURCE2/TEST3/SUBTEST3 must exists")
	}
}
func TestGroupByDate(t *testing.T) {
	fm := NewFoldersManager(config.Config{Security: config.SecurityConfig{}}, nil)
	filesRoot := Files{}
	filesRoot["f1"] = &Node{Name: "f1", IsFolder: false, Date: time.Date(2020, 3, 10, 12, 0, 12, 0, time.Local)}
	filesRoot["f2"] = &Node{Name: "f2", IsFolder: false, Date: time.Date(2020, 3, 10, 12, 15, 36, 0, time.Local)}
	filesRoot["f3"] = &Node{Name: "f3", IsFolder: false, Date: time.Date(2020, 3, 7, 12, 0, 12, 0, time.Local)}
	filesRoot["f4"] = &Node{Name: "f4", IsFolder: false, Date: time.Date(2020, 3, 12, 0, 0, 12, 0, time.Local)}
	filesRoot["f5"] = &Node{Name: "f5", IsFolder: false, Date: time.Date(2020, 3, 12, 23, 59, 12, 0, time.Local)}
	filesRoot["f6"] = &Node{Name: "f6", IsFolder: false, Date: time.Date(2020, 4, 12, 23, 59, 12, 0, time.Local)}

	fm.Sources["root"] = &SourceNode{Name: "root", Files: map[string]*Node{"home": NewFolder("/home", "/home/folder1", "folder1", filesRoot, false)}}
	ff := make(map[string]common.INode)
	for key, value := range filesRoot {
		ff[key] = value
	}
	byDate := common.ComputeNodeByDate(ff)
	if len(byDate) != 4 {
		t.Error("Must find 4 group of date")
	}
	if list, exist := byDate[time.Date(2020, 3, 10, 0, 0, 0, 0, time.UTC)]; !exist || len(list) != 2 {
		t.Error("Must find 2 photos for 20200310")
	}
}

func TestFindNode(t *testing.T) {
	fm := createStructure()
	if node, _, err := fm.FindNode("root/folder1/folder2"); node == nil || err != nil {
		t.Error("Must find the node")
	} else {
		if !strings.EqualFold("folder2", node.Name) {
			t.Error("Node must be called folder2 but found", node.Name)
		}
	}
}

func TestTagManager(t *testing.T) {
	testDate := "20200502"
	fm := createStructure()
	tagManager := NewTagManager(fm)

	if tagManager.AddTagByFolder("root/ploup", "vacances", "green") == nil {
		t.Error("Must return an Error")
	}
	if len(tagManager.GetTagsByFolder("root/ploup")) != 0 {
		t.Error("Must return 0 tag")
	}
	if err := tagManager.AddTagByFolder("root/folder1/folder2", "vacances", "green"); err != nil {
		t.Error("Must not return an Error", err)
	}
	if len(tagManager.GetTagsByFolder("root/folder1/folder2")) != 1 {
		t.Error("Must return 1 tag")
	}
	if err := tagManager.AddTagByFolder("root/folder1/folder2", "eliott", "red"); err != nil {
		t.Error("Must not return an Error", err)
	}
	if len(tagManager.GetTagsByFolder("root/folder1/folder2")) != 2 {
		t.Error("Must return 2 tag")
	}
	if len(tagManager.GetTagsByDate(testDate)) != 2 {
		t.Error("Must return 2 tag")
	}
	// CHange color
	if err := tagManager.AddTagByFolder("root/folder1/folder2", "eliott", "green"); err != nil {
		t.Error("Must not return an Error", err)
	}
	if l := len(tagManager.GetTagsByFolder("root/folder1/folder2")); l != 2 {
		t.Error("Must return 2 tag but found", l)
	}
	if list := tagManager.GetTagsByFolder("root/folder1/folder2"); !strings.EqualFold("green", list[0].Color) || !strings.EqualFold("green", list[1].Color) {
		t.Error("Must return color green")
	}
	if l := len(tagManager.GetTagsByDate(testDate)); l != 2 {
		t.Error("Must return 2 tag but fount", l)
	}

}

func TestRemoveNode(t *testing.T) {
	fm := createStructure()
	if node, _, err := fm.FindNode("root/folder1/folder2/leaf1.jpg"); node == nil || err != nil {
		t.Error("Must find the node")
	}
	if fm.RemoveNode("root/folder1/folder2/leaf1.jpg") != nil {
		t.Error("Delete must success")
	}
	if node, _, err := fm.FindNode("root/folder1/folder2/leaf1.jpg"); node != nil || err == nil {
		t.Error("Must not find the node")
	}
}

func createFolderNode(path string) *Node {
	name := filepath.Base(path)
	//dir := filepath.Dir(path)
	return &Node{RelativePath: name, IsFolder: true, Name: name}
}

func createImageNode(rootFolder, path string) *Node {
	name := filepath.Base(path)
	dir := filepath.Dir(path)
	return &Node{RelativePath: strings.ReplaceAll(dir, rootFolder, ""), IsFolder: false, Name: name, Width: int(rand.Int31() % 400), Height: int(rand.Int31() % 200), ImagesResized: true}
}
