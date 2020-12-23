package config

import (
	"strings"
)

type CronTask struct {
	Cron string `yaml:"cron"`
	Run string `yaml:"run"`
}

type CronTasks [] CronTask

// Config of oauth2
type OAuth2Config struct {
	// Can only be google for now
	Provider string `yaml:"provider"`
	ClientID string	`yaml:"client_id"`
	ClientSecret string	`yaml:"client_secret"`
	RedirectUrl string `yaml:"redirect_url"`
	AuthorizedEmails	[]string `yaml:"emails"`
	AdminEmails		[]string `yaml:"admin_emails"`
	SuffixEmailShare []string `yaml:"suffix_email_share"`
}

type BasicConfig struct {
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
}

type SecurityConfig struct {
	MaskForAdmin   string `yaml:"mask-admin"`
	HS256SecretKey string `yaml:"secret"`
	BasicConfig BasicConfig `yaml:"basic"`
	OAuth2Config OAuth2Config `yaml:"oauth2"`
}

type VideoConfig struct {
	ExifTool               string `yaml:"exiftool"`
	FFMPEGPath             string `yaml:"ffmpeg"`
	ConvertServer          string `yaml:"convert-server"`
	OriginalUploadedFolder string `yaml:"original-upload-folder"`
	HLSUploadedFolder      string `yaml:"hls-upload-folder"`
}

type Config struct {
	CacheFolder string	`yaml:"cache"` // mandatory to specify where pictures are resized
	WebResources string	`yaml:"resources"`	//mandatory to specify where web resources are
	Port string	`yaml:"port"` // default 9006
	VideoConfig VideoConfig `yaml:"video"`
	Garbage string	`yaml:"garbage"`
	UploadedFolder string `yaml:"upload-folder"`
	OverrideUploadFolder string  `yaml:"override-upload"`
	Security SecurityConfig `yaml:"security"`
	Tasks CronTasks `yaml:"tasks"`
}

//Check if the config is complete
func (c Config)Check()bool{
	return !strings.EqualFold("",c.CacheFolder) && !strings.EqualFold("",c.WebResources)
}