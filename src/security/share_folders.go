package security

import (
	"encoding/json"
	"errors"
	"github.com/jotitan/photos_server/logger"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type ShareUser struct {
	// Email or username
	Id string
	NbConnection int
	Folders map[string]struct{}
}

func newShareUser(id string)*ShareUser{
	return &ShareUser{Id:id,Folders:make(map[string]struct{})}
}

func (su * ShareUser)add(path string){
	su.Folders[path] = struct{}{}
}

func (su *ShareUser) remove(path string) {
	delete(su.Folders,path)
}

// Store for each email the autorized paths
type ShareFolders struct {
	pathsByUser map[string]*ShareUser
	usersByPath map[string]map[string]struct{}
	security * SecurityAccess
}

func NewShareFolders(security * SecurityAccess)*ShareFolders{
	shares := ShareFolders{
		pathsByUser:make(map[string]*ShareUser),
		usersByPath:make(map[string]map[string]struct{}),
		security:security}
	if err := shares.load(); err != nil {
		logger.GetLogger2().Error("Impossible to load shares",err.Error())
		return nil
	}
	logger.GetLogger2().Info("Load shares with",len(shares.pathsByUser),"user(s)")
	return &shares
}

func (shares ShareFolders)checkUser(email string)bool{
	return shares.security.accessProvider.CheckShareMailValid(email)
}

func ( shares * ShareFolders)Add(user,path string,checkPathExist func(path string)bool)error{
	path = cleanPath(path)
	if !shares.checkUser(user){
		return errors.New("impossible to add path for this user")
	}
	if !checkPathExist(path) {
		return errors.New("try to add share on unknown path")
	}
	if _,exist := shares.pathsByUser[user] ; !exist {
		shares.pathsByUser[user] = newShareUser(user)
	}
	shares.pathsByUser[user].add(path)
	if users,exist := shares.usersByPath[path] ; exist{
		users[user] = struct{}{}
	}else{
		shares.usersByPath[path] = map[string]struct{}{user: {}}
	}
	return shares.save()
}

func ( shares * ShareFolders)Remove(user,path string,checkPathExist func(path string)bool)error{
	path = cleanPath(path)
	if !checkPathExist(path) {
		return errors.New("try to add share on unknown path")
	}
	if shareUser,exist := shares.pathsByUser[user] ; exist {
		shareUser.remove(path)
	}
	if users,exist := shares.usersByPath[path] ; exist {
		delete(users,user)
	}

	return shares.save()
}

func (shares ShareFolders)getFilename()string{
	wd,_ := os.Getwd()
	return filepath.Join(wd,"shares.json")
}

func (shares * ShareFolders)load()error {
	if data, err := ioutil.ReadFile(shares.getFilename()); err == nil {
		if err := json.Unmarshal(data,&shares.pathsByUser) ; err != nil {
			return err
		}
		shares.buildUsersByPath()
	}
	return nil
}

func (shares * ShareFolders)buildUsersByPath(){
	shares.usersByPath = make(map[string]map[string]struct{})
	// For each path, store users
	for user,shareUser := range shares.pathsByUser {
		for path := range shareUser.Folders {
			if users,exist := shares.usersByPath[path] ; !exist {
				shares.usersByPath[path] = map[string]struct{}{user:{}}
			}else{
				users[user]=struct{}{}
			}
		}
	}
	logger.GetLogger2().Info("Load",len(shares.usersByPath),"path(s)")
}

// Save shares in file
func ( shares * ShareFolders)save()error{
	if f, err := os.OpenFile(shares.getFilename(),os.O_CREATE|os.O_RDWR|os.O_TRUNC,os.ModePerm);err == nil {
		defer f.Close()
		if data,err := json.Marshal(shares.pathsByUser) ; err == nil {
			f.Write(data)
		return nil
		}else{
			return err
		}
	}else{
		return err
	}
}

func (shares *ShareFolders)Connect(user string)bool{
	if guest ,exist := shares.pathsByUser[user] ; exist {
		guest.NbConnection++
		shares.save()
		return true
	}
	return false
}

func (shares *ShareFolders)Exist(user string)bool{
	_,exist := shares.pathsByUser[user]
	return exist
}

func (shares *ShareFolders)Get(user string)([]string,error){
	if userPaths,exist := shares.pathsByUser[user] ; exist {
		list := make([]string,0,len(userPaths.Folders))
		for value := range userPaths.Folders {
			list = append(list,value)
		}
		return list,nil
	}else{
		return nil,errors.New("unknown user")
	}
}

func (shares * ShareFolders) CanRead(user,path string) bool {
	if users,exist := shares.usersByPath[path] ; exist {
		_,exist := users[user]
		return exist
	}
	return false
}

func (shares * ShareFolders) GetUsersOfPath(path string) []string{
	return toList(shares.usersByPath[cleanPath(path)])
}

func cleanPath(path string)string{
	path = strings.ReplaceAll(path,"\\","/")
	if path[0] == '/' {
		return path[1:]
	}
	return path
}

func toList(m map[string]struct{})[]string{
	list := make([]string,0,len(m))
	for val := range m {
		list = append(list,val)
	}
	return list
}