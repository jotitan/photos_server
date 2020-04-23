package main

import (
	"github.com/jotitan/photos_server/arguments"
	"github.com/jotitan/photos_server/photos_server"
)

func main(){
	args := arguments.NewArguments()
	cacheFolder := args.GetMandatoryString("cache","Argument -cache is mandatory to specify where pictures are resized")
	webResources := args.GetMandatoryString("resources","Argument -resources is mandatory to specify where web resources are")
	port := args.GetStringDefault("port","9006")
	garbage := args.GetString("garbage")
	maskForAdmin:= args.GetString("mask-admin")
	server := photos_server.NewPhotosServer(cacheFolder,webResources,garbage,maskForAdmin)
	server.Launch(port)
}
