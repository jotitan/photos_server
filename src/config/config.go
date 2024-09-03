package config

import (
	"strings"
)

type CronTask struct {
	Cron string `yaml:"cron"`
	Run  string `yaml:"run"`
}

type CronTasks []CronTask

// Config of oauth2
type OAuth2Config struct {
	// Can only be google for now
	Provider         string   `yaml:"provider"`
	ClientID         string   `yaml:"client_id"`
	ClientSecret     string   `yaml:"client_secret"`
	RedirectUrl      string   `yaml:"redirect_url"`
	AuthorizedEmails []string `yaml:"emails"`
	AdminEmails      []string `yaml:"admin_emails"`
	SuffixEmailShare []string `yaml:"suffix_email_share"`
}

type BasicConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type SecurityConfig struct {
	MaskForAdmin     string       `yaml:"mask-admin"`
	HS256SecretKey   string       `yaml:"secret"`
	UrlPublicKeys    string       `yaml:"url_public_keys"`
	SuffixEmailShare []string     `yaml:"suffix_email_share"`
	BasicConfig      BasicConfig  `yaml:"basic"`
	OAuth2Config     OAuth2Config `yaml:"oauth2"`
}

type PhotoConfig struct {
	Converter string `yaml:"converter"` // local | remote
	Url       string `yaml:"url"`       // Url of remote server
}

type VideoConfig struct {
	ExifTool               string `yaml:"exiftool"`
	FFMPEGPath             string `yaml:"ffmpeg"`
	ConvertServer          string `yaml:"convert-server"`
	OriginalUploadedFolder string `yaml:"original-upload-folder"`
	HLSUploadedFolder      string `yaml:"hls-upload-folder"`
}

type Config struct {
	CacheFolder          string          `yaml:"cache"`     // mandatory to specify where pictures are resized
	WebResources         string          `yaml:"resources"` //mandatory to specify where web resources are
	Port                 string          `yaml:"port"`      // default 9006
	VideoConfig          VideoConfig     `yaml:"video"`
	PhotoConfig          PhotoConfig     `yaml:"photo"`
	Garbage              string          `yaml:"garbage"`
	UploadedFolder       string          `yaml:"upload-folder"`
	OverrideUploadFolder string          `yaml:"override-upload"`
	Security             SecurityConfig  `yaml:"security"`
	Tasks                CronTasks       `yaml:"tasks"`
	Mirroring            MirroringConfig `yaml:"mirroring"`
}

type MirroringConfig struct {
	StorageType string `yaml:"type"`
	Path        string `yaml:"path"`
	// If true, wait before finishing migration
	Consistency bool `yaml:"consistency"`
}

// Check if the config is complete
func (c Config) Check() bool {
	return !strings.EqualFold("", c.CacheFolder) && !strings.EqualFold("", c.WebResources)
}
