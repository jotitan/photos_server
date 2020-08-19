package config

import "strings"

type Config struct {
	CacheFolder string	`yaml:"cache"` // mandatory to specify where pictures are resized
	WebResources string	`yaml:"resources"`	//mandatory to specify where web resources are
	Port string	`yaml:"port"` // default 9006
	Garbage string	`yaml:"garbage"`
	UploadedFolder string `yaml:"upload-folder"`
	OverrideUploadFolder string  `yaml:"override-upload"`
	Security struct {
		MaskForAdmin   string `yaml:"mask-admin"`
		Username       string `yaml:"username"`
		Password       string `yaml:"password"`
		HS256SecretKey string `yaml:"secret"`
	}
}

//Check if the config is complete
func (c Config)Check()bool{
	return !strings.EqualFold("",c.CacheFolder) && !strings.EqualFold("",c.WebResources)
}