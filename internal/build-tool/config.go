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

	Bison     ConfigTomlTool     `toml:"bison"`
	BpfAsm    ConfigTomlBpfAsm   `toml:"bpf_asm"`
	CapnProto ConfigTomlTool     `toml:"capnproto"`
	Flex      ConfigTomlTool     `toml:"flex"`
	Generate  ConfigTomlGenerate `toml:"generate"`
	Go        ConfigTomlGo       `toml:"go"`
	GoCapnp   ConfigTomlTool     `toml:"go-capnp"`
	Linux     ConfigTomlLinux    `toml:"linux"`
	TinyGo    ConfigTomlTool     `toml:"tinygo"`
}

type ConfigTomlTool struct {
	DownloadUrl string
	Executable  string
	Version     string
}

type ConfigTomlBpfAsm struct {
	Executable string
	GoPath     string
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

type ConfigTomlLinux struct {
	DownloadUrl string
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

	Bison     *runtimeConfigTool
	BpfAsm    *runtimeConfigBpfAsm
	CapnProto *runtimeConfigTool
	Flex      *runtimeConfigTool
	Generate  *runtimeConfigGenerate
	GoCapnp   *runtimeConfigTool
	linux     *runtimeConfigLinux
	TinyGo    *runtimeConfigTool
}

type runtimeConfigTool struct {
	downloadUrlTemplate string // from config.toml or downloads.toml
	Executable          string // from config.toml or empty
	filenameTemplate    string // from downloads.toml
	files               map[string]runtimeConfigFile // from downloads.toml
	Name                string // Tool name, suitable for display, e.g., "Bison"
	Prefix              string // Tool prefix, e.g., "bison-"
	// NB!
	// toolchainDir is the directory that might exist in toolchain, and is
	// formed by combining the tool's prefix with the desired version.
	// This means that it may not have the same version as
	// toolchainVersion.
	toolchainDir        string
	ToolChainExecutable string // the Bison executable that might exist in /toolchain
	toolchainVersion    string // the version of Bison that might exist in /toolchain
	version             string // from config.toml or from downloads.toml
	versionedDir        string // e.g., "bison-3.8.2"
}

type runtimeConfigBpfAsm struct {
	Executable          string
	toolchainDir        string
	ToolChainExecutable string
	toolchainVersion    string
	version             string
}

type runtimeConfigCapnProto struct {
	downloadUrlTemplate string
	Executable          string
	filenameTemplate    string
	files               map[string]runtimeConfigFile
	ToolChainExecutable string
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
	ToolChainExecutable string
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

func BuildConfiguration(configFile *ConfigTomlTopLevel, downloadsFile *DownloadsTomlTopLevel) (*RuntimeConfigBuildTool, error) {
	config := new(RuntimeConfigBuildTool)
	var err error
	// Top-level
	config.downloadUserAgent = configFile.BuildTool.DownloadUserAgent
	// Directories
	config.Directories = new(runtimeConfigDirectories)
	buildDir, err := buildDirWithHomeTemplate("BuildDir", configFile.BuildTool.BuildDirTemplate)
	if err != nil {
		return nil, err
	}
	config.Directories.BuildDir, err = filepath.Abs(buildDir)
	if err != nil {
		return nil, err
	}
	downloadDir, err := buildDirWithHomeTemplate("DownloadDir", configFile.BuildTool.DownloadDirTemplate)
	if err != nil {
		return nil, err
	}
	config.Directories.DownloadDir, err = filepath.Abs(downloadDir)
	if err != nil {
		return nil, err
	}
	config.Directories.IncrementalDir = filepath.Join(config.Directories.BuildDir, "incremental")
	toolChainDir, err := buildDirWithHomeTemplate("ToolChainDir", configFile.BuildTool.ToolChainDirTemplate)
	if err != nil {
		return nil, err
	}
	config.Directories.ToolChainDir, err = filepath.Abs(toolChainDir)
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
	config.Bison = new(runtimeConfigTool)
	config.Bison.Name = "Bison"
	config.Bison.Prefix = "bison-"
	err = populateToolRuntimeConfig(config.Bison, config.Directories, &configFile.BuildTool.Bison, &downloadsFile.Bison, toolchainToml.Bison)
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
	config.CapnProto = new(runtimeConfigTool)
	config.CapnProto.Name = "Cap'n Proto"
	config.CapnProto.Prefix = "capnp-"
	err = populateToolRuntimeConfig(config.CapnProto, config.Directories, &configFile.BuildTool.CapnProto, &downloadsFile.CapnProto, toolchainToml.CapnProto)
	if err != nil {
		return nil, err
	}
	// Flex
	config.Flex = new(runtimeConfigTool)
	config.Flex.Name = "Flex"
	config.Flex.Prefix = "flex-"
	err = populateToolRuntimeConfig(config.Flex, config.Directories, &configFile.BuildTool.Flex, &downloadsFile.Flex, toolchainToml.Flex)
	if err != nil {
		return nil, err
	}
	// go-capnp
	// config.Generate.Capnp needs values from config.GoCapnp, so this
	// comes first.
	config.GoCapnp = new(runtimeConfigTool)
	config.GoCapnp.Name = "go-capnp"
	config.GoCapnp.Prefix = "go-capnp-"
	err = populateToolRuntimeConfig(config.GoCapnp, config.Directories, &configFile.BuildTool.GoCapnp, &downloadsFile.GoCapnp, toolchainToml.GoCapnp)
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
	config.TinyGo = new(runtimeConfigTool)
	config.TinyGo.Name = "TinyGo"
	config.TinyGo.Prefix = "tinygo-"
	err = populateToolRuntimeConfig(config.TinyGo, config.Directories, &configFile.BuildTool.TinyGo, &downloadsFile.TinyGo, toolchainToml.TinyGo)
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

func populateToolRuntimeConfig(runtimeConfig *runtimeConfigTool, directories *runtimeConfigDirectories, configFile *ConfigTomlTool, downloadsFile *DownloadsTomlTool, toolChainTool *ToolchainTomlTool) error {
	// First, get the version.
	if configFile.Version != "" {
		runtimeConfig.version = configFile.Version
	} else {
		runtimeConfig.version = downloadsFile.PreferredVersion
	}
	if runtimeConfig.version == "" {
		return fmt.Errorf("%s has no configured version", runtimeConfig.Name)
	}
	// With that, we can figured out the versionedDir
	runtimeConfig.versionedDir = runtimeConfig.Prefix + runtimeConfig.version

	if configFile.DownloadUrl != "" {
		runtimeConfig.downloadUrlTemplate = configFile.DownloadUrl
	} else {
		runtimeConfig.downloadUrlTemplate = downloadsFile.DownloadUrlTemplate
	}

	if configFile.Executable != "" {
		runtimeConfig.Executable = configFile.Executable
	} else {
		// There is no executable
		runtimeConfig.Executable = ""
	}
	runtimeConfig.filenameTemplate = downloadsFile.FilenameTemplate
	runtimeConfig.files = make(map[string]runtimeConfigFile)
	for fileName, fileStruct := range downloadsFile.Files {
		runtimeConfig.files[fileName] = runtimeConfigFile{
			fileStruct.Sha256,
			fileStruct.Size,
		}
	}
	runtimeConfig.toolchainDir = filepath.Join(directories.ToolChainDir, runtimeConfig.versionedDir)
	if toolChainTool == nil {
		runtimeConfig.ToolChainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainVersion = toolChainTool.Version
		runtimeConfig.ToolChainExecutable = filepath.Join(directories.ToolChainDir, toolChainTool.Executable)
	}

	return nil
}

func populateBpfAsmRuntimeConfig(runtimeConfig *runtimeConfigBpfAsm, directories *runtimeConfigDirectories, configFile *ConfigTomlBpfAsm, toolchainToml *ToolchainTomlTopLevel, configFileLinux *ConfigTomlLinux, downloadsFileLinux *DownloadsTomlTool) error {
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
		runtimeConfig.Executable = filepath.Join(directories.ToolChainDir, toolchainToml.BpfAsm.Executable)
	} else {
		runtimeConfig.Executable = ""
	}
	prefix := "bpf_asm-"
	if toolchainToml.BpfAsm == nil {
		runtimeConfig.toolchainDir = ""
		runtimeConfig.ToolChainExecutable = ""
		runtimeConfig.toolchainVersion = ""
	} else {
		runtimeConfig.toolchainVersion = toolchainToml.BpfAsm.Version
		versionedDir := prefix + runtimeConfig.toolchainVersion
		runtimeConfig.toolchainDir = filepath.Join(directories.ToolChainDir, versionedDir)
		runtimeConfig.ToolChainExecutable = filepath.Join(directories.ToolChainDir, toolchainToml.BpfAsm.Executable)
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
	} else if toolchainToml.Go != nil && toolchainToml.Go.Executable != "" {
		config.Executables.goExecutable = filepath.Join(config.Directories.ToolChainDir, toolchainToml.Go.Executable)
	}
	goPath, err := getGoPath(config, configFile)
	if err != nil {
		return err
	}
	config.Executables.goPath = goPath
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
	runtimeConfig.StdDir = stdDir
	return nil
}

func populateLinuxRuntimeConfig(runtimeConfig *runtimeConfigLinux, configFile *ConfigTomlLinux, downloadsFile *DownloadsTomlTool) error {
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

func ReadConfigFile(configFilePath *string) (*ConfigTomlTopLevel, error) {
	config := new(ConfigTomlTopLevel)
	_, err := toml.DecodeFile(*configFilePath, config)
	return config, err
}
