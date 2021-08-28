package people_tag

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/jotitan/photos_server/logger"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
)

const (
	headerSize = 5000
)

type HeaderElement struct{
	idFolder int
	position int64
	// First byte means delete, second byte means empty
	flags byte
}

func (he HeaderElement)isToDelete()bool{
	return he.flags & 1 == 1
}

func (he HeaderElement)isEmpty()bool{
	return he.flags & 2 == 2
}

func(he *HeaderElement)toDelete(){
	he.flags = byte(1)
}

func (he *HeaderElement)empty(){
	he.flags = he.flags | 2
}

/**
A tag is a person identify with is id
*/

type PeopleTagHeader struct {
	size int16
	nbWritten int16
	// header are sorted to improve search (dicotomy)
	elements []*HeaderElement
	// Map to find quikly last block of folder
	latestFolder map[int]*HeaderElement
}

func (pth PeopleTagHeader)getFoldersId()[]int{
	ids := make([]int,0,len(pth.latestFolder))
	for e,h := range pth.latestFolder {
		if !h.isEmpty() {
			ids = append(ids, e)
		}
	}
	return ids
}

func (pth PeopleTagHeader)sizeHeader()int{
	return 4 + 11 * headerSize
}

func (pth * PeopleTagHeader)read(f *os.File){
	header := make([]byte,pth.sizeHeader())
	f.Read(header)

	pth.size=bytesToInt16(header[0:2])
	pth.nbWritten=bytesToInt16(header[2:4])
	pth.latestFolder=make(map[int]*HeaderElement)

	// Create full list of header and a map on latest
	pth.elements = make([]*HeaderElement,pth.nbWritten,pth.size)
	for i :=0 ; i < int(pth.nbWritten) ; i++ {
		data := header[4+11*i:4 + (i+1)*11]
		he := &HeaderElement{
			idFolder: int(bytesToInt16(data[0:2])),
			position: bytesToInt64(data[2:10]),
			flags:    data[10],
		}
		pth.elements[i] = he
		if !he.isToDelete() {
			pth.latestFolder[he.idFolder] = he
		}
	}
}

func (pth *PeopleTagHeader)disable(idFolder int){
	if latest,exist := pth.latestFolder[idFolder] ; exist {
		latest.toDelete()
	}
}

func (pth *PeopleTagHeader)add(he *HeaderElement){
	pth.elements = append(pth.elements,he)
	pth.latestFolder[he.idFolder] = he
	pth.nbWritten++
}

func (pth *PeopleTagHeader)has(idFolder int)bool{
	_,exist := pth.latestFolder[idFolder]
	return exist
}

// Write header
func (pth *PeopleTagHeader) getAsBytes()[]byte {
	// Sort header
	sort.SliceIsSorted(pth.elements,func(i,j int)bool{
		if pth.elements[i].idFolder == pth.elements[j].idFolder {
			return !pth.elements[i].isToDelete()
		}
		return pth.elements[i].idFolder > pth.elements[j].idFolder
	})
	// Create block data to write in one time
	data := make([]byte,pth.sizeHeader())
	writeInt16(data,pth.size,0)
	writeInt16(data,pth.nbWritten,2)

	for i,elem := range pth.elements {
		// idFolder(2) | position(8) | flags(1)
		writeInt16(data,int16(elem.idFolder),4+i*11)
		writeInt64(data,elem.position,4+i*11 + 2)
		data[4+i*11 + 10] = elem.flags
	}
	return data
}

// Store all path by folder for a specific tag id
type PeopleTag struct {
	idTag int
	// New path to save
	pathsToSave [][]string
	// Current position in file
	currentPosition int64
	// Header of file
	header *PeopleTagHeader
	// Where files are stored
	folder string
}

// Size block : blockSize (4) | nbPath(2) | [lengthPath(2) | path(xxx)]
func (pt PeopleTag)computeSizeBlock(paths []string)int64{
	count := int64(6)
	for _,path := range paths {
		count+=int64(2 + len(path))
	}
	return count
}

func (pt PeopleTag)readPaths(idFolder int)[]string{
	if f,err := os.Open(pt.getFilename()) ; err == nil {
		defer f.Close()
		if headerElement,exist := pt.header.latestFolder[idFolder] ; exist {
			// Read Blocksize first
			blockSizeData := make([]byte, 4)
			nbPathsData := make([]byte, 2)
			f.ReadAt(blockSizeData, headerElement.position)
			f.ReadAt(nbPathsData, headerElement.position+4)

			nbPaths := int(bytesToInt16(nbPathsData))
			blockData := make([]byte, bytesToInt32(blockSizeData))
			f.ReadAt(blockData, headerElement.position+6)

			pointer := 0
			paths := make([]string, nbPaths)
			for i := 0; i < nbPaths; i++ {
				// Read first byte (255 max length) for path length
				length := int(bytesToInt16(blockData[pointer : pointer+2]))
				paths[i] = string(blockData[pointer+2 : pointer+2+length])
				pointer += length + 2
			}
			return paths
		}
		return []string{}
	}
	return []string{}
}

func toSet(list []string)map[string]struct{}{
	m := make(map[string]struct{},len(list))
	for _,value := range list {
		m[value] = struct{}{}
	}
	return m
}

func substractList(list []string, sub map[string]struct{})[]string{
	newList := make([]string,0,len(list))
	for _,value :=range list {
		if _,exist := sub[value] ; !exist {
			newList = append(newList,value)
		}
	}
	return newList
}

func (pt *PeopleTag)Tag(idFolder int, paths, deleted []string){
	// Check if exist in header

	if pt.header.has(idFolder) {
		// extract path from file at good position
		existingPaths := pt.readPaths(idFolder)
		// remove deleted from existing paths
		existingPaths = substractList(existingPaths,toSet(deleted))
		// Store in list to save, not in map, impossible to read map (no multiple write)
		paths = append(existingPaths,paths...)
		pt.pathsToSave = append(pt.pathsToSave,paths)
		// Disable existing header
		pt.header.disable(idFolder)
	}else{
		pt.pathsToSave = append(pt.pathsToSave,paths)
	}
	h := &HeaderElement{idFolder: idFolder, position: pt.currentPosition, flags: 0}
	if len(paths) == 0 {
		h.empty()
	}
	pt.header.add(h)
	pt.currentPosition+=pt.computeSizeBlock(paths)
}

func (pt *PeopleTag)loadHeaderIfEmpty()error{
	if pt.header == nil || pt.header.nbWritten == 0 {
		if f, err := os.Open(pt.getFilename()); err == nil {
			defer f.Close()
			pt.header = &PeopleTagHeader{}
			pt.header.read(f)
		}else{
			return err
		}
	}
	return nil
}

func (pt *PeopleTag)Search(idFolder int)[]string{
	if err := pt.loadHeaderIfEmpty() ; err != nil {
		logger.GetLogger2().Error("Impossible to open file",err)
		return []string{}
	}
	if !pt.header.has(idFolder) {
		return []string{}
	}
	return pt.readPaths(idFolder)
}

func (pt PeopleTag)getFilename()string{
	return filepath.Join(pt.folder,fmt.Sprintf("people_tag_%d.data",pt.idTag))
}

// Reserve a header size to manage many folder, for instance 5000. If header to enough place, create a new file or compact
// Header contains : size(2) | nbWritten(2) | [ idFolder(2) | posInFile(8) | flags(1) ] x 5000
// Flags : First byte is one when block need to be delete (by compaction)
func (pt *PeopleTag)Save()error{
	// Save header first, append data at end of file
	if f,err := os.OpenFile(pt.getFilename(),os.O_RDWR |os.O_CREATE,os.ModePerm) ; err == nil {
		defer f.Close()
		f.WriteAt(pt.header.getAsBytes(),0)
		f.Seek(0,2)
		// Write new data at the end
		data := make([]byte,0)
		for _,paths := range pt.pathsToSave {
			// blocSize (4) | nbPath (2) | [ pathLength(2) | path (x) ]
			data = append(data,int32ToBytes(int32(pt.computeSizeBlock(paths)))...)
			data = append(data,int16ToBytes(int16(len(paths)))...)
			for _,path := range paths {
				data = append(data,int16ToBytes(int16(len(path)))...)
				data = append(data,[]byte(path)...)
			}
		}
		f.Write(data)
		return nil
	}else{
		return err
	}
}

// Create people tag and load from folder if exist
func NewPeopleTag(folder string, idTag int, loadHeader bool)(*PeopleTag,error){
	pt := PeopleTag{idTag:idTag,folder:folder,header:&PeopleTagHeader{},pathsToSave:make([][]string,0)}
	if loadHeader {
		if f, err := os.Open(pt.getFilename()); err == nil {
			defer f.Close()
			pt.header.read(f)
			// Current position at end of file
			pt.currentPosition, _ = f.Seek(0, 2)
		} else {
			//Create empty header
			pt.header = &PeopleTagHeader{
				size:         headerSize,
				nbWritten:    0,
				elements:     make([]*HeaderElement, 0, headerSize),
				latestFolder: make(map[int]*HeaderElement),
			}
			// Position is header size
			pt.currentPosition = int64(pt.header.sizeHeader())
		}
	}
	return &pt,nil
}

func int64ToBytes(value int64)[]byte{
	data := make([]byte,8)
	binary.LittleEndian.PutUint64(data,uint64(value))
	return data
}

func int16ToBytes(value int16)[]byte{
	data := make([]byte,2)
	binary.LittleEndian.PutUint16(data,uint16(value))
	return data
}

func int32ToBytes(value int32)[]byte{
	data := make([]byte,4)
	binary.LittleEndian.PutUint32(data,uint32(value))
	return data
}

func writeBytes(data,value []byte,offset int){
	for i := 0 ; i < len(value) ; i++ {
		data[offset + i] = value[i]
	}
}

func writeInt16(data []byte,value int16,offset int){
	writeBytes(data,int16ToBytes(value),offset)
}

func writeInt64(data []byte,value int64,offset int){
	writeBytes(data,int64ToBytes(value),offset)
}

func bytesToInt64(value []byte)int64{
	return int64(binary.LittleEndian.Uint64(value))
}

func bytesToInt32(value []byte)int32{
	return int32(binary.LittleEndian.Uint32(value))
}

func bytesToInt16(value []byte)int16{
	return int16(binary.LittleEndian.Uint16(value))
}

/**
Manage tagging people on pictures
*/
type PeopleTagManager struct{
	// Cache of peopletag, before flushing. Key is idTag
	peopleTags map[int]*PeopleTag
	folder string
}

type peopleTag struct {
	Id int `json:"id"`
	Name string	`json:"name"`
}

// return id
func AddPeopleTag(folder,name string)(int,error){
	// Load existent people, add new one
	filename := filepath.Join(folder,"peoples_tag.list")
	list := make([]peopleTag,0)
	if data,err := ioutil.ReadFile(filename) ; err == nil {
		json.Unmarshal(data, &list)
	}
	id := len(list)+1
	list = append(list,peopleTag{Id:id,Name:name})

	// Write file
	data,_ := json.Marshal(list)
	if err := ioutil.WriteFile(filename,data,os.ModePerm) ; err != nil {
		return 0,err
	}
	return id,nil
}

func GetPeoples(folder string)([]peopleTag,error) {
	// Load existent people, add new one
	filename := filepath.Join(folder, "peoples_tag.list")
	list := make([]peopleTag, 0)
	if data, err := ioutil.ReadFile(filename); err == nil {
		json.Unmarshal(data, &list)
		return list,nil
	}else{
		return nil,err
	}
}

func GetPeoplesAsByte(folder string)([]byte,error){
	// Load existent people, add new one
	filename := filepath.Join(folder,"peoples_tag.list")
	return ioutil.ReadFile(filename)
}

func NewPeopleTagManager(folder string)*PeopleTagManager{
	return &PeopleTagManager{peopleTags: make(map[int]*PeopleTag),folder:folder}
}


func (ptm *PeopleTagManager)getPeopleTag(idTag int)*PeopleTag{
	if pt,exist := ptm.peopleTags[idTag] ; exist {
		return pt
	}
	// otherwise return new one
	pt,_ := NewPeopleTag(ptm.folder,idTag,true)
	ptm.peopleTags[idTag] = pt
	return pt
}

func (ptm *PeopleTagManager)Tag(idFolder, idTag int,paths,deleted []string){
	pm := ptm.getPeopleTag(idTag)
	pm.Tag(idFolder,paths, deleted)
}

// Search all folders containing specific tag
func (ptm PeopleTagManager)SearchAllFolder(tag int)[]int{
	// Just read header
	pt := ptm.getPeopleTag(tag)
	if pt.loadHeaderIfEmpty() != nil {
		return []int{}
	}
	return pt.header.getFoldersId()
}

// Return, for each people tag, just paths of folder
func (ptm *PeopleTagManager)SearchFolder(folder int)map[int][]string{
	peoples,err := GetPeoples(ptm.folder)
	data := make(map[int][]string)
	if err != nil {
		return data
	}
	for _,p := range peoples {
		data[p.Id] = ptm.Search(folder,p.Id)
	}
	return data
}

func (ptm * PeopleTagManager)Search(idFolder,idTag int)[]string{
	pm := ptm.getPeopleTag(idTag)
	return pm.Search(idFolder)
}

// Save data in folder
func (ptm * PeopleTagManager)Flush()error{
	for _,pt := range ptm.peopleTags {
		if err := pt.Save() ; err != nil {
			return err
		}
	}
	return nil
}
