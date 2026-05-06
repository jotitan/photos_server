package people_tag

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

type resultsFaceDetection struct {
	Results []resultFace
}

type resultFace struct {
	Id       int    `json:"id"`
	Filename string `json:"image_file"`
}

type FaceDetector struct {
	tagManager           *PeopleTagManager
	faceDetectUrlService string
}

func NewFaceDetector(url, folderTag string) *FaceDetector {
	if url == "" {
		return nil
	}
	return &FaceDetector{
		faceDetectUrlService: url,
		tagManager:           NewPeopleTagManager(folderTag),
	}
}

// Launch launch a request on a distance service
func (fd FaceDetector) Launch(folderId int, pathFolder string) (int, int, error) {
	log.Println(fmt.Sprintf("%s?folder=%s", fd.faceDetectUrlService, pathFolder))
	resp, err := http.Post(fmt.Sprintf("%s?folder=%s", fd.faceDetectUrlService, pathFolder), "application/json", nil)
	if err != nil {
		return 0, 0, errors.New("impossible to detect faces " + err.Error())
	}
	data, _ := io.ReadAll(resp.Body)
	var results resultsFaceDetection
	if err = json.Unmarshal(data, &results); err != nil {
		return 0, 0, err
	}
	// Group by people
	groups := make(map[int][]string)
	for _, r := range results.Results {
		groups[r.Id] = append(groups[r.Id], r.Filename)
	}
	for idTag, paths := range groups {
		fd.tagManager.Tag(folderId, idTag, paths, []string{})
	}
	return len(results.Results), len(groups), fd.tagManager.Flush()
}
