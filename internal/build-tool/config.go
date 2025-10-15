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

	Bison     ConfigTomlBison     `toml:"bison"`
	BpfAsm    ConfigTomlBpfAsm    `toml:"bpf_asm"`
	CapnProto ConfigTomlCapnProto `toml:"capnproto"`
	Flex      ConfigTomlFlex      `toml:"flex"`
	Generate  ConfigTomlGenerate  `toml:"generate"`
	Go        ConfigTomlGo        `toml:"go"`
	GoCapnp   ConfigTomlGoCapnp   `toml:"go-capnp"`
	Linux     ConfigTomlLinux     `toml:"linux"`
	TinyGo    ConfigTomlTinyGo    `toml:"tinygo"`
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

type ConfigTomlGenerate struct {
	Capnp ConfigTomlGenerateCapnp `toml:"capnp"`
}

type ConfigTomlGenerateCapnp struct {
	CapnpDirs      []string
	StdDirTemplate string
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

type configStdDirTemplateValues struct {
	GoCapnpVersion string
	Home           string
	ToolChainDir   string
}

// Runtime config types
//
// Runtime configuration is private to the package, so struct names and field
// names should be in camelCase.

type RuntimeConfigBuildTool struct {
	downloadUserAgent string

	Directories *runtimeConfigDirectories
	Executables *runtimeConfigExecutables

	Bison     *runtimeConfigBison
	BpfAsm    *runtimeConfigBpfAsm
	CapnProto *runtimeConfigCapnProto
	Flex      *runtimeConfigFlex
	Generate  *runtimeConfigGenerate
	GoCapnp   *runtimeConfigGoCapnp
	linux     *runtimeConfigLinux
	TinyGo    *runtimeConfigTinyGo
}

type runtimeConfigBison struct {
	downloadUrlTemplate string
	Executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	toolchainDir        string
	toolchainExecutable string
	toolchainVersion    string
	version             string
	versionedDir        string
}

type runtimeConfigBpfAsm struct {
	Executable          string
	toolchainDir        string
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

type runtimeConfigCapnProto struct {
	downloadUrlTemplate string
	Executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

type runtimeConfigDirectories struct {
	BuildDir       string
	DownloadDir    string
	IncrementalDir string
	ToolChainDir   string
}

type runtimeConfigExecutables struct {
	goExecutable string
	goPath       string
}

type runtimeConfigFile struct {
	sha256 string
	size   int64
}

type runtimeConfigFlex struct {
	downloadUrlTemplate string
	Executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	toolchainDir        string
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

type runtimeConfigGenerate struct {
	Capnp *runtimeConfigGenerateCapnp
}

type runtimeConfigGenerateCapnp struct {
	CapnpDirs []string
	StdDir    string
}

type runtimeConfigGoCapnp struct {
	downloadUrlTemplate string
	Executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	toolchainDir        string
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
	Executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

func BuildConfiguration(configFile *ConfigTomlTopLevel, downloadsFile *DownloadsTomlTopLevel) (*RuntimeConfigBuildTool, error) {
	config := new(RuntimeConfigBuildTool)
	var err error
	// Top-level
	config.downloadUserAgent = configFile.BuildTool.DownloadUserAgent
	// Directories
	config.Directories = new(runtimeConfigDirectories)
	config.Directories.BuildDir, err = buildDirWithHomeTemplate("BuildDir", configFile.BuildTool.BuildDirTemplate)
	if err != nil {
		return nil, err
	}
	config.Directories.DownloadDir, err = buildDirWithHomeTemplate("DownloadDir", configFile.BuildTool.DownloadDirTemplate)
	if err != nil {
		return nil, err
	}
	config.Directories.IncrementalDir = filepath.Join(config.Directories.BuildDir, "incremental")
	config.Directories.ToolChainDir, err = buildDirWithHomeTemplate("ToolChainDir", configFile.BuildTool.ToolChainDirTemplate)
	if err != nil {
		return nil, err
	}
	var toolchainToml *ToolchainTomlTopLevel
	toolchainToml, err = ReadToolchainToml(config.Directories.ToolChainDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		toolchainToml = new(ToolchainTomlTopLevel)
	}
	config.Executables = new(runtimeConfigExecutables)
	err = populateExecutablesRuntimeConfig(config, configFile, toolchainToml)
	if err != nil {
		return nil, err
	}
	// Bison
	config.Bison = new(runtimeConfigBison)
	err = populateBisonRuntimeConfig(config.Bison, config.Directories, &configFile.BuildTool.Bison, &downloadsFile.Bison, toolchainToml)
	if err != nil {
		return nil, err
	}
	// bpf-asm
	config.BpfAsm = new(runtimeConfigBpfAsm)
	err = populateBpfAsmRuntimeConfig(config.BpfAsm, config.Directories, &configFile.BuildTool.BpfAsm, toolchainToml, &configFile.BuildTool.Linux, &downloadsFile.Linux)
	if err != nil {
		return nil, err
	}
	// Cap'n Proto
	config.CapnProto = new(runtimeConfigCapnProto)
	err = populateCapnProtoRuntimeConfig(config.CapnProto, &configFile.BuildTool.CapnProto, &downloadsFile.CapnProto, toolchainToml)
	if err != nil {
		return nil, err
	}
	// Flex
	config.Flex = new(runtimeConfigFlex)
	err = populateFlexRuntimeConfig(config.Flex, config.Directories, &configFile.BuildTool.Flex, &downloadsFile.Flex, toolchainToml)
	if err != nil {
		return nil, err
	}
	// go-capnp
	// config.Generate.Capnp needs values from config.GoCapnp, so this
	// comes first.
	config.GoCapnp = new(runtimeConfigGoCapnp)
	err = populateBuildGoCapnpRuntimeConfig(config.GoCapnp, config.Directories, &configFile.BuildTool.GoCapnp, &downloadsFile.GoCapnp, toolchainToml)
	if err != nil {
		return nil, err
	}
	// Generate
	config.Generate = new(runtimeConfigGenerate)
	// Generate Cap'n Proto
	config.Generate.Capnp = new(runtimeConfigGenerateCapnp)
	err = populateGenerateCapnpRuntimeConfig(config.Generate.Capnp, config.Directories, &configFile.BuildTool.Generate.Capnp, config.GoCapnp.version)
	if err != nil {
		return nil, err
	}
	// Linux
	config.linux = new(runtimeConfigLinux)
	err = populateLinuxRuntimeConfig(config.linux, &configFile.BuildTool.Linux, &downloadsFile.Linux)
	if err != nil {
		return nil, err
	}
	// TinyGo
	config.TinyGo = new(runtimeConfigTinyGo)
	err = populateTinyGoRuntimeConfig(config.TinyGo, &configFile.BuildTool.TinyGo, &downloadsFile.TinyGo, toolchainToml)
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

func buildStringWithTemplateAndValues(templateName string, templateString string, templateValues any) (string, error) {
	parsedTemplate, err := template.New(templateName).Parse(templateString)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	err = parsedTemplate.Execute(&buffer, templateValues)
	if err != nil {
		return "", err
	}
	result := buffer.String()
	return result, nil
}

func getGoPath(config *RuntimeConfigBuildTool, configFile *ConfigTomlTopLevel) (string, error) {
	env := envMap()
	goPathFromEnv, envHasGoPath := env["GOPATH"]
	if configFile.BuildTool.Go.GoPathTemplate != "" {
		goPathWithTemplate, err := buildDirWithToolChainDirTemplate("goPath", configFile.BuildTool.Go.GoPathTemplate, config.Directories.ToolChainDir)
		if err != nil {
			// If we already have a value from the environment
			// variable, return that.
			if envHasGoPath {
				return goPathFromEnv, nil
			}
			return "", err
		}
		if envHasGoPath && goPathFromEnv != goPathWithTemplate {
			log.Print("GOPATH environment variable and [build-tool.go].GoPathTemplate in config.toml have conflicting values")
			log.Print("Please set only one of:")
			log.Print(" - GOPATH environment variable: " + goPathFromEnv)
			log.Print(" - [build-tool.go].GoPathTemplate: " + goPathWithTemplate)
			return "", fmt.Errorf("Conflicting GOPATH values")
		}
		return goPathWithTemplate, nil
	}
	if envHasGoPath {
		return goPathFromEnv, nil
	}
	// Return a default value
	goPath, err := filepath.Abs(filepath.Join(config.Directories.ToolChainDir, "gopath-"+runtime.Version()))
	if err != nil {
		return "", err
	}
	return goPath, nil
}

func populateBisonRuntimeConfig(runtimeConfig *runtimeConfigBison, directories *runtimeConfigDirectories, configFile *ConfigTomlBison, downloadsFile *DownloadsTomlBison, toolchainToml *ToolchainTomlTopLevel) error {
	if configFile.DownloadUrl != "" {
		runtimeConfig.downloadUrlTemplate = configFile.DownloadUrl
	} else {
		runtimeConfig.downloadUrlTemplate = downloadsFile.DownloadUrlTemplate
	}
	if configFile.Executable != "" {
		runtimeConfig.Executable = configFile.Executable
	} else if toolchainToml.Bison != nil && toolchainToml.Bison.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.Bison.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.Executable = absExecutable
	} else {
		runtimeConfig.Executable = ""
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	prefix := "bison-"
	if toolchainToml.Bison == nil {
		runtimeConfig.toolchainDir = ""
		runtimeConfig.toolchainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainVersion = toolchainToml.Bison.Version
		versionedDir := prefix + runtimeConfig.toolchainVersion
		runtimeConfig.toolchainDir = filepath.Join(directories.ToolChainDir, versionedDir)
		runtimeConfig.toolchainExecutable = toolchainToml.Bison.Executable
	}
	if configFile.Version != "" {
		runtimeConfig.version = configFile.Version
	} else {
		runtimeConfig.version = downloadsFile.PreferredVersion
	}
	if runtimeConfig.version == "" {
		return fmt.Errorf("Bison has no configured version")
	}
	runtimeConfig.versionedDir = prefix + runtimeConfig.version
	runtimeConfig.files = make(map[string]runtimeConfigFile)
	for fileName, fileStruct := range downloadsFile.Files {
		runtimeConfig.files[fileName] = runtimeConfigFile{
			fileStruct.Sha256,
			fileStruct.Size,
		}
	}
	return nil
}

func populateBpfAsmRuntimeConfig(runtimeConfig *runtimeConfigBpfAsm, directories *runtimeConfigDirectories, configFile *ConfigTomlBpfAsm, toolchainToml *ToolchainTomlTopLevel, configFileLinux *ConfigTomlLinux, downloadsFileLinux *DownloadsTomlLinux) error {
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
		runtimeConfig.Executable = configFile.Executable
	} else if toolchainToml.BpfAsm != nil && toolchainToml.BpfAsm.Executable != "" && toolchainToml.BpfAsm.Version == runtimeConfig.version {
		absExecutable, err := filepath.Abs(toolchainToml.BpfAsm.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.Executable = absExecutable
	} else {
		runtimeConfig.Executable = ""
	}
	prefix := "bpf_asm-"
	if toolchainToml.BpfAsm == nil {
		runtimeConfig.toolchainDir = ""
		runtimeConfig.toolchainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainVersion = toolchainToml.BpfAsm.Version
		versionedDir := prefix + runtimeConfig.toolchainVersion
		runtimeConfig.toolchainDir = filepath.Join(directories.ToolChainDir, versionedDir)
		runtimeConfig.toolchainExecutable = toolchainToml.BpfAsm.Executable
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
		runtimeConfig.Executable = configFile.Executable
	} else if toolchainToml.CapnProto != nil && toolchainToml.CapnProto.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.CapnProto.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.Executable = absExecutable
	} else {
		runtimeConfig.Executable = ""
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

func populateExecutablesRuntimeConfig(config *RuntimeConfigBuildTool, configFile *ConfigTomlTopLevel, toolchainToml *ToolchainTomlTopLevel) error {
	var err error
	if configFile.BuildTool.Go.Executable != "" {
		config.Executables.goExecutable, err = filepath.Abs(configFile.BuildTool.Go.Executable)
		if err != nil {
			return err
		}
	} else if toolchainToml.Go.Executable != "" {
		config.Executables.goExecutable, err = filepath.Abs(toolchainToml.Go.Executable)
		if err != nil {
			return err
		}
	}
	goPath, err := getGoPath(config, configFile)
	if err != nil {
		return err
	}
	config.Executables.goPath = goPath
	return nil
}

func populateFlexRuntimeConfig(runtimeConfig *runtimeConfigFlex, directories *runtimeConfigDirectories, configFile *ConfigTomlFlex, downloadsFile *DownloadsTomlFlex, toolchainToml *ToolchainTomlTopLevel) error {
	if configFile.DownloadUrl != "" {
		runtimeConfig.downloadUrlTemplate = configFile.DownloadUrl
	} else {
		runtimeConfig.downloadUrlTemplate = downloadsFile.DownloadUrlTemplate
	}
	if configFile.Executable != "" {
		runtimeConfig.Executable = configFile.Executable
	} else if toolchainToml.Flex != nil && toolchainToml.Flex.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.Flex.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.Executable = absExecutable
	} else {
		runtimeConfig.Executable = ""
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if toolchainToml.Flex == nil {
		runtimeConfig.toolchainDir = ""
		runtimeConfig.toolchainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainDir = filepath.Join(directories.ToolChainDir, "flex-"+toolchainToml.Flex.Version)
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

func populateGenerateCapnpRuntimeConfig(runtimeConfig *runtimeConfigGenerateCapnp, directories *runtimeConfigDirectories, configFile *ConfigTomlGenerateCapnp, goCapnpVersion string) error {
	runtimeConfig.CapnpDirs = configFile.CapnpDirs
	//	incrementalDir :=
	stdDirTemplate := configFile.StdDirTemplate
	if stdDirTemplate == "" {
		stdDirTemplate = "{{ .ToolChainDir }}/go-capnp-{{ .GoCapnpVersion }}/std"
	}
	// Template values
	user, err := user.Current()
	if err != nil {
		return err
	}
	homeDirectory := user.HomeDir
	values := configStdDirTemplateValues{
		goCapnpVersion,
		homeDirectory,
		directories.ToolChainDir,
	}
	stdDir, err := buildStringWithTemplateAndValues("stdDir", stdDirTemplate, values)
	if err != nil {
		return nil
	}
	log.Printf("stdDir: %s", stdDir)
	runtimeConfig.StdDir = stdDir
	return nil
}

func populateBuildGoCapnpRuntimeConfig(runtimeConfig *runtimeConfigGoCapnp, directories *runtimeConfigDirectories, configFile *ConfigTomlGoCapnp, downloadsFile *DownloadsTomlGoCapnp, toolchainToml *ToolchainTomlTopLevel) error {
	if configFile.DownloadUrl != "" {
		runtimeConfig.downloadUrlTemplate = configFile.DownloadUrl
	} else {
		runtimeConfig.downloadUrlTemplate = downloadsFile.DownloadUrlTemplate
	}
	if configFile.Executable != "" {
		runtimeConfig.Executable = configFile.Executable
	} else if toolchainToml.GoCapnp != nil && toolchainToml.GoCapnp.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.GoCapnp.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.Executable = absExecutable
	} else {
		runtimeConfig.Executable = ""
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	if toolchainToml.GoCapnp == nil {
		runtimeConfig.toolchainDir = ""
		runtimeConfig.toolchainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainDir = filepath.Join(directories.ToolChainDir, "capnproto-"+toolchainToml.GoCapnp.Version)
		runtimeConfig.toolchainExecutable = toolchainToml.GoCapnp.Executable
		runtimeConfig.toolchainVersion = toolchainToml.GoCapnp.Version
	}
	if configFile.Version != "" {
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
		runtimeConfig.Executable = configFile.Executable
	} else if toolchainToml.TinyGo != nil && toolchainToml.TinyGo.Executable != "" {
		absExecutable, err := filepath.Abs(toolchainToml.TinyGo.Executable)
		if err != nil {
			return err
		}
		runtimeConfig.Executable = absExecutable
	} else {
		runtimeConfig.Executable = ""
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
