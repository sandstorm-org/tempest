// Tempest
// Copyright (c) 2025 Sandstorm Development Team and contributors
// All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package buildtool

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
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
	Bison                ConfigTomlBison  `toml:"bison"`
	BpfAsm               ConfigTomlBpfAsm `toml:"bpf_asm"`
	Flex                 ConfigTomlFlex   `toml:"flex"`
	Linux                ConfigTomlLinux  `toml:"linux"`
	TinyGo               ConfigTomlTinyGo `toml:"tinygo"`
}

type ConfigTomlBison struct {
	DownloadUrl string
	Executable  string
	Version     string
}

type ConfigTomlBpfAsm struct {
	Executable string
}

type ConfigTomlFlex struct {
	DownloadUrl string
	Executable  string
	Version     string
}

type ConfigTomlLinux struct {
	DownloadUrl string
	Version     string
}

type ConfigTomlTinyGo struct {
	DownloadUrl string
	Executable  string
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
	bison             *runtimeConfigBison
	bpfAsm            *runtimeConfigBpfAsm
	flex              *runtimeConfigFlex
	linux             *runtimeConfigLinux
	tinyGo            *runtimeConfigTinyGo
}

type runtimeConfigBison struct {
	downloadUrlTemplate string
	executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	version             string
}

type runtimeConfigBpfAsm struct {
	executable string
}

type runtimeConfigFlex struct {
	downloadUrlTemplate string
	executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	version             string
}

type runtimeConfigLinux struct {
	downloadUrlTemplate string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	version             string
}

type runtimeConfigTinyGo struct {
	downloadUrlTemplate string
	executable          string
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
	var toolchainToml *ToolchainTomlTopLevel
	toolchainToml, err = ReadToolchainToml(config.toolChainDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		toolchainToml = new(ToolchainTomlTopLevel)
	}
	config.bison = new(runtimeConfigBison)
	err = populateBisonRuntimeConfig(config.bison, &configFile.BuildTool.Bison, &downloadsFile.Bison, toolchainToml)
	if err != nil {
		return nil, err
	}
	config.bpfAsm = new(runtimeConfigBpfAsm)
	err = populateBpfAsmRuntimeConfig(config.bpfAsm, &configFile.BuildTool.BpfAsm, toolchainToml)
	if err != nil {
		return nil, err
	}
	config.flex = new(runtimeConfigFlex)
	err = populateFlexRuntimeConfig(config.flex, &configFile.BuildTool.Flex, &downloadsFile.Flex, toolchainToml)
	if err != nil {
		return nil, err
	}
	config.linux = new(runtimeConfigLinux)
	err = populateLinuxRuntimeConfig(config.linux, &configFile.BuildTool.Linux, &downloadsFile.Linux)
	if err != nil {
		return nil, err
	}
	config.tinyGo = new(runtimeConfigTinyGo)
	err = populateTinyGoRuntimeConfig(config.tinyGo, &configFile.BuildTool.TinyGo, &downloadsFile.TinyGo, toolchainToml)
	if err != nil {
		return nil, err
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

func populateBisonRuntimeConfig(runtimeConfig *runtimeConfigBison, configFile *ConfigTomlBison, downloadsFile *DownloadsTomlBison, toolchainToml *ToolchainTomlTopLevel) error {
	if configFile.DownloadUrl != "" {
		runtimeConfig.downloadUrlTemplate = configFile.DownloadUrl
	} else {
		runtimeConfig.downloadUrlTemplate = downloadsFile.DownloadUrlTemplate
	}
	if configFile.Executable != "" {
		runtimeConfig.executable = configFile.Executable
	} else if toolchainToml.Bison != nil && toolchainToml.Bison.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.Bison.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.executable = absExecutable
	} else {
		// Expect the user to have bison in the PATH.
		runtimeConfig.executable = "bison"
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if configFile.Version != "" {
		runtimeConfig.version = configFile.Version
	} else {
		runtimeConfig.version = downloadsFile.PreferredVersion
	}
	if runtimeConfig.version == "" {
		return fmt.Errorf("Bison has no configured version")
	}
	runtimeConfig.files = make(map[string]runtimeConfigFile)
	for fileName, fileStruct := range downloadsFile.Files {
		runtimeConfig.files[fileName] = runtimeConfigFile{
			fileStruct.Sha256,
			fileStruct.Size,
		}
	}
	return nil
}

func populateBpfAsmRuntimeConfig(runtimeConfig *runtimeConfigBpfAsm, configFile *ConfigTomlBpfAsm, toolchainToml *ToolchainTomlTopLevel) error {
	if configFile.Executable != "" {
		runtimeConfig.executable = configFile.Executable
	} else if toolchainToml.BpfAsm != nil && toolchainToml.BpfAsm.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.BpfAsm.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.executable = absExecutable
	} else {
		// Expect the user to have bpf_asm in the PATH.
		runtimeConfig.executable = "bpf_asm"
	}
	return nil
}

func populateFlexRuntimeConfig(runtimeConfig *runtimeConfigFlex, configFile *ConfigTomlFlex, downloadsFile *DownloadsTomlFlex, toolchainToml *ToolchainTomlTopLevel) error {
	if configFile.DownloadUrl != "" {
		runtimeConfig.downloadUrlTemplate = configFile.DownloadUrl
	} else {
		runtimeConfig.downloadUrlTemplate = downloadsFile.DownloadUrlTemplate
	}
	if configFile.Executable != "" {
		runtimeConfig.executable = configFile.Executable
	} else if toolchainToml.Flex != nil && toolchainToml.Flex.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.Flex.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.executable = absExecutable
	} else {
		// Expect the user to have flex in the PATH.
		runtimeConfig.executable = "flex"
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if configFile.Version != "" {
		runtimeConfig.version = configFile.Version
	} else {
		runtimeConfig.version = downloadsFile.PreferredVersion
	}
	if runtimeConfig.version == "" {
		return fmt.Errorf("Flex has no configured version")
	}
	runtimeConfig.files = make(map[string]runtimeConfigFile)
	for fileName, fileStruct := range downloadsFile.Files {
		runtimeConfig.files[fileName] = runtimeConfigFile{
			fileStruct.Sha256,
			fileStruct.Size,
		}
	}
	return nil
}

func populateLinuxRuntimeConfig(runtimeConfig *runtimeConfigLinux, configFile *ConfigTomlLinux, downloadsFile *DownloadsTomlLinux) error {
	if configFile.DownloadUrl != "" {
		runtimeConfig.downloadUrlTemplate = configFile.DownloadUrl
	} else {
		runtimeConfig.downloadUrlTemplate = downloadsFile.DownloadUrlTemplate
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if configFile.Version != "" {
		runtimeConfig.version = configFile.Version
	} else {
		runtimeConfig.version = downloadsFile.PreferredVersion
	}
	if runtimeConfig.version == "" {
		return fmt.Errorf("Linux has no configured version")
	}
	runtimeConfig.files = make(map[string]runtimeConfigFile)
	for fileName, fileStruct := range downloadsFile.Files {
		runtimeConfig.files[fileName] = runtimeConfigFile{
			fileStruct.Sha256,
			fileStruct.Size,
		}
	}
	return nil
}

func populateTinyGoRuntimeConfig(runtimeConfig *runtimeConfigTinyGo, configFile *ConfigTomlTinyGo, downloadsFile *DownloadsTomlTinyGo, toolchainToml *ToolchainTomlTopLevel) error {
	if configFile.DownloadUrl != "" {
		runtimeConfig.downloadUrlTemplate = configFile.DownloadUrl
	} else {
		runtimeConfig.downloadUrlTemplate = downloadsFile.DownloadUrlTemplate
	}
	if configFile.Executable != "" {
		runtimeConfig.executable = configFile.Executable
	} else if toolchainToml.TinyGo != nil && toolchainToml.TinyGo.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.TinyGo.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.executable = absExecutable
	} else {
		// Expect the user to have tinygo in the PATH.
		runtimeConfig.executable = "tinygo"
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if configFile.Version != "" {
		runtimeConfig.version = configFile.Version
	} else {
		runtimeConfig.version = downloadsFile.PreferredVersion
	}
	if runtimeConfig.version == "" {
		return fmt.Errorf("TinyGo has no configured version")
	}
	runtimeConfig.files = make(map[string]runtimeConfigFile)
	for fileName, fileStruct := range downloadsFile.Files {
		runtimeConfig.files[fileName] = runtimeConfigFile{
			fileStruct.Sha256,
			fileStruct.Size,
		}
	}
	return nil
}

func ReadConfigFile(configFilePath *string) (*ConfigTomlTopLevel, error) {
	config := new(ConfigTomlTopLevel)
	_, err := toml.DecodeFile(*configFilePath, config)
	return config, err
}
