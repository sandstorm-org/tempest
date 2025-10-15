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
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

type goCapnpConfig struct {
	downloadFile        string
	downloadUrl         string
	executable          string
	expectedFileSize    int64
	expectedSha256      string
	goExecutable        string
	goPath              string
	tarGzDir            string
	toolchainDir        string
	toolchainExecutable string
	toolchainVersion    string
	version             string
	versionedDir        string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type goCapnpDownloadUrlTemplateValues struct {
	Filename string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type goCapnpFilenameTemplateValues struct {
	Version string
}

func BootstrapGoCapnp(buildToolConfig *RuntimeConfigBuildTool) ([]string, error) {
	messages := make([]string, 0, 5)
	goCapnpConfig, err := getGoCapnpConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get go-capnp configuration")
		return messages, err
	}
	if goCapnpConfig.executable != "" {
		executableExists, err := fileExistsAtPath(goCapnpConfig.executable)
		if err != nil {
			log.Printf("fileExistsAtPath err\n")
			return messages, err
		}
		if executableExists {
			messages = append(messages, fmt.Sprintf("Skipping download and installation of go-capnp because %s (from config.toml) exists", goCapnpConfig.executable))
			return messages, nil
		} else {
			err = fmt.Errorf("User-specified go-capnp executable %s does not exist.", goCapnpConfig.executable)
			return messages, err
		}
	}
	if goCapnpConfig.toolchainExecutable != "" {
		executableExists, err := fileExistsAtPath(goCapnpConfig.toolchainExecutable)
		if err != nil {
			log.Printf("fileExistsAtPath err\n")
			return messages, err
		}
		if executableExists {
			if goCapnpConfig.version == goCapnpConfig.toolchainVersion {
				messages = append(messages, fmt.Sprintf("Skipping download and installation of go-capnp because %s (from toolchain) exists", goCapnpConfig.executable))
				return messages, nil
			} else {
				messages = append(messages, fmt.Sprintf("The toolchain executable does not match the desired version.  Continuing."))
			}
		}
	}
	err = ensureDownloadDirExists(buildToolConfig.Directories.DownloadDir)
	if err != nil {
		return messages, err
	}
	downloadPath := filepath.Join(buildToolConfig.Directories.DownloadDir, goCapnpConfig.downloadFile)
	downloadPathExists, err := fileExistsAtPath(downloadPath)
	if err != nil {
		return messages, err
	}
	if downloadPathExists {
		messages = append(messages, fmt.Sprintf("Skipping go-capnp download because %s exists", downloadPath))
	} else {
		err := downloadUrlToDir(goCapnpConfig.downloadUrl, buildToolConfig.Directories.DownloadDir, downloadPath)
		if err != nil {
			return messages, err
		}
	}
	err = verifyFileSize(goCapnpConfig.expectedFileSize, downloadPath)
	if err != nil {
		return messages, err
	}
	err = verifySha256(goCapnpConfig.expectedSha256, downloadPath)
	if err != nil {
		return messages, err
	}
	messages = append(messages, fmt.Sprintf("%s has the correct SHA-256", downloadPath))
	filterGoCapnpTarGz := filterGoCapnpTarGzFactory(goCapnpConfig.tarGzDir)
	transformGoCapnpTarGz := transformGoCapnpTarGzFactory(goCapnpConfig.toolchainDir, len(goCapnpConfig.tarGzDir))
	err = extractTarGz(downloadPath, filterGoCapnpTarGz, transformGoCapnpTarGz)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Failed to extract %s", downloadPath))
		return messages, err
	}
	capnpcGoDir := filepath.Join(goCapnpConfig.toolchainDir, "capnpc-go")
	err = buildCapnpcGo(goCapnpConfig, capnpcGoDir)
	if err != nil {
		messages = append(messages, "Failed while running go build for capnpc-go")
		return messages, err
	}
	toolchainTomlExecutable := filepath.Join(goCapnpConfig.versionedDir, "capnpc-go", "capnpc-go")
	err = updateGoCapnpToolchainToml(buildToolConfig.Directories.ToolChainDir, toolchainTomlExecutable, goCapnpConfig.version)
	return messages, err
}

func buildCapnpcGo(config *goCapnpConfig, buildDir string) error {
	cmd := exec.Command(config.goExecutable)
	cmd.Args = append(cmd.Args, "build")
	cmd.Dir = buildDir
	cmd.Env = append(cmd.Env, os.Environ()...)
	for _, envLine := range os.Environ() {
		if i := strings.Index(envLine, "="); i > 0 {
			if envLine[:i] != "GOPATH" {
				cmd.Env = append(cmd.Env, envLine)
			}
		}
	}
	cmd.Env = append(cmd.Env, "GOPATH="+config.goPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func filterGoCapnpTarGz(tarGzDir string, filePath string) bool {
	acceptable := strings.HasPrefix(filePath, tarGzDir)
	if !acceptable {
		// TODO: Figure out how to get this in the messages slice.
		log.Printf("Rejecting file with invalid prefix: %s\n", filePath)
	}
	return acceptable
}

func filterGoCapnpTarGzFactory(tarGzDir string) fileFilter {
	tarGzDir = ensureTrailingSlash(tarGzDir)
	return func(filePath string) bool {
		return filterGoCapnpTarGz(tarGzDir, filePath)
	}
}

func getGoCapnpConfig(buildToolConfig *RuntimeConfigBuildTool) (*goCapnpConfig, error) {
	if buildToolConfig.Directories == nil {
		return nil, fmt.Errorf("buildToolConfig.Directories is nil")
	}
	if buildToolConfig.GoCapnp == nil {
		return nil, fmt.Errorf("buildToolConfig.GoCapnp is nil")
	}
	// Version
	version := buildToolConfig.GoCapnp.version
	// Download File
	filenameValues := goCapnpFilenameTemplateValues{
		version,
	}
	filenameTemplate, err := template.New("filename").Parse(buildToolConfig.GoCapnp.filenameTemplate)
	if err != nil {
		return nil, err
	}
	var filenameBuffer bytes.Buffer
	err = filenameTemplate.Execute(&filenameBuffer, filenameValues)
	if err != nil {
		return nil, err
	}
	downloadFile := filenameBuffer.String()

	// Download URL
	downloadUrlValues := goCapnpDownloadUrlTemplateValues{
		downloadFile,
	}
	downloadUrlTemplate, err := template.New("downloadUrl").Parse(buildToolConfig.GoCapnp.downloadUrlTemplate)
	if err != nil {
		return nil, err
	}
	var downloadUrlBuffer bytes.Buffer
	err = downloadUrlTemplate.Execute(&downloadUrlBuffer, downloadUrlValues)
	if err != nil {
		return nil, err
	}
	downloadUrl := downloadUrlBuffer.String()
	downloadFileInfo := buildToolConfig.GoCapnp.files[downloadFile]
	if downloadFileInfo == (runtimeConfigFile{}) {
		return nil, fmt.Errorf("File size and SHA-256 not found in downloads.toml for %s", downloadFile)
	}
	// Expected file size and SHA-256
	expectedFileSize := downloadFileInfo.size
	expectedSha256 := downloadFileInfo.sha256
	// TarGz directory
	tarGzDir := "go-capnp-" + version
	// capnp executable
	executable := buildToolConfig.GoCapnp.Executable
	// Toolchain directory
	toolchainDir := buildToolConfig.GoCapnp.toolchainDir
	// Toolchain executable
	toolchainExecutable := buildToolConfig.GoCapnp.ToolChainExecutable
	// Toolchain version
	toolchainVersion := buildToolConfig.GoCapnp.toolchainVersion
	// Versioned directory
	versionedDir := buildToolConfig.GoCapnp.versionedDir

	goCapnpConfig := new(goCapnpConfig)
	goCapnpConfig.downloadFile = downloadFile
	goCapnpConfig.downloadUrl = downloadUrl
	goCapnpConfig.executable = executable
	goCapnpConfig.expectedFileSize = expectedFileSize
	goCapnpConfig.expectedSha256 = expectedSha256
	goCapnpConfig.goExecutable = buildToolConfig.Executables.goExecutable
	goCapnpConfig.goPath = buildToolConfig.Executables.goPath
	goCapnpConfig.tarGzDir = tarGzDir
	goCapnpConfig.toolchainDir = toolchainDir
	goCapnpConfig.toolchainExecutable = toolchainExecutable
	goCapnpConfig.toolchainVersion = toolchainVersion
	goCapnpConfig.version = version
	goCapnpConfig.versionedDir = versionedDir
	return goCapnpConfig, nil
}

func transformGoCapnpTarGz(toolchainDir string, filePath string, prefixLength int) string {
	maxLength := min(len(filePath), prefixLength)
	return filepath.Join(toolchainDir, filePath[maxLength:])
}

func transformGoCapnpTarGzFactory(destinationDir string, prefixLength int) fileTransformer {
	destinationDir = ensureTrailingSlash(destinationDir)
	return func(filePath string) string {
		return transformGoCapnpTarGz(destinationDir, filePath, prefixLength)
	}
}

func updateGoCapnpToolchainToml(toolchainDir string, executable string, version string) error {
	toolchainTomlTopLevel, err := ReadToolchainToml(toolchainDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		toolchainTomlTopLevel = new(ToolchainTomlTopLevel)
	}
	if toolchainTomlTopLevel.GoCapnp == nil {
		toolchainTomlTopLevel.GoCapnp = new(ToolchainTomlTool)
	}
	toolchainTomlTopLevel.GoCapnp.Executable = executable
	toolchainTomlTopLevel.GoCapnp.Version = version
	return WriteToolchainToml(toolchainDir, toolchainTomlTopLevel)
}
