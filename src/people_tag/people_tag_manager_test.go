package people_tag

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAddTag(t *testing.T){
	folder := os.TempDir()
	ptm := NewPeopleTagManager(folder)
	ptm.Tag(1,1,[]string{"path1","path2"})

	if length := len(ptm.Search(1,1)) ; length != 2 {
		t.Error(fmt.Sprintf("Must find 2 but found %d",length))
	}
}

func TestAddTagAndSave(t *testing.T){
	folder := filepath.Join(os.TempDir(),fmt.Sprintf("tag_%d",time.Now().Unix()))
	os.MkdirAll(folder,os.ModePerm)

	ptm := NewPeopleTagManager(folder)
	ptm.Tag(1,1,[]string{"path1","path2"})

	ptm2 := NewPeopleTagManager(folder)
	if length := len(ptm2.Search(1,1)) ; length != 0 {
		t.Error(fmt.Sprintf("Must find 0 but found %d",length))
	}

	ptm.Flush()
	if length := len(ptm2.Search(1,1)) ; length != 2 {
		t.Error(fmt.Sprintf("Must find 2 but found %d",length))
	}

	ptm2 = NewPeopleTagManager(folder)
	ptm2.Tag(1,2,[]string{"new path1","new path2","new path 2"})
	ptm2.Tag(1,1,[]string{"missing one path"})
	ptm2.Tag(2,1,[]string{"single path"})

	ptm2.Flush()

	if length := len(ptm2.Search(2,1)) ; length != 1 {
		t.Error(fmt.Sprintf("Must find 1 but found %d",length))
	}
	if length := len(ptm2.Search(1,2)) ; length != 3 {
		t.Error(fmt.Sprintf("Must find 3 but found %d",length))
	}
	if length := len(ptm2.Search(1,1)) ; length != 3 {
		t.Error(fmt.Sprintf("Must find 3 but found %d",length))
	}

	ptm2 = NewPeopleTagManager(folder)
	if length := len(ptm2.Search(1,1)) ; length != 3 {
		t.Error(fmt.Sprintf("Must find 3 but found %d",length))
	}
}
