# Photos server

Simple website which display photos stored on a HDD.
Features : 
* Present folders as tree folder
* Display a thumbnail and lightbox to show pictures
* Possible to select pictures to delete (by moving in garbage)
* Possible to update a specific folder
* Possible to add a folder (api rest : /addFolder)

When indexing pictures, create two resized : 250 px and 1080 px height. 
Images are also rotated cause Chrome can't use exif orientation.

I extract date from exif to sort images in view.

## Tech

The light server is written in Go. The front is written in React 
* [react-grid-gallery](https://www.npmjs.com/package/react-grid-gallery)
* [ant design](https://ant.design/)

## Build

### Front

To build front : 
* npm install 
* npm run-script build

After that, use build as resources.

### Back

Goland dependencies : 
 * github.com/disintegration/imaging : Rotation images
 * github.com/nfnt/resize : Resize images
 * github.com/dsoprea/go-jpeg-image-structure
 * github.com/rwcarlsen/goexif/exif : Exif reader
 * github.com/dsoprea/go-exif/v2 : Exif writer 
 
Build : go build main/photos_server_run.go

## Run

To run server, used those options : 
* **-cache** : specify where reduced images must be saved
* **-resources** : folder where build front resources are
* -garbage : optional, folder where to move deleted files
* -mask-admin : mandatory to use garbage, mask on referer. Used to protect admin operation to be launch only at home on personal network

_Ex : ./photos_server_run -cache /data/cache -resources /appli/photos_resources -garbage /data/garbage -mask-admin localhost_

**Server run on port 9006**

At home, server run on Raspberry Pi 2 with an old SAN.
Indexation is quite slow but after that, displaying is very fast.