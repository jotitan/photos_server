package photos_server

import (
	"errors"
	"fmt"
	"strings"
)

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
	tagsByDate map[string][]*Tag
	tagsByFolder map[string][]*Tag
	foldersManager * foldersManager
}

func NewTagManager(foldersManager *foldersManager)*TagManager{
	return &TagManager{tagsByDate:make(map[string][]*Tag),tagsByFolder:make(map[string][]*Tag),foldersManager:foldersManager}
}

// Detect dates of nodes to improve by dates
func (tm * TagManager)AddTagByFolder(path,value,color string)error{
	if folder,_,err := tm.foldersManager.FindNode(path) ; err == nil && folder.IsFolder{
		tm.addTagInMap(tm.tagsByFolder,path,Tag{value,color})
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
	return nil
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
	tm.removeTagInMap(tm.tagsByFolder,path,Tag{value,color})
	if folder,_,err := tm.foldersManager.FindNode(path) ; err == nil && folder.IsFolder{
		dates := tm.findDatesOfNodes(folder)
		for date := range dates {
			tm.RemoveByDate(date,value,color)
		}
	}
}

func (tm * TagManager)RemoveByDate(key string,value,color string){
	tm.removeTagInMap(tm.tagsByDate,key,Tag{value,color})
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

func (tm * TagManager)AddTagByDate(date,value,color string)error{
	tm.addTagInMap(tm.tagsByDate,date,Tag{value,color})
	return nil
}

func (tm * TagManager)GetTagsByFolder(folder string)[]*Tag {
	return tm.getTags(tm.tagsByFolder,folder)
}

func (tm * TagManager)GetTagsByDate(date string)[]*Tag {
	return tm.getTags(tm.tagsByDate,date)
}

func (tm * TagManager)getTags(mapTags map[string][]*Tag,key string)[]*Tag {
	if tags,exist := mapTags[key] ; exist {
		return tags
	}
	return []*Tag{}
}