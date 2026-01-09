package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"gioui.org/app"
	"gioui.org/op"
)

type RemoteConfig struct {
	token     string
	serverUrl string
	localUrl  string
	port      string
	name      string
	height    float64
	width     float64
}

func CreateConfig(args []string) RemoteConfig {
	return RemoteConfig{
		token:     args[1],
		serverUrl: args[2],
		localUrl:  args[3],
		port:      args[4],
		name:      args[5],
		height:    float64(getInt(args[6])),
		width:     float64(getInt(args[7])),
	}
}

func getInt(value string) int {
	if value, err := strconv.Atoi(value); err == nil {
		return value
	}
	return 0
}

type GioPhotosApp struct {
	token    string
	baseUrl  string
	localUrl string
	port     string
	name     string
	height   float64
	width    float64
}

func Run(conf RemoteConfig) {
	photoApp := GioPhotosApp{
		token:    conf.token,
		baseUrl:  conf.serverUrl,
		localUrl: conf.localUrl,
		port:     conf.port,
		name:     conf.name,
		height:   conf.height,
		width:    conf.width,
	}
	go func() {
		window := new(app.Window)
		err := photoApp.run(window)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func (g GioPhotosApp) connectAndRunHeartbeat() error {
	_, err := g.doRequest(fmt.Sprintf("%s/remote/connect?name=%s&browser=false&url=%s", g.baseUrl, g.name, g.localUrl))
	if err != nil {
		return err
	}
	go g.runHeartbeat()
	return nil
}

func (g GioPhotosApp) runHeartbeat() {
	// Launch heartbeat every minute
	t := time.NewTicker(time.Minute)
	for {
		<-t.C
		g.doRequest(fmt.Sprintf("%s/remote/heartbeat?name=%s", g.baseUrl, g.name))
	}
}

func (g GioPhotosApp) createServer() {
	s := http.ServeMux{}
	s.HandleFunc("/event", g.switchEvent)
	log.Println("Listen on port " + g.port)
	http.ListenAndServe(":"+g.port, &s)
}

func (g GioPhotosApp) switchEvent(w http.ResponseWriter, r *http.Request) {
	switch r.FormValue("event") {
	case "previous":
		g.previous()
	case "next":
		g.next()
	case "show":
		pos, _ := strconv.Atoi(r.FormValue("pos"))
		g.selectImage(pos)
	case "folder":
		g.loadImages(r.FormValue("url"))
	case "status":
		g.getStatus(w)
	default:
	}
}

func (g GioPhotosApp) getStatus(w http.ResponseWriter) {
	data := fmt.Sprintf("{\"Source\":\"%s\",\"Current\":%d,\"Size\":%d}", currentFolder, currentImage, len(images))
	w.Write([]byte(data))
}

func (g GioPhotosApp) run(window *app.Window) error {
	go g.createServer()
	if err := g.connectAndRunHeartbeat(); err != nil {
		log.Fatal(err)
	}
	window.Option(app.Title("Photos viewer"))
	window.Option(app.Fullscreen.Option())
	var ops op.Ops
	for {
		switch e := window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			k := checkEvent(gtx)
			switch k {
			case Previous:
				g.previous()
			case Next:
				g.next()
			}
			paint.Fill(&ops, color.NRGBA{R: 0, G: 0, B: 0, A: 0xff})
			g.showImage(&ops)
			e.Frame(gtx.Ops)
		}
	}
}

func (g GioPhotosApp) selectImage(pos int) {
	if pos < 0 || len(images) == 0 || pos >= len(images)-1 {
		return
	}
	currentImage = pos
	g.loadImage()
}

func (g GioPhotosApp) previous() {
	g.selectImage(currentImage - 1)
}

func (g GioPhotosApp) next() {
	g.selectImage(currentImage + 1)
}

func (g GioPhotosApp) loadImage() {
	pathImage = images[currentImage]
	reload = true
	log.Println("Log image", pathImage)
}

func (g GioPhotosApp) showImage(ops *op.Ops) {
	img, size, err := g.getImage()
	if err != nil {
		return
	}
	r1 := float64(size.X) / g.width
	r2 := float64(size.Y) / g.height
	r := float32(math.Max(r1, r2))
	img.Add(ops)
	op.Affine(f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(r, r))).Add(ops)
	paint.PaintOp{}.Add(ops)
}

type Action string

const (
	Previous Action = "previous"
	Next     Action = "next"
	None     Action = "none"
)

func checkEvent(gtx layout.Context) Action {
	ev, b := gtx.Event(key.Filter{})
	if !b {
		return None
	}
	event := ev.(key.Event)
	if event.State == 1 {
		switch event.Name {
		case "→":
			return Next
		case "←":
			return Previous
		default:
			return None
		}
	}
	return None
}

var imgOp *paint.ImageOp

var pathImage = ""
var reload = false
var currentImage = 0
var currentFolder = ""
var images []string
var size image.Point

func (g GioPhotosApp) getImage() (paint.ImageOp, image.Point, error) {
	if pathImage != "" && (imgOp == nil || reload) {
		reload = false
		resp, err := g.doRequest(g.baseUrl + pathImage)
		if err != nil {
			return paint.ImageOp{}, image.Point{}, err
		}
		img, err2 := jpeg.Decode(resp.Body)
		if err2 != nil {
			return paint.ImageOp{}, image.Point{}, err
		}
		size = img.Bounds().Size()
		imageOp := paint.NewImageOp(img)
		imageOp.Filter = paint.FilterNearest
		imgOp = &imageOp
	}
	if imgOp == nil {
		return paint.ImageOp{}, image.Point{}, errors.New("no")
	}
	return *imgOp, size, nil
}

func (g GioPhotosApp) loadImages(url string) {
	log.Println("Load folder", url)
	currentFolder = url
	resp, _ := g.doRequest(g.baseUrl + url)
	m := make(map[string]interface{})
	data, _ := io.ReadAll(resp.Body)
	json.Unmarshal(data, &m)
	images = make([]string, 0)
	for _, img := range m["Files"].([]interface{}) {
		if link, exists := img.(map[string]interface{})["ImageLink"]; exists {
			images = append(images, link.(string))
		}
	}
}

func (g GioPhotosApp) doRequest(url string) (*http.Response, error) {
	r, _ := http.NewRequest("GET", url, nil)
	r.AddCookie(&http.Cookie{Name: "token", Value: g.token})
	return http.DefaultClient.Do(r)
}
