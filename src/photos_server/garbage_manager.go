package photos_server

import (
	"io"
	"logger"
	"os"
	"path/filepath"
	"strings"
)

// GarbageManager manage the deletions of image
type GarbageManager struct {
	// Where images are moved
	folder string
	manager *foldersManager
}

func NewGarbageManager(folder,maskAdmin string,manager *foldersManager)*GarbageManager {
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

func (g GarbageManager)moveOriginalFile(node *Node)bool{

	moveName := filepath.Join(g.folder,strings.Replace(node.RelativePath[1:],string(filepath.Separator),".",-1))
	if move,err := os.OpenFile(moveName,os.O_TRUNC|os.O_CREATE|os.O_RDWR,os.ModePerm); err == nil {
		defer move.Close()
		if from,err := os.Open(node.AbsolutePath) ; err == nil {
			if _,err := io.Copy(move,from) ; err == nil {
				move.Close()
				from.Close()
				logger.GetLogger2().Info("Move",node.AbsolutePath,"to garbage",moveName)
				if err := os.Remove(node.AbsolutePath); err == nil {
					return true
				}else{
					logger.GetLogger2().Error("Impossible to remove",node.AbsolutePath)
					return false
				}
			}
		}
	}
	logger.GetLogger2().Error("Impossible to move",node.AbsolutePath,"in garbage")
	return false
}