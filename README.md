# Photos server ![CI build photos app](https://github.com/jotitan/photos_server/workflows/CI%20build%20photos%20app/badge.svg?branch=master)

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

To run server, specify yaml configuration file with -config option.
Configuration Yaml File :  
```yaml
cache: <specify where reduced images must be saved (mandatory)>
resources: <folder where build front resources are (mandatory)>
port: <to ovveride default port (9006)>
garbage: <folder where to move deleted files>
upload-folder: <folder where to upload pictures>
security:    
  mask-admin: <mandatory to use garbage, mask on referer. Used to protect admin operation to be launch only at home on personal network>
  username: <username to get admin access outside>
  password: <password linked to user>
  secret: <key used to sign jwt Token (HS256) (https://mkjwk.org/ > oct / HS256)>
```
**Server run on port 9006**

Https is not enaled cause I'm using secured proxy in front.

At home, server run on Raspberry Pi 2 with an old SAN.
Indexation is quite slow but after that, displaying is very fast.
