package photos_server

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	exifutil "github.com/dsoprea/go-exif/v2"
	"github.com/dsoprea/go-jpeg-image-structure"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"github.com/jotitan/photos_server/progress"
	"github.com/jotitan/photos_server/resize"
	"github.com/rwcarlsen/goexif/exif"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//Manage reducing pictures

type Reducer struct {
	// Where reduced images are created
	cache string
	// Differents sizes to produce
	sizes []uint
	// Receive an absolute path of image and a relative path to cache
	imagesToResize chan ImageToResize
	resize resize.GoResizerManager
	totalCount int
}

func NewReducer(conf config.Config, sizes []uint)Reducer{
	r := Reducer{
		cache: conf.CacheFolder,
		sizes:sizes,
		imagesToResize:make(chan ImageToResize,100),
	}
	if strings.EqualFold(conf.PhotoConfig.Converter,"remote") {
		r.resize = resize.NewHttpGoResizer(conf.PhotoConfig.Url)
	}else{
		r.resize = resize.NewAsyncGoResize()
	}
	go r.listenAndResize()
	return r
}

type ImageToResize struct{
	path         string
	relativePath string
	// Override cache resize folder by adding the folder
	overrideOutput string
	node         * Node
	waiter       *progress.UploadProgress
	forceRotate  bool
	existings    map[string]struct{}
}

func setExif(path string,orientation int,date time.Time)bool{
	jmp := jpegstructure.NewJpegMediaParser()
	media,_ := jmp.ParseFile(path)
	sl := media.(*jpegstructure.SegmentList)
	root := exifutil.NewIfdBuilder(exifutil.NewIfdMappingWithStandard(),exifutil.NewTagIndex(),"IFD",binary.LittleEndian)

	idfBuilder,_ := exifutil.GetOrCreateIbFromRootIb(root,"IFD0")
	idfBuilder.SetStandardWithName("Orientation",string(byte(orientation)))
	updatedTimestampPhrase := exifutil.ExifFullTimestampString(date)

	idfBuilder.SetStandardWithName("DateTime", updatedTimestampPhrase)
	idfBuilder.SetStandardWithName("DateTimeOriginal", updatedTimestampPhrase)
	sl.SetExif(root)
	f,_ := os.OpenFile(path,os.O_RDWR|os.O_CREATE|os.O_TRUNC,os.ModePerm)
	defer f.Close()
	return sl.Write(f) == nil
}


// @param updateExif : if true, update exif (datePhoto & orientation) on each resized photo
func (itr ImageToResize)update(h,w uint, datePhoto time.Time, orientation int, conversions []resize.ImageToResize, updateExif bool){
	itr.node.Height = int(h)
	itr.node.Width = int(w)
	itr.node.Date = datePhoto
	// Useless in fact
	if updateExif {
		logger.GetLogger2().Info("Update exif of",itr.path,orientation)
		for _,img := range conversions {
			setExif(img.To,orientation,datePhoto)
		}
	}
	itr.node.ImagesResized = true
	itr.waiter.Done()
}

func (r Reducer)AddImage(path,relativePath,overrideOutput string,node * Node,progresser *progress.UploadProgress, existings map[string]struct{}, forceRotate bool){
	r.imagesToResize <- ImageToResize{path,relativePath,overrideOutput,node,progresser,forceRotate,existings}
}

// Return number of images wating to reduce and number of images reduced
func (r * Reducer)Stat()(int,int){
	return len(r.imagesToResize),r.totalCount
}

func (r * Reducer)listenAndResize(){
	go func(){
		for {
			imageToResize := <-r.imagesToResize
			r.totalCount++
			targetFolder := filepath.Dir(imageToResize.relativePath)
			folder := filepath.Join(r.cache, imageToResize.overrideOutput,targetFolder)
			if r.createPathInCache(folder) == nil {
				r.resizeMultiformat(imageToResize,folder)
			}
		}
	}()
}

// Called when index photo or update
func GetExif(path string)(time.Time,int){
	if f,err := os.Open(path) ; err == nil {
		defer f.Close()
		if infos,err := exif.Decode(f) ; err == nil {
			return getExifDate(infos,path),getExifOrientation(infos)
		}
	}
	return getModificationDate(path),0
}

func getModificationDate(path string)time.Time{
	if f,err := os.Open(path) ; err == nil {
		defer f.Close()
		if s,err := f.Stat();err == nil {
			return s.ModTime()
		}
	}
	return time.Now()
}

func getExifDate(infos *exif.Exif,path string)time.Time{
	date := getExifValue(infos,exif.DateTimeDigitized)
	if strings.EqualFold("",date){
		if date = getExifValue(infos,exif.DateTime) ; strings.EqualFold("",date) {
			// If no exif date, use modification date
			return getModificationDate(path)
		}
	}

	if d,err := time.Parse("\"2006:01:02 15:04:05\"",date) ; err == nil {
		return d
	}
	return time.Now()
}

// Return angle in degres
func getExifOrientation(infos *exif.Exif)int{
	if value,err := strconv.ParseInt(getExifValue(infos,exif.Orientation),10,32); err == nil {
		return int(value)
	}
	return 0
	/*
		1 : 0, 8 : 90, 3 : 180, 6 : 270
	*/
}

func getExifValue(infos *exif.Exif, field exif.FieldName)string{
	if d,err := infos.Get(field) ; err == nil {
		return d.String()
	}
	return ""
}

func (r Reducer) resizeMultiformat(imageToResize ImageToResize,folder string){
	// Reuse computed image to accelerate
	from := imageToResize.path
	datePhoto,orientation := GetExif(from)
	// Check if both exist, if true, return, otherwise, resize
	conversions,alreadyExist := r.checkAlreadyExist(folder,imageToResize)
	if alreadyExist {
		// All exist, get Size of little one and return
		r.treatAlreadyExist(conversions,datePhoto,orientation,imageToResize)
		return
	}
	callback := func(err error,width,height uint,correctOrientation int){
		if err != nil {
			logger.GetLogger2().Info("Got Error on resize",from,err)
			imageToResize.waiter.Done()
		}else{
			if width != 0 && height != 0 {
				imageToResize.update(height,width,datePhoto,correctOrientation,conversions,true)
			}
		}
	}
	r.resize.ResizeAsync(from,orientation,conversions,callback)
}

func (r Reducer)checkAlreadyExist(folder string,imageToResize ImageToResize)([]resize.ImageToResize,bool){
	conversions := make([]resize.ImageToResize,len(r.sizes))
	nbExist := 0
	for i, size := range r.sizes {
		conversions[i] = resize.ImageToResize{To:r.createJpegFile(folder,imageToResize.path,size),Width:0,Height:size}
		if _,exist := imageToResize.existings[conversions[i].To]; exist {
			nbExist++
		}
	}
	return conversions,nbExist == len(r.sizes)
}

func (r Reducer)treatAlreadyExist(conversions []resize.ImageToResize,datePhoto time.Time,orientation int, imageToResize ImageToResize){
	// All exist, get Size of little one and return
	w,h := resize.GetSize(conversions[len(conversions)-1].To)
	logger.GetLogger2().Info("Image already exist",imageToResize.path, "extract infos",w,h,orientation,datePhoto)
	// If force rotate, rotate images and set exif orientation to 0
	if imageToResize.forceRotate && orientation != 1{
		// If rotation is 90 or -90, change w and h
		if orientation %2 == 0 {
			w,h = h,w
		}
		// Rotate all images and set exit on all
		for _,c := range conversions {
			r.rotateImage(c.To,orientation)
		}
		orientation = 1
	}
	imageToResize.update(h,w,datePhoto,orientation,conversions,true)
}

func (r Reducer)rotateImage(path string,orientation int){
	if f,err := os.Open(path) ; err == nil {
		logger.GetLogger2().Info("Launch rotate image",path,orientation)
		img,_ :=jpeg.Decode(f)
		f.Close()
		angle := resize.CorrectRotation(orientation)
		img = imaging.Rotate(img,float64(angle),color.Transparent)
		f,_ := os.OpenFile(path,os.O_TRUNC|os.O_RDWR|os.O_CREATE,os.ModePerm)
		if err := jpeg.Encode(f,img,&(jpeg.Options{75})) ; err != nil {
			logger.GetLogger2().Error("Impsosible to rotate image",err)
		}
		f.Close()
	}
}

func (r Reducer)createPathInCache(path string)error{
	if f,err := os.Open(path) ; err != nil {
		// Create folder
		return os.MkdirAll(path,os.ModePerm)
	}else{
		defer f.Close()
		if stat,err := f.Stat() ; err != nil || !stat.IsDir(){
			return errors.New("Impossible to use this folder : "  + path)
		}
	}
	return nil
}

func (r Reducer)createJpegFile(folder, basePath string, size uint)string{
	return filepath.Join(folder, r.CreateJpegName(filepath.Base(basePath), size))
}

// Generate a jpeg name from size
func (r Reducer)CreateJpegName(name string, size uint)string{
	extension := filepath.Ext(name)
	baseName := name[:len(name) - len(extension)]
	return fmt.Sprintf("%s-%d%s",baseName,size,".jpg")
}
