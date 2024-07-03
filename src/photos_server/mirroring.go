package photos_server

import (
	"github.com/jotitan/photos_server/config"
	"io"
	"os"
	"path/filepath"
)

type Mirroring interface {
	copy(inputPath, outputPath string) error
}

func newMirroring(conf config.MirroringConfig) Mirroring {
	switch conf.StorageType {
	case "filer":
		return MirroringReal{storage: newMirroringStorage(conf)}
	default:
		return MirroringOff{}
	}
}

type MirroringOff struct {
}

func (m MirroringOff) copy(_, _ string) error {
	return nil
}

type MirroringReal struct {
	storage     mirroringStorage
	consistency bool
}

func (m MirroringReal) copy(inputPath, outputPath string) error {
	// If consistency, wait the end of copy
	if m.consistency {
		return m.storage.copy(inputPath, outputPath)
	} else {
		go m.storage.copy(inputPath, outputPath)
		return nil
	}
}

// mirroringStorage : represent a kind of storage (filer, s3...)
type mirroringStorage interface {
	copy(inputPath, outputPath string) error
}

func newMirroringStorage(conf config.MirroringConfig) mirroringStorage {
	return filerStorage{
		folder: conf.Path,
	}
}

type filerStorage struct {
	folder string
}

func (f filerStorage) copy(inputPath, outputPath string) error {
	if inputFile, err := os.Open(inputPath); err == nil {
		defer inputFile.Close()
		fullOutputPath := filepath.Join(f.folder, outputPath)
		if err = os.MkdirAll(filepath.Dir(fullOutputPath), os.ModePerm); err != nil {
			return err
		}
		if newFile, err := os.OpenFile(fullOutputPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.ModePerm); err == nil {
			defer newFile.Close()
			if _, err := io.Copy(newFile, inputFile); err != nil {
				// Send Error to progresser and stop
				return err
			}
			return nil
		} else {
			return err
		}
	} else {
		return err
	}
}
