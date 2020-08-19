package config

import (
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v2"
	"os"
)

func ReadConfig(path string)(*Config,error){
	if f,err := os.Open(path) ; err == nil {
		decoder := yaml.NewDecoder(f)
		conf := &Config{Port:"9006"}
		if err := decoder.Decode(conf) ; err == nil {
			if conf.Check() {
				return conf,nil
			}
			return nil,errors.New("Mauvaise configuration")
		}
		return nil,err
	}else{
		return nil,err
	}
}
