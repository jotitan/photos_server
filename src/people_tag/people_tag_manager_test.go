package people_tag

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeleteTag(t *testing.T) {
	folder := filepath.Join(os.TempDir(), fmt.Sprintf("tag_%d_%d", rand.Int(), time.Now().Unix()))
	log.Println(folder)
	os.MkdirAll(folder, os.ModePerm)

	ptm := NewPeopleTagManager(folder)
	id, err := AddPeopleTag(folder, "toto")
	if err != nil {
		t.Error(err.Error())
	}
	ptm.Tag(12, id, []string{"path1", "path2", "path3"}, []string{})
	ptm.Flush()

	if nb := len(ptm.Search(12, id)); nb != 3 {
		t.Error("Must find 3 but find", nb)
	}

	ptm = NewPeopleTagManager(folder)
	ptm.Tag(12, id, []string{}, []string{"path2"})
	ptm.Flush()

	if nb := len(ptm.Search(12, id)); nb != 2 {
		t.Error("Must find 2 but find", nb)
	}

	if nb := len(ptm.SearchAllFolder(id)); nb != 1 {
		t.Error("Must find 1 but find", nb)
	}

	ptm.Tag(12, id, []string{}, []string{"path1", "path3"})
	ptm.Flush()

	if nb := len(ptm.SearchAllFolder(id)); nb != 0 {
		t.Error("Must find 0 but find", nb)
	}

}

func TestAddTagAndSave(t *testing.T) {
	folder := filepath.Join(os.TempDir(), fmt.Sprintf("tag_%d_%d", rand.Int(), time.Now().Unix()))
	log.Println(folder)
	os.MkdirAll(folder, os.ModePerm)

	ptm := NewPeopleTagManager(folder)
	ptm.Tag(1, 1, []string{"path1", "path2"}, []string{})

	ptm2 := NewPeopleTagManager(folder)
	if length := len(ptm2.Search(1, 1)); length != 0 {
		t.Error(fmt.Sprintf("Must find 0 but found %d", length))
	}

	ptm.Flush()
	if length := len(ptm2.Search(1, 1)); length != 2 {
		t.Error(fmt.Sprintf("Must find 2 but found %d", length))
	}

	ptm2 = NewPeopleTagManager(folder)
	ptm2.Tag(1, 2, []string{"new path1", "new path2", "new path 2"}, []string{})
	ptm2.Tag(1, 1, []string{"missing one path"}, []string{})
	ptm2.Tag(2, 1, []string{"single path"}, []string{})

	ptm2.Flush()

	if length := len(ptm2.Search(2, 1)); length != 1 {
		t.Error(fmt.Sprintf("Must find 1 but found %d", length))
	}
	if length := len(ptm2.Search(1, 2)); length != 3 {
		t.Error(fmt.Sprintf("Must find 3 but found %d", length))
	}
	if length := len(ptm2.Search(1, 1)); length != 3 {
		t.Error(fmt.Sprintf("Must find 3 but found %d", length))
	}

	ptm2 = NewPeopleTagManager(folder)
	if length := len(ptm2.Search(1, 1)); length != 3 {
		t.Error(fmt.Sprintf("Must find 3 but found %d", length))
	}

	ptm2.Tag(1, 1, []string{"new1.1 1", "new1.1 2"}, []string{})
	ptm2.Flush()
	ptm2.Tag(1, 1, []string{"new1.1 3", "new1.1 4", "new1.1 5"}, []string{})
	ptm2.Flush()

	if length := len(ptm2.Search(1, 1)); length != 8 {
		t.Error(fmt.Sprintf("Must find 8 but found %d", length))
	}

	ptm2 = NewPeopleTagManager(folder)
	if length := len(ptm2.Search(1, 1)); length != 8 {
		t.Error(fmt.Sprintf("Must find 8 but found %d", length))
	}
}

func TestLocaly(t *testing.T) {
	folder := "your_folder_here"

	ptm2 := NewPeopleTagManager(folder)
	for i := 1; i <= 9; i++ {
		pt1 := ptm2.getPeopleTag(i)
		fmt.Println(pt1.Diag())
	}
}

func TestCompactLocaly(t *testing.T) {
	folder := "c:\\Projets\\PERSO\\DEV\\data"

	ptm := NewPeopleTagManager(folder)
	fmt.Println(ptm.CompactAll())
	ptm = NewPeopleTagManager(folder)
	for i := 1; i < 10; i++ {
		fmt.Println(ptm.getPeopleTag(i).Diag())
	}
}

func TestAddTagAndSaveMulti(t *testing.T) {
	folder := filepath.Join(os.TempDir(), fmt.Sprintf("tag_%d_%d", rand.Int(), time.Now().Unix()))
	log.Println(folder)
	os.MkdirAll(folder, os.ModePerm)

	ptm2 := NewPeopleTagManager(folder)

	ptm2.Tag(1, 1, []string{"new1.1 1", "new1.1 2"}, []string{})
	ptm2.Flush()
	ptm2 = NewPeopleTagManager(folder)
	ptm2.Tag(1, 1, []string{"new1.1 3", "new1.1 4", "new1.1 5"}, []string{})
	ptm2.Flush()

	if length := len(ptm2.Search(1, 1)); length != 5 {
		t.Error(fmt.Sprintf("Must find 5 but found %d", length))
	}

	ptm2 = NewPeopleTagManager(folder)
	if length := len(ptm2.Search(1, 1)); length != 5 {
		t.Error(fmt.Sprintf("Must find 5 but found %d", length))
	}
}

func TestCompact(t *testing.T) {
	folder := filepath.Join(os.TempDir(), fmt.Sprintf("tag_%d_%d", rand.Int(), time.Now().Unix()))
	os.MkdirAll(folder, os.ModePerm)

	AddPeopleTag(folder, "user1")
	AddPeopleTag(folder, "user2")
	AddPeopleTag(folder, "user3")

	ptm := NewPeopleTagManager(folder)

	ptm.Tag(1, 1, []string{"new1.1 1", "new1.1 2"}, []string{})
	ptm.Tag(1, 2, []string{"new2.1 1", "new2.1 2"}, []string{})
	ptm.Flush()
	ptm = NewPeopleTagManager(folder)
	ptm.Tag(1, 1, []string{"new1.1 3", "new1.1 4", "new1.1 5"}, []string{})
	ptm.Tag(1, 3, []string{"new3.1 3"}, []string{})
	ptm.Flush()

	ptm = NewPeopleTagManager(folder)

	ptm.CompactAll()

	ptm = NewPeopleTagManager(folder)
	if paths := ptm.Search(1, 1); len(paths) != 5 {
		t.Error(fmt.Sprintf("Must find 5 but found %d", len(paths)))
	}
	if paths := ptm.Search(1, 3); len(paths) != 1 {
		t.Error(fmt.Sprintf("Must find 1 but found %d", len(paths)))
	}
}
