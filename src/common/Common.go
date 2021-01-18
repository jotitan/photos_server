package common

import "time"

type INode interface {
	GetDate()time.Time
	GetFiles()map[string]INode
	GetIsFolder()bool
}

func ComputeNodeByDate(files map[string]INode) map[time.Time][]INode {
	byDate := make(map[time.Time][]INode)
	// Browse all pictures and group by date
	for _,node := range files {
		if node.GetIsFolder() {
			// Relaunch
			for date,nodes := range ComputeNodeByDate(node.GetFiles()) {
				addInTimeMap(byDate,date,nodes)
			}
		}else{
			formatDate := GetMidnightDate(node.GetDate())
			addInTimeMap(byDate,formatDate,[]INode{node})
		}
	}
	return byDate
}


func addInTimeMap(byDate map[time.Time][]INode,date time.Time,nodes []INode){
	if list,exist := byDate[date] ; !exist {
		byDate[date] = nodes
	}else{
		byDate[date] = append(list,nodes...)
	}
}


func GetMidnightDate(date time.Time)time.Time {
	if format,err := time.Parse("2006-01-02",date.Format("2006-01-02")) ; err == nil {
		return format
	}
	return date
}


type NodeByDate struct {
	Date time.Time
	Nb int
}
