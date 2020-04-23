package photos_server

import (
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompare(t *testing.T){
	oldFiles := Files{}
	oldFiles["folder1"] = createFolderNode("/home/folder1")
	oldFiles["folder1-2"] = createFolderNode("/home/folder1/folder2")

	newFiles := Files{}
	newFiles["folder1"] = createFolderNode("/home/folder1")
	newFiles["folder1-2"] = createFolderNode("/home/folder1/folder2")

	nodes,deletions := newFiles.Compare(oldFiles)
	if len(nodes) != 0 || len(deletions) != 0{
		t.Error("Same structure must return 0 differences but find",len(nodes))
	}

	newFiles["image1"] = createImageNode("/home","/home/folder1/image1.jpg")
	nodes,deletions = newFiles.Compare(oldFiles)
	if len(nodes) != 1  || len(deletions) != 0{
		t.Error("New image must be detected but find",len(nodes))
	}

	newFiles["folder1-3"] = createFolderNode("/home/folder1/folder3")
	nodes,deletions = newFiles.Compare(oldFiles)
	if len(nodes) != 1  || len(deletions) != 0{
		t.Error("New folder must not be return, only new images but find",len(nodes))
	}

	newFiles["image1-3"] = createImageNode("/home","/home/folder1/folder3/image1-3.jpg")
	nodes,deletions = newFiles.Compare(oldFiles)
	if len(nodes) != 2  || len(deletions) != 0{
		t.Error("New image in subfolder must be found but find",len(nodes))
	}

	oldFiles["image1-2"] = createImageNode("/home","/home/folder1/folder2/image1-2.jpg")
	nodes,deletions = newFiles.Compare(oldFiles)
	if len(deletions) != 1{
		t.Error("Old image must be deleted but find",len(deletions))
	}
}

func TestManager(t *testing.T){
	fm := NewFoldersManager("//","","")
	leaf1 := NewImage("/home","/home/folder1/folder2/leaf1.jpg","leaf1.jpg")
	leaf2 := NewImage("/home","/home/folder1/folder2/leaf2.jpg","leaf2.jpg")
	filesSub2 := Files{}
	filesSub2["leaf1.jpg"] = leaf1
	filesSub2["leaf2.jpg"] = leaf2
	sub2 := NewFolder("/home","/home/folder1/folder2","folder2",filesSub2,false)
	filesSub1 := Files{}
	filesSub1["folder2"] = sub2
	sub1 := NewFolder("/home","/home/folder1","folder1",filesSub1,false)
	filesRoot := Files{}
	filesRoot["folder1"] = sub1
	fm.Folders["root"] = NewFolder("/home","/home/folder1","folder1",filesRoot,false)
	if node,_,err := fm.FindNode("root/folder1/folder2/leaf2.jpg") ; err != nil || node == nil {
		t.Error("Imposible to find node")
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
