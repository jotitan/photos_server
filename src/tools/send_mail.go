package main

import (
	"fmt"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure"

	"github.com/jotitan/photos_server/common"
)

func main2() {
	//fmt.Println(common.SendEmail("smtp.gmail.com", 465, "titanbar@gmail.com", "Eefuth1a", "titanbar@gmail.com", []string{"titanbar@gmail.com"}, "Test Send Email", "Envoie de mail de test", false))
	fmt.Println(common.SendEmail("in-v3.mailjet.com", 587, "c4a29ddf2c4b37ca4cbb996c088f1d88", "e6acb48a9e2576b112eef9e80b9bedb5", "titanbar@hotmail.com", []string{"titanbar@gmail.com"}, "Test Send Email", "Envoie de mail de test", false))
}

func main() {
	path := ""
	jmp := jpegstructure.NewJpegMediaParser()
	media, _ := jmp.ParseFile(path)
	sl := media.(*jpegstructure.SegmentList)
}
