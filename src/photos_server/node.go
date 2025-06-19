package photos_server

import (
	"errors"
	"fmt"
	"github.com/jotitan/photos_server/common"
	"path/filepath"
	"strings"
	"time"
)

// A root source
type SourceNode struct {
	Folder string `json:"folder"`
	Name   string `json:"name"`
	Files  Files  `json:"Files,omitempty"`
}

func (s SourceNode) GetSourceFolder() string {
	return filepath.Dir(s.Folder)
}

type SourceNodes map[string]*SourceNode

func (sn SourceNodes) countFolders() int {
	count := 0
	for _, src := range sn {
		count += len(src.Files)
	}
	return count
}

func (sn SourceNodes) getSourceFolder(path string) string {
	if source, _, err := sn.getSourceFromPath(path); err == nil {
		return source.GetSourceFolder()
	}
	return ""
}

func (sn SourceNodes) getSource(source string) (*SourceNode, error) {
	// Source is the first name of path
	if node, exist := sn[source]; exist {
		return node, nil
	}
	return nil, errors.New("Source not found")
}

func (sn SourceNodes) getSourceFromPath(path string) (*SourceNode, string, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	if path[0] == '/' {
		path = path[1:]
	}
	// Source is the first name of path
	if idx := strings.Index(path, "/"); idx != -1 {
		if node, exist := sn[path[:idx]]; exist {
			return node, path[idx+1:], nil
		}
	} else {
		if node, exist := sn[path]; exist {
			return node, "", nil
		}
	}
	return nil, "", errors.New("Source not found")
}

func (sn SourceNodes) getSources() []string {
	sources := make([]string, 0, len(sn))
	for src := range sn {
		sources = append(sources, src)
	}
	return sources
}

type Node struct {
	//AbsolutePath string
	// Path of node relative to head
	RelativePath string
	Width        int
	Height       int
	Date         time.Time `json:"Date,omitempty"`
	Name         string
	IsFolder     bool
	// Store files in a map with name
	Files         Files `json:"Files,omitempty"`
	ImagesResized bool
	Id            int `json:"id,omitempty"`
	// Only if node is a folder
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

func (n Node) GetAbsolutePath(sn SourceNodes) string {
	source, path, err := sn.getSourceFromPath(n.RelativePath)
	if err == nil {
		return filepath.Join(source.Folder, path)
	}
	return ""
}

func (n Node) GetDate() time.Time {
	return n.Date
}

func (n Node) GetIsFolder() bool {
	return n.IsFolder
}

func (n Node) GetFiles() map[string]common.INode {
	nodes := make(map[string]common.INode, len(n.Files))
	for key, value := range n.Files {
		nodes[key] = value
	}
	return nodes
}

func (n Node) applyOnEach(sources SourceNodes, fct func(path, relativePath string, node *Node)) {
	for _, file := range n.Files {
		if file.IsFolder {
			file.applyOnEach(sources, fct)
		} else {
			fct(file.GetAbsolutePath(sources), file.RelativePath, file)
		}
	}
}

func (n Node) String() string {
	return fmt.Sprintf("%s : %t : %s : %s" /* n.AbsolutePath,*/, n.RelativePath, n.ImagesResized, n.Name, n.Files)
}

func NewImage(rootFolder, path, name string) *Node {
	relativePath := strings.ReplaceAll(strings.ReplaceAll(path, strings.ReplaceAll(rootFolder, "\\\\", "\\"), ""), "\\", "/")
	return &Node{ /*AbsolutePath: path,*/ RelativePath: relativePath, Name: name, IsFolder: false, Files: nil, ImagesResized: false}
}

func NewFolder(rootFolder, path, name string, files Files, imageResized bool) *Node {
	relativePath := strings.ReplaceAll(strings.ReplaceAll(path, strings.ReplaceAll(rootFolder, "\\\\", "\\"), ""), "\\", "/")
	return &Node{ /*AbsolutePath: path,*/ RelativePath: relativePath, Name: name, IsFolder: true, Files: files, ImagesResized: imageResized}
}

func NewFolderWithRel(path, relativePath, name string, files Files, imageResized bool) *Node {
	relativePath = strings.ReplaceAll(relativePath, "\\", "/")
	return &Node{ /*AbsolutePath: path, */ RelativePath: relativePath, Name: name, IsFolder: true, Files: files, ImagesResized: imageResized}
}
