package buildtool

import (
	"bytes"
	"fmt"
	"os/user"
	"text/template"

	"github.com/BurntSushi/toml"
)

// Config file types
//
// Config.toml parsing may be used outside this package, so struct names and
// field names should be in PascalCase.  Prefer lower-case for table names in
// the config file.

type ConfigTomlTopLevel struct {
	Tempest   ConfigTomlTempest   `toml:"tempest"`
	BuildTool ConfigTomlBuildTool `toml:"build-tool"`
}

type ConfigTomlTempest struct {
	User  string
	Group string
}

type ConfigTomlBuildTool struct {
	ToolChainDirTemplate string
	DownloadDirTemplate  string
	DownloadUserAgent    string
	DownloadsFile        string
	TinyGo               ConfigTomlTinyGo `toml:"tinygo"`
}

type ConfigTomlTinyGo struct {
	DownloadUrl string
	Version     string
}

type configTomlDirTemplateValues struct {
	Home string
}

// Runtime config types
//
// Runtime configuration is private to the package, so struct names and field
// names should be in camelCase.

type RuntimeConfigBuildTool struct {
	toolChainDir      string
	downloadDir       string
	downloadUserAgent string
	tinyGo            *runtimeConfigTinyGo
}

type runtimeConfigTinyGo struct {
	downloadUrlTemplate string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	version             string
}

type runtimeConfigFile struct {
	sha256 string
	size   int64
}

func BuildConfiguration(configFile *ConfigTomlTopLevel, downloadsFile *DownloadsTomlTopLevel) (*RuntimeConfigBuildTool, error) {
	config := new(RuntimeConfigBuildTool)
	var err error
	config.toolChainDir, err = buildDirWithTemplate("toolchainDir", configFile.BuildTool.ToolChainDirTemplate)
	if err != nil {
		return nil, err
	}
	config.downloadDir, err = buildDirWithTemplate("downloadDir", configFile.BuildTool.DownloadDirTemplate)
	if err != nil {
		return nil, err
	}
	config.downloadUserAgent = configFile.BuildTool.DownloadUserAgent
	config.tinyGo = new(runtimeConfigTinyGo)
	if configFile.BuildTool.TinyGo.DownloadUrl != "" {
		config.tinyGo.downloadUrlTemplate = configFile.BuildTool.TinyGo.DownloadUrl
	} else {
		config.tinyGo.downloadUrlTemplate = downloadsFile.TinyGo.DownloadUrlTemplate
	}
	config.tinyGo.filenameTemplate = downloadsFile.TinyGo.FilenameTemplate
	if configFile.BuildTool.TinyGo.Version != "" {
		config.tinyGo.version = configFile.BuildTool.TinyGo.Version
	} else {
		config.tinyGo.version = downloadsFile.TinyGo.PreferredVersion
	}
	if config.tinyGo.version == "" {
		return nil, fmt.Errorf("TinyGo has no configured version")
	}
	config.tinyGo.files = make(map[string]runtimeConfigFile)
	for fileName, fileStruct := range downloadsFile.TinyGo.Files {
		config.tinyGo.files[fileName] = runtimeConfigFile{
			fileStruct.Sha256,
			fileStruct.Size,
		}
	}
	return config, nil
}

func buildDirWithTemplate(templateName string, dirTemplate string) (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	homeDirectory := user.HomeDir
	values := configTomlDirTemplateValues{
		homeDirectory,
	}
	parsedTemplate, err := template.New(templateName).Parse(dirTemplate)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	err = parsedTemplate.Execute(&buffer, values)
	if err != nil {
		return "", err
	}
	result := buffer.String()
	return result, nil
}

func LoadConfigFile(configFilePath *string) (*ConfigTomlTopLevel, error) {
	config := new(ConfigTomlTopLevel)
	_, err := toml.DecodeFile(*configFilePath, config)
	return config, err
}
