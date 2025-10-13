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
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
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
	BuildDirTemplate     string
	DownloadDirTemplate  string
	DownloadUserAgent    string
	DownloadsFile        string
	ToolChainDirTemplate string

	Bison                ConfigTomlBison     `toml:"bison"`
	BpfAsm               ConfigTomlBpfAsm    `toml:"bpf_asm"`
	CapnProto            ConfigTomlCapnProto `toml:"capnproto"`
	Flex                 ConfigTomlFlex      `toml:"flex"`
	Go                   ConfigTomlGo        `toml:"go"`
	GoCapnp              ConfigTomlGoCapnp   `toml:"go-capnp"`
	Linux                ConfigTomlLinux     `toml:"linux"`
	TinyGo               ConfigTomlTinyGo    `toml:"tinygo"`
}

type ConfigTomlBison struct {
	DownloadUrl string
	Executable  string
	Version     string
}

type ConfigTomlBpfAsm struct {
	Executable string
	GoPath     string
}

type ConfigTomlCapnProto struct {
	DownloadUrl string
	Executable  string
	Version     string
}

type ConfigTomlFlex struct {
	DownloadUrl string
	Executable  string
	Version     string
}

type ConfigTomlGo struct {
	Executable     string
	GoPathTemplate string
}

type ConfigTomlGoCapnp struct {
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

type configGoPathTemplateValues struct {
	GoVersion    string
	Home         string
	ToolChainDir string
}

// Runtime config types
//
// Runtime configuration is private to the package, so struct names and field
// names should be in camelCase.

type RuntimeConfigBuildTool struct {
	toolChainDir      string
	BuildDir          string
	downloadDir       string
	downloadUserAgent string
	goExecutable      string
	goPath            string
	bison             *runtimeConfigBison
	bpfAsm            *runtimeConfigBpfAsm
	capnProto         *runtimeConfigCapnProto
	flex              *runtimeConfigFlex
	goCapnp           *runtimeConfigGoCapnp
	linux             *runtimeConfigLinux
	tinyGo            *runtimeConfigTinyGo
}

type runtimeConfigBison struct {
	downloadUrlTemplate string
	executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

type runtimeConfigBpfAsm struct {
	executable          string
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

type runtimeConfigCapnProto struct {
	downloadUrlTemplate string
	executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

type runtimeConfigFlex struct {
	downloadUrlTemplate string
	executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

type runtimeConfigGoCapnp struct {
	downloadUrlTemplate string
	executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	goExecutable        string
	goPath              string
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

type runtimeConfigLinux struct {
	downloadUrlTemplate string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	toolchainVersion    string
	version             string
}

type runtimeConfigTinyGo struct {
	downloadUrlTemplate string
	executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

type runtimeConfigFile struct {
	sha256 string
	size   int64
}

func BuildConfiguration(configFile *ConfigTomlTopLevel, downloadsFile *DownloadsTomlTopLevel) (*RuntimeConfigBuildTool, error) {
	config := new(RuntimeConfigBuildTool)
	var err error
	config.BuildDir, err = buildDirWithHomeTemplate("buildDir", configFile.BuildTool.BuildDirTemplate)
	if err != nil {
		return nil, err
	}
	config.downloadDir, err = buildDirWithHomeTemplate("downloadDir", configFile.BuildTool.DownloadDirTemplate)
	if err != nil {
		return nil, err
	}
	config.downloadUserAgent = configFile.BuildTool.DownloadUserAgent
	config.toolChainDir, err = buildDirWithHomeTemplate("toolChainDir", configFile.BuildTool.ToolChainDirTemplate)
	if err != nil {
		return nil, err
	}
	var toolchainToml *ToolchainTomlTopLevel
	toolchainToml, err = ReadToolchainToml(config.toolChainDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		toolchainToml = new(ToolchainTomlTopLevel)
	}
	if configFile.BuildTool.Go.Executable != "" {
		config.goExecutable, err = filepath.Abs(configFile.BuildTool.Go.Executable)
		if err != nil {
			return nil, err
		}
	} else if toolchainToml.Go.Executable != "" {
		config.goExecutable, err = filepath.Abs(toolchainToml.Go.Executable)
		if err != nil {
			return nil, err
		}
	}
	env := envMap()
	goPath, envHasGoPath := env["GOPATH"]
	if envHasGoPath {
		config.goPath = goPath
		if configFile.BuildTool.Go.GoPathTemplate != "" {
			log.Print("GOPATH environment variable set and [build-tool.go].GoPathTemplate set in config.toml")
			log.Print("Please set only one of:")
			log.Print(" - GOPATH environment variable")
			log.Print(" - [build-tool.go].GoPathTemplate in config.toml")
			panic("GOPATH overload")
		}
	} else if configFile.BuildTool.Go.GoPathTemplate != "" {
		config.goPath, err = buildDirWithToolChainDirTemplate("goPath", configFile.BuildTool.Go.GoPathTemplate, config.toolChainDir)
		if err != nil {
			return nil, err
		}
	} else {
		config.goPath, err = filepath.Abs(filepath.Join(config.toolChainDir, "gopath"))
		if err != nil {
			return nil, err
		}
	}
	config.bison = new(runtimeConfigBison)
	err = populateBisonRuntimeConfig(config.bison, &configFile.BuildTool.Bison, &downloadsFile.Bison, toolchainToml)
	if err != nil {
		return nil, err
	}
	config.bpfAsm = new(runtimeConfigBpfAsm)
	err = populateBpfAsmRuntimeConfig(config.bpfAsm, &configFile.BuildTool.BpfAsm, toolchainToml, &configFile.BuildTool.Linux, &downloadsFile.Linux)
	if err != nil {
		return nil, err
	}
	config.capnProto = new(runtimeConfigCapnProto)
	err = populateCapnProtoRuntimeConfig(config.capnProto, &configFile.BuildTool.CapnProto, &downloadsFile.CapnProto, toolchainToml)
	if err != nil {
		return nil, err
	}
	config.flex = new(runtimeConfigFlex)
	err = populateFlexRuntimeConfig(config.flex, &configFile.BuildTool.Flex, &downloadsFile.Flex, toolchainToml)
	if err != nil {
		return nil, err
	}
	config.goCapnp = new(runtimeConfigGoCapnp)
	err = populateGoCapnpRuntimeConfig(config.goCapnp, &configFile.BuildTool.GoCapnp, &downloadsFile.GoCapnp, toolchainToml)
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

func buildDirWithHomeTemplate(templateName string, dirTemplate string) (string, error) {
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

func buildDirWithToolChainDirTemplate(templateName string, dirTemplate string, toolChainDir string) (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	goVersion := runtime.Version()
	homeDirectory := user.HomeDir
	values := configGoPathTemplateValues{
		goVersion,
		homeDirectory,
		toolChainDir,
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
		runtimeConfig.executable = ""
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if toolchainToml.Bison == nil {
		runtimeConfig.toolchainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainExecutable = toolchainToml.Bison.Executable
		runtimeConfig.toolchainVersion = toolchainToml.Bison.Version
	}
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

func populateBpfAsmRuntimeConfig(runtimeConfig *runtimeConfigBpfAsm, configFile *ConfigTomlBpfAsm, toolchainToml *ToolchainTomlTopLevel, configFileLinux *ConfigTomlLinux, downloadsFileLinux *DownloadsTomlLinux) error {
	// Version
	if configFileLinux.Version != "" {
		runtimeConfig.version = configFileLinux.Version
	} else {
		runtimeConfig.version = downloadsFileLinux.PreferredVersion
	}
	if runtimeConfig.version == "" {
		return fmt.Errorf("bpf_asm is unable to get Linux's configured version")
	}
	// Executable
	if configFile.Executable != "" {
		runtimeConfig.executable = configFile.Executable
	} else if toolchainToml.BpfAsm != nil && toolchainToml.BpfAsm.Executable != "" && toolchainToml.BpfAsm.Version == runtimeConfig.version {
		absExecutable, err := filepath.Abs(toolchainToml.BpfAsm.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.executable = absExecutable
	} else {
		runtimeConfig.executable = ""
	}
	if toolchainToml.BpfAsm == nil {
		runtimeConfig.toolchainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainExecutable = toolchainToml.BpfAsm.Executable
		runtimeConfig.toolchainVersion = toolchainToml.BpfAsm.Version
	}
	return nil
}

func populateCapnProtoRuntimeConfig(runtimeConfig *runtimeConfigCapnProto, configFile *ConfigTomlCapnProto, downloadsFile *DownloadsTomlCapnProto, toolchainToml *ToolchainTomlTopLevel) error {
	if configFile.DownloadUrl != "" {
		runtimeConfig.downloadUrlTemplate = configFile.DownloadUrl
	} else {
		runtimeConfig.downloadUrlTemplate = downloadsFile.DownloadUrlTemplate
	}
	if configFile.Executable != "" {
		runtimeConfig.executable = configFile.Executable
	} else if toolchainToml.CapnProto != nil && toolchainToml.CapnProto.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.CapnProto.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.executable = absExecutable
	} else {
		runtimeConfig.executable = ""
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if toolchainToml.CapnProto == nil {
		runtimeConfig.toolchainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainExecutable = toolchainToml.CapnProto.Executable
		runtimeConfig.toolchainVersion = toolchainToml.CapnProto.Version
	}
	if configFile.Version != "" {
		runtimeConfig.version = configFile.Version
	} else {
		runtimeConfig.version = downloadsFile.PreferredVersion
	}
	if runtimeConfig.version == "" {
		return fmt.Errorf("Cap'n Proto has no configured version")
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
		runtimeConfig.executable = ""
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if toolchainToml.Flex == nil {
		runtimeConfig.toolchainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainExecutable = toolchainToml.Flex.Executable
		runtimeConfig.toolchainVersion = toolchainToml.Flex.Version
	}
	if configFile.Version != "" {
		runtimeConfig.version = configFile.Version
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

func populateGoCapnpRuntimeConfig(runtimeConfig *runtimeConfigGoCapnp, configFile *ConfigTomlGoCapnp, downloadsFile *DownloadsTomlGoCapnp, toolchainToml *ToolchainTomlTopLevel) error {
	if configFile.DownloadUrl != "" {
		runtimeConfig.downloadUrlTemplate = configFile.DownloadUrl
	} else {
		runtimeConfig.downloadUrlTemplate = downloadsFile.DownloadUrlTemplate
	}
	if configFile.Executable != "" {
		runtimeConfig.executable = configFile.Executable
	} else if toolchainToml.GoCapnp != nil && toolchainToml.GoCapnp.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.GoCapnp.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.executable = absExecutable
	} else {
		runtimeConfig.executable = ""
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if toolchainToml.GoCapnp == nil {
		runtimeConfig.toolchainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainExecutable = toolchainToml.GoCapnp.Executable
		runtimeConfig.toolchainVersion = toolchainToml.GoCapnp.Version
	}
	if configFile.Version != "" {
		runtimeConfig.version = configFile.Version
		runtimeConfig.version = configFile.Version
	} else {
		runtimeConfig.version = downloadsFile.PreferredVersion
	}
	if runtimeConfig.version == "" {
		return fmt.Errorf("go-capnp has no configured version")
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
		runtimeConfig.executable = ""
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if toolchainToml.TinyGo == nil {
		runtimeConfig.toolchainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainExecutable = toolchainToml.TinyGo.Executable
		runtimeConfig.toolchainVersion = toolchainToml.TinyGo.Version
	}
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
