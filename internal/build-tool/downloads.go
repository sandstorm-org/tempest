package buildtool

import (
	"github.com/BurntSushi/toml"
)

type DownloadsTomlTopLevel struct {
	TinyGo DownloadsTomlTinyGo `toml:"tinygo"`
}

type DownloadsTomlTinyGo struct {
	DownloadUrlTemplate string
	FilenameTemplate    string
	Files               map[string]DownloadsTomlFile
	PreferredVersion    string
}

type DownloadsTomlFile struct {
	Sha256 string `toml:"SHA-256"`
	Size   int64
}

func LoadDownloadsFile(downloadsFilePath *string) (*DownloadsTomlTopLevel, error) {
	downloads := new(DownloadsTomlTopLevel)
	_, err := toml.DecodeFile(*downloadsFilePath, downloads)
	if err != nil {
		return nil, err
	}
	return downloads, nil
}
