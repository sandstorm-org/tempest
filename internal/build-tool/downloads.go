package buildtool

import (
	"github.com/BurntSushi/toml"
)

type DownloadsTomlTopLevel struct {
	Bison  DownloadsTomlBison  `toml:"bison"`
	Flex   DownloadsTomlFlex   `toml:"flex"`
	Linux  DownloadsTomlLinux  `toml:"linux"`
	TinyGo DownloadsTomlTinyGo `toml:"tinygo"`
}

type DownloadsTomlBison struct {
	DownloadUrlTemplate string
	FilenameTemplate    string
	Files               map[string]DownloadsTomlFile
	PreferredVersion    string
}

type DownloadsTomlFlex struct {
	DownloadUrlTemplate string
	FilenameTemplate    string
	Files               map[string]DownloadsTomlFile
	PreferredVersion    string
}

type DownloadsTomlLinux struct {
	DownloadUrlTemplate string
	FilenameTemplate    string
	Files               map[string]DownloadsTomlFile
	PreferredVersion    string
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

func ReadDownloadsFile(downloadsFilePath *string) (*DownloadsTomlTopLevel, error) {
	downloads := new(DownloadsTomlTopLevel)
	_, err := toml.DecodeFile(*downloadsFilePath, downloads)
	if err != nil {
		return nil, err
	}
	return downloads, nil
}
