package photos_server

import (
	"github.com/jotitan/photos_server/logger"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GarbageManager manage the deletions of image
type GarbageManager struct {
	// Where images are moved
	folder string
	manager *FoldersManager
}

func NewGarbageManager(folder,maskAdmin string,manager *FoldersManager)*GarbageManager {
	// Test if folder exist
	if strings.EqualFold("",maskAdmin) {
		logger.GetLogger2().Error("Impossible to use garbage without a security mask")
		return nil
	}
	if dir,err := os.Open(folder) ; err == nil {
		defer dir.Close()
		if stat,err := dir.Stat() ; err == nil {
			if stat.IsDir() {
				return &GarbageManager{folder:folder,manager:manager}
			}
		}
	}
	logger.GetLogger2().Error("Impossible to create garbage, folder is not available",folder)
	return nil
}

func (g GarbageManager)Remove(files []string)int{
	// For each image to delete, find the good node
	success := 0
	for _,file := range files {
		if node,parent,err := g.manager.FindNode(file) ; err == nil {
			// Remove copy only if move works
			if g.moveOriginalFile(node) {
				if err := g.manager.removeFilesNode(node) ; err == nil {
					// Remove node from structure
					delete(parent, node.Name)
					success++
					logger.GetLogger2().Info("Remove image", node.AbsolutePath)
				}else{
					logger.GetLogger2().Error("Impossible to delete images",err)
				}
			}
		}
	}
	// Save structure
	g.manager.save()
	return success
}

func (g GarbageManager)alreadyMoved(originalPath,movePath string)bool{
	if move,err := os.Open(movePath);err == nil {
		move.Close()
		if f,err := os.Open(originalPath) ; err != nil {
			logger.GetLogger2().Info("Image " + originalPath + " is already in garbage")
			return true
		}else{
			f.Close()
		}
	}
	return false
}

func (g GarbageManager)moveOriginalFile(node *Node)bool{
	return g.MoveOriginalFileFromPath(node.AbsolutePath,node.RelativePath)
}

var replaceSeparator,_ = regexp.Compile("(\\\\)|(/)")
func (g GarbageManager) MoveOriginalFileFromPath(absolutePath,relativePath string)bool{
	moveName := filepath.Join(g.folder,replaceSeparator.ReplaceAllString(relativePath,"."))
	// Check if copy in garbage already exist and source already missing
	if g.alreadyMoved(absolutePath,moveName){
		return true
	}
	if move,err := os.OpenFile(moveName,os.O_TRUNC|os.O_CREATE|os.O_RDWR,os.ModePerm); err == nil {
		if from,err := os.Open(absolutePath) ; err == nil {
			if _,err := io.Copy(move,from) ; err == nil {
				move.Close()
				from.Close()
				logger.GetLogger2().Info("Move",absolutePath,"to garbage",moveName)
				if err := os.Remove(absolutePath); err == nil {
					return true
				}else{
					logger.GetLogger2().Error("Impossible to remove",absolutePath,err )
					return false
				}
			}
		}
	}

	logger.GetLogger2().Error("Impossible to move",absolutePath,"in garbage")
	return false
}