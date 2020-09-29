package tasks

import (
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"github.com/robfig/cron"
	"os"
	"os/exec"
	"strings"
)

func LaunchTasks(tasks config.CronTasks){
	if len(tasks) == 0 {
		return
	}
	c := cron.New()
	for _,task := range tasks {
		if err := c.AddFunc(task.Cron,func(t config.CronTask)func(){
			return func(){
				splits := strings.Split(t.Run," ")
				cmd := exec.Command(splits[0],splits[1:]...)
				cmd.Stdout = os.Stdout
				if err := cmd.Run() ; err != nil {
					logger.GetLogger2().Error("impossible to run command",t.Run,":",err.Error())
				}else{
					logger.GetLogger2().Info("Task",t.Run,"run correctly")
				}
			}
		}(task)) ; err != nil {
			logger.GetLogger2().Error("impossible to add this cron")
		}else{
			logger.GetLogger2().Info("Add task",task.Run)
		}
	}
	go c.Run()
}