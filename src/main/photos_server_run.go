package main

import (
	"github.com/jotitan/photos_server/arguments"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"github.com/jotitan/photos_server/photos_server"
)

func main(){
	args := arguments.NewArguments()
	pathConfig := args.GetMandatoryString("config","Argument -config is mandatory to specify path of YAML config")

	if conf,errConfig := config.ReadConfig(pathConfig) ; errConfig == nil {
		server := photos_server.NewPhotosServerFromConfig(conf)
		server.Launch(conf)
	}else{
		logger.GetLogger2().Error(errConfig.Error())
	}
}
