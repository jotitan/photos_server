package people_tag

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	faceDetectUrlService string
}

func NewFaceDetector(url string) *FaceDetector {
	if url == "" {
		return nil
	}
	return &FaceDetector{
		faceDetectUrlService: url,
	}
}

// Launch launch a request on a distance service
func (fd FaceDetector) Launch(pathFolder string) (map[int][]string, error) {
	resp, err := http.Post(fmt.Sprintf("%s?folder=%s", fd.faceDetectUrlService, pathFolder), "application/json", nil)
	if resp.StatusCode != 200 {
		return nil, errors.New("impossible to contact server")
	}
	if err != nil {
		return nil, errors.New("impossible to detect faces " + err.Error())
	}
	data, _ := io.ReadAll(resp.Body)
	var results resultsFaceDetection
	if err = json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	// Group by people
	groups := make(map[int][]string)
	for _, r := range results.Results {
		groups[r.Id] = append(groups[r.Id], r.Filename)
	}
	return groups, nil
}
