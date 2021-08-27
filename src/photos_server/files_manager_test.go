package photos_server

import (
	"github.com/jotitan/photos_server/common"
	"github.com/jotitan/photos_server/config"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompare(t *testing.T){
	oldFiles := Files{}
	oldFiles["folder1"] = createFolderNode("/home/folder1")
	oldFiles["folder1-2"] = createFolderNode("/home/folder1/folder2")

	newFiles := Files{}
	newFiles["folder1"] = createFolderNode("/home/folder1")
	newFiles["folder1-2"] = createFolderNode("/home/folder1/folder2")

	nodes,deletions,_ := newFiles.Compare(oldFiles)
	if len(nodes) != 0 || len(deletions) != 0{
		t.Error("Same structure must return 0 differences but find",len(nodes))
	}

	newFiles["image1"] = createImageNode("/home","/home/folder1/image1.jpg")
	nodes,deletions,_ = newFiles.Compare(oldFiles)
	if len(nodes) != 1  || len(deletions) != 0{
		t.Error("New image must be detected but find",len(nodes))
	}

	newFiles["folder1-3"] = createFolderNode("/home/folder1/folder3")
	nodes,deletions,_ = newFiles.Compare(oldFiles)
	if len(nodes) != 1  || len(deletions) != 0{
		t.Error("New folder must not be return, only new images but find",len(nodes))
	}

	newFiles["image1-3"] = createImageNode("/home","/home/folder1/folder3/image1-3.jpg")
	nodes,deletions,_ = newFiles.Compare(oldFiles)
	if len(nodes) != 2  || len(deletions) != 0{
		t.Error("New image in subfolder must be found but find",len(nodes))
	}

	oldFiles["image1-2"] = createImageNode("/home","/home/folder1/folder2/image1-2.jpg")
	nodes,deletions,_ = newFiles.Compare(oldFiles)
	if len(deletions) != 1{
		t.Error("Old image must be deleted but find",len(deletions))
	}
}

func TestManager(t *testing.T){
	fm := createStructure()
	if node,_,err := fm.FindNode("root/folder1/folder2/leaf2.jpg") ; err != nil || node == nil {
		t.Error("Impossible to find node")
	}
}

func newImage(folder,path,name,date string)*Node{
	img := NewImage(folder,path,name)
	d,_ :=time.Parse("20060102",date)
	img.Date = d
	return img
}

func createStructure()*FoldersManager {
	fm := NewFoldersManager(config.Config{Security:config.SecurityConfig{}},nil)
	filesSub2 := Files{}
	filesSub2["leaf1.jpg"] = newImage("/home","/home/folder1/folder2/leaf1.jpg","leaf1.jpg","20200502")
	filesSub2["leaf2.jpg"] = newImage("/home","/home/folder1/folder2/leaf2.jpg","leaf2.jpg","20200502")
	filesSub2["leaf3.jpg"] = newImage("/home","/home/folder1/folder2/leaf3.jpg","leaf3.jpg","20200506")
	filesSub2["leaf4.jpg"] = newImage("/home","/home/folder1/folder2/leaf4.jpg","leaf4.jpg","20200506")
	sub2 := NewFolder("/home","/home/folder1/folder2","folder2",filesSub2,false)
	filesSub1 := Files{}
	filesSub1["folder2"] = sub2
	filesSub1["leaf5.jpg"] = newImage("/home","/home/folder1/leaf5.jpg","leaf5.jpg","20200507")
	sub1 := NewFolder("/home","/home/folder1","folder1",filesSub1,false)
	filesRoot := Files{}
	filesRoot["folder1"] = sub1
	fm.Folders["root"] = NewFolder("/home","/home/folder1","folder1",filesRoot,false)
	return fm
}

func TestGroupByDate(t *testing.T){
	fm := NewFoldersManager(config.Config{Security:config.SecurityConfig{}},nil)
	filesRoot := Files{}
	filesRoot["f1"] = &Node{Name:"f1",IsFolder:false,Date:time.Date(2020,3,10,12,0,12,0,time.Local)}
	filesRoot["f2"] = &Node{Name:"f2",IsFolder:false,Date:time.Date(2020,3,10,12,15,36,0,time.Local)}
	filesRoot["f3"] = &Node{Name:"f3",IsFolder:false,Date:time.Date(2020,3,7,12,0,12,0,time.Local)}
	filesRoot["f4"] = &Node{Name:"f4",IsFolder:false,Date:time.Date(2020,3,12,0,0,12,0,time.Local)}
	filesRoot["f5"] = &Node{Name:"f5",IsFolder:false,Date:time.Date(2020,3,12,23,59,12,0,time.Local)}
	filesRoot["f6"] = &Node{Name:"f6",IsFolder:false,Date:time.Date(2020,4,12,23,59,12,0,time.Local)}

	fm.Folders["root"] = NewFolder("/home","/home/folder1","folder1",filesRoot,false)
	ff := make(map[string]common.INode)
	for key,value := range fm.Folders {
		ff[key] = value
	}
	byDate := common.ComputeNodeByDate(ff)
	if len(byDate) != 4 {
		t.Error("Must find 4 group of date")
	}
	if list,exist := byDate[time.Date(2020,3,10,0,0,0,0,time.UTC)] ; !exist || len(list) != 2 {
		t.Error("Must find 2 photos for 20200310")
	}
}

func TestFindNode(t *testing.T) {
	fm := createStructure()
	if node, _, err := fm.FindNode("root/folder1/folder2"); node == nil || err != nil {
		t.Error("Must find the node")
	}else{
		if !strings.EqualFold("folder2",node.Name) {
			t.Error("Node must be called folder2 but found",node.Name)
		}
	}
}

func TestTagManager(t *testing.T){
	testDate := "20200502"
	fm := createStructure()
	tagManager := NewTagManager(fm)

	if tagManager.AddTagByFolder("root/ploup","vacances","green") == nil {
		t.Error("Must return an Error")
	}
	if len(tagManager.GetTagsByFolder("root/ploup")) != 0 {
		t.Error("Must return 0 tag")
	}
	if err := tagManager.AddTagByFolder("root/folder1/folder2","vacances","green") ;err!= nil {
		t.Error("Must not return an Error",err)
	}
	if len(tagManager.GetTagsByFolder("root/folder1/folder2")) != 1 {
		t.Error("Must return 1 tag")
	}
	if err := tagManager.AddTagByFolder("root/folder1/folder2","eliott","red") ;err!= nil {
		t.Error("Must not return an Error",err)
	}
	if len(tagManager.GetTagsByFolder("root/folder1/folder2")) != 2 {
		t.Error("Must return 2 tag")
	}
	if len(tagManager.GetTagsByDate(testDate)) != 2 {
		t.Error("Must return 2 tag")
	}
	// CHange color
	if err := tagManager.AddTagByFolder("root/folder1/folder2","eliott","green") ;err!= nil {
		t.Error("Must not return an Error",err)
	}
	if l := len(tagManager.GetTagsByFolder("root/folder1/folder2")) ; l != 2 {
		t.Error("Must return 2 tag but found",l)
	}
	if list := tagManager.GetTagsByFolder("root/folder1/folder2") ; !strings.EqualFold("green",list[0].Color) || !strings.EqualFold("green",list[1].Color){
		t.Error("Must return color green")
	}
	if l := len(tagManager.GetTagsByDate(testDate)) ; l!= 2 {
		t.Error("Must return 2 tag but fount",l)
	}

}

func TestRemoveNode(t *testing.T){
	fm := createStructure()
	if node,_,err :=  fm.FindNode("root/folder1/folder2/leaf1.jpg") ; node == nil || err != nil {
		t.Error("Must find the node")
	}
	if fm.RemoveNode("root/folder1/folder2/leaf1.jpg") != nil {
		t.Error("Delete must success")
	}
	if node,_,err :=  fm.FindNode("root/folder1/folder2/leaf1.jpg") ; node != nil || err == nil {
		t.Error("Must not find the node")
	}
}

func createFolderNode(path string)*Node {
	name := filepath.Base(path)
	dir := filepath.Dir(path)
	return &Node{AbsolutePath:dir,RelativePath:name,IsFolder:true,Name:name}
}

func createImageNode(rootFolder,path string)*Node {
	name := filepath.Base(path)
	dir := filepath.Dir(path)
	return &Node{AbsolutePath:dir,RelativePath:strings.ReplaceAll(dir,rootFolder,""),IsFolder:false,Name:name,Width:int(rand.Int31()%400),Height:int(rand.Int31()%200),ImagesResized:true}
}
