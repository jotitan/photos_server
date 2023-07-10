package resize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"io"
	"net/http"
)

type GoResizerManager interface {
	ResizeAsync(from string, orientation int, conversions []ImageToResize, callback func(err error, w uint, h uint, o int))
}

type ConversionRequest struct {
	Input       string          `json:"input"`
	Orientation int             `json:"orientation"`
	Conversions []ImageToResize `json:"conversions"`
}

type conversionResponse struct {
	Height      uint `json:"height"`
	Width       uint `json:"width"`
	Orientation int  `json:"orientation"`
}

type HttpGoResizer struct {
	url string
}

func NewHttpGoResizer(url string) HttpGoResizer {
	logger.GetLogger2().Info("Use http go resizer with", url)
	return HttpGoResizer{url: url}
}

func (hgr HttpGoResizer) ResizeAsync(from string, orientation int, conversions []ImageToResize, callback func(err error, w uint, h uint, o int)) {
	request := ConversionRequest{Input: from, Orientation: orientation, Conversions: conversions}
	if data, err := json.Marshal(request); err != nil {
		logger.GetLogger2().Error("Impossible to launch remote conversion", err)
	} else {
		req, err := http.Post(fmt.Sprintf("%s/convert", hgr.url), "application/json", bytes.NewBuffer(data))
		if err != nil || req.StatusCode != 200 {
			logger.GetLogger2().Error("Impossible to execute remote", err, req.StatusCode)
			callback(err, 0, 0, 0)
			return
		}
		data, _ := io.ReadAll(req.Body)
		response := conversionResponse{}
		if err := json.Unmarshal(data, &response); err == nil {
			callback(nil, response.Width, response.Height, response.Orientation)
		}
	}

}
