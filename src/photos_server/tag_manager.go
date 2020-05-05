package photos_server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

const bufferSize = 100

type Tag struct {
	Value string
	Color string
}

func (t Tag)String()string{
	return fmt.Sprintf("Tag : %s-%s",t.Value,t.Color)
}

func (t Tag)Equals(tag Tag)bool {
	return strings.EqualFold(t.Value,tag.Value) && strings.EqualFold(t.Color,tag.Color)
}

type TagManager struct {
	TagsByDate     map[string][]*Tag
	TagsByFolder   map[string][]*Tag
	foldersManager * foldersManager
	counter        int32
	// Used to synchronize write
	locker sync.Mutex
}

func NewTagManager(foldersManager *foldersManager)*TagManager{
	tm := &TagManager{TagsByDate: make(map[string][]*Tag), TagsByFolder:make(map[string][]*Tag),foldersManager:foldersManager,counter:0,locker:sync.Mutex{}}
	tm.load()
	return tm
}

// Detect dates of nodes to improve by dates
func (tm * TagManager)AddTagByFolder(path,value,color string)error{
	if folder,_,err := tm.foldersManager.FindNode(path) ; err == nil && folder.IsFolder{
		tm.addTagInMap(tm.TagsByFolder,path,Tag{value,color})
		dates := tm.findDatesOfNodes(folder)
		for date := range dates {
			tm.AddTagByDate(date,value,color)
		}
	}else{
		if err == nil {
			return errors.New("not a folder")
		}
		return err
	}
	tm.countOperation()
	return nil
}

func (tm * TagManager)AddTagByDate(date,value,color string)error{
	tm.addTagInMap(tm.TagsByDate,date,Tag{value,color})
	tm.countOperation()
	return nil
}

// Count number of operation on tag manager and flush data if necessary
func (tm *TagManager)countOperation(){
	atomic.AddInt32(&tm.counter,1)
	if tm.counter >= bufferSize {
		tm.flush()
		atomic.StoreInt32(&tm.counter,0)
	}
}

func (tm * TagManager)load(){
	if data,err := ioutil.ReadFile("tag_database.json") ; err == nil {
		tempTM := TagManager{}
		if json.Unmarshal(data,&tempTM) == nil {
			tm.TagsByFolder = tempTM.TagsByFolder
			tm.TagsByDate = tempTM.TagsByDate
			logger.GetLogger2().Info("Tag database well imported",len(tm.TagsByFolder),len(tm.TagsByDate))
		}
	}else{
		logger.GetLogger2().Info("Impossible to import tag database, does not exist")
	}
}

func (tm *TagManager)flush(){
	tm.locker.Lock()
	defer tm.locker.Unlock()

	if file,err := os.OpenFile("tag_database.json",os.O_RDWR|os.O_CREATE|os.O_TRUNC,os.ModePerm) ; err == nil {
		defer file.Close()
		if data,err := json.Marshal(tm) ; err == nil {
			if _,err := file.Write(data) ; err== nil {
				logger.GetLogger2().Info("Save in file tag_database well done")
			}else{
				logger.GetLogger2().Error("Impossible to save tag database",err)
			}
		}
	}
}

func (tm * TagManager)findDatesOfNodes(node *Node)map[string]struct{}{
	dates := make(map[string]struct{})
	for _,file := range node.Files {
		if !file.IsFolder {
			dates[file.Date.Format("20060102")] = struct{}{}
		}
	}
	return dates
}

func (tm * TagManager)searchTagByName(tags []*Tag,value string)*Tag {
	for _,tag := range tags {
		if strings.EqualFold(value,tag.Value) {
			return tag
		}
	}
	return nil
}

func (tm * TagManager)RemoveByFolder(path string,value,color string){
	tm.removeTagInMap(tm.TagsByFolder,path,Tag{value,color})
	if folder,_,err := tm.foldersManager.FindNode(path) ; err == nil && folder.IsFolder{
		dates := tm.findDatesOfNodes(folder)
		for date := range dates {
			tm.RemoveByDate(date,value,color)
		}
	}
	tm.countOperation()
}

func (tm * TagManager)RemoveByDate(key string,value,color string){
	tm.removeTagInMap(tm.TagsByDate,key,Tag{value,color})
	tm.countOperation()
}

func (tm * TagManager)removeTagInMap(mapTag map[string][]*Tag,key string,tagToRemove Tag){
	if tags,exist := mapTag[key] ; exist {
		for pos,tag := range tags {
			if tag.Equals(tagToRemove) {
				tags[pos] = tags[len(tags)-1]
				mapTag[key] = tags[:len(tags)-1]
				return
			}
		}
	}
}

func (tm * TagManager)addTagInMap(mapTag map[string][]*Tag,key string,tag Tag){
	if tags,exist := mapTag[key] ; exist {
		// Check if tag already exist, if true, update color
		if foundTag := tm.searchTagByName(tags,tag.Value) ; foundTag != nil {
			foundTag.Color = tag.Color
		}else {
			mapTag[key] = append(tags, &tag)
		}
	}else{
		mapTag[key] = []*Tag{&tag}
	}
}


func (tm * TagManager)GetTagsByFolder(folder string)[]*Tag {
	return tm.getTags(tm.TagsByFolder,folder)
}

func (tm * TagManager)GetTagsByDate(date string)[]*Tag {
	return tm.getTags(tm.TagsByDate,date)
}

func (tm * TagManager)getTags(mapTags map[string][]*Tag,key string)[]*Tag {
	if tags,exist := mapTags[key] ; exist {
		return tags
	}
	return []*Tag{}
}