package main

import (
	"github.com/jotitan/photos_server/arguments"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"github.com/jotitan/photos_server/photos_server"
	"github.com/jotitan/photos_server/tasks"
)

func test(){
	//h := photos_server.NewHSLLocalManager("C:\\Users\\jonathan.baranzini\\Downloads\\ffmpeg-20190519-fbdb3aa-win64-static\\bin\\ffmpeg.exe")
	//h.Convert("C:\\Perso\\20200804\\export\\hugo_danse.mp4","c:\\toto",[]string{"960x540","640x360"},[]string{"2000","365"})
	//c := h.Convert("C:\\Perso\\20200804\\export\\hugo_danse.mp4","c:\\toto",[]string{"640x360"},[]string{"365"})
	//fmt.Println(<-c)
}


func main(){
	args := arguments.NewArguments()
	pathConfig := args.GetMandatoryString("config","Argument -config is mandatory to specify path of YAML config")

	if conf,errConfig := config.ReadConfig(pathConfig) ; errConfig == nil {
		tasks.LaunchTasks(conf.Tasks)
		server := photos_server.NewPhotosServerFromConfig(conf)
		server.Launch(conf)
	}else{
		logger.GetLogger2().Error(errConfig.Error())
	}
}
