package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// UpdateDateTaken met à jour la date EXIF (DateTimeOriginal/CreateDate/ModifyDate).
// Requiert exiftool installé et accessible dans le PATH.
func UpdateDateTaken(exifToolPath, filePath string, takenAt time.Time) error {
	// Format EXIF attendu: "YYYY:MM:DD HH:MM:SS"
	exifTime := takenAt.Format("2006:01:02 15:04:05")

	// -overwrite_original évite de créer un _original
	// -AllDates met à jour DateTimeOriginal, CreateDate, ModifyDate (JPEG/vidéo)
	cmd := exec.Command(
		exifToolPath,
		"-overwrite_original",
		"-AllDates="+exifTime,
		filePath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool error: %v, output: %s", err, string(out))
	}
	return nil
}

func toInt(v string) int {
	i, _ := strconv.Atoi(v)
	return i
}

func main() {
	if len(os.Args) != 4 {
		log.Fatal("Need to specify arguments : <folder photos> <exif path> <date start>")
	}
	folder := os.Args[1]
	exifTool := os.Args[2]
	dateStart := strings.Split(os.Args[3], "-")
	year, month, day, hour := toInt(dateStart[0]), time.Month(toInt(dateStart[1])), toInt(dateStart[2]), toInt(dateStart[3])
	dir, err := os.Open(folder)
	if err != nil {
		log.Fatal("Impossible to open folder", err)
	}
	files, _ := dir.Readdirnames(-1)
	for i, file := range files {
		path := filepath.Join(folder, file)
		t := time.Date(year, month, day, hour, 0+i, 0, 0, time.Local)
		if err := UpdateDateTaken(exifTool, path, t); err != nil {
			log.Println("Error:", err)
		} else {
			log.Println("Update picture", path)
		}
	}

}
