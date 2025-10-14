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
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

type tinyGoConfig struct {
	downloadFile        string
	downloadUrl         string
	executable          string
	expectedFileSize    int64
	expectedSha256      string
	toolchainDir        string
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type tinyGoDownloadUrlTemplateValues struct {
	Filename string
	Version  string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type tinyGoFilenameTemplateValues struct {
	Arch    string
	Version string
}

func BootstrapTinyGo(buildToolConfig *RuntimeConfigBuildTool) ([]string, error) {
	messages := make([]string, 0, 5)
	tinyGoConfig, err := getTinyGoConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get TinyGo configuration")
		return messages, err
	}
	exists, err := fileExistsAtPath(tinyGoConfig.executable)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if tinyGoConfig.executable == tinyGoConfig.toolchainExecutable {
		if tinyGoConfig.version == tinyGoConfig.toolchainVersion && exists {
			messages = append(messages, fmt.Sprintf("Skipping download and installation of TinyGo because %s exists", tinyGoConfig.executable))
			return messages, err
		}
	} else if tinyGoConfig.executable != "" {
		if exists {
			messages = append(messages, fmt.Sprintf("Skipping download and installation of TinyGo because %s exists", tinyGoConfig.executable))
			return messages, err
		} else {
			err = fmt.Errorf("Specified TinyGo executable %s is outside the toolchain and does not exist.")
			return messages, err
		}
	}
	err = ensureDownloadDirExists(buildToolConfig.Directories.DownloadDir)
	if err != nil {
		return messages, err
	}
	downloadPath := filepath.Join(buildToolConfig.Directories.DownloadDir, tinyGoConfig.downloadFile)
	exists, err = fileExistsAtPath(downloadPath)
	if err != nil {
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Skipping TinyGo download because %s exists", downloadPath))
	} else {
		err := downloadUrlToDir(tinyGoConfig.downloadUrl, buildToolConfig.Directories.DownloadDir, downloadPath)
		if err != nil {
			return messages, err
		}
	}
	err = verifyFileSize(tinyGoConfig.expectedFileSize, downloadPath)
	if err != nil {
		return messages, err
	}
	err = verifySha256(tinyGoConfig.expectedSha256, downloadPath)
	if err != nil {
		return messages, err
	}
	messages = append(messages, fmt.Sprintf("%s has the correct SHA-256", downloadPath))
	exists, err = fileExistsAtPath(tinyGoConfig.executable)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Refusing to install TinyGo because %s exists", tinyGoConfig.executable))
	} else {
		transformTinyGoTarGz := transformTinyGoTarGzFactory(tinyGoConfig.toolchainDir)
		err = extractTarGz(downloadPath, filterTinyGoTarGz, transformTinyGoTarGz)
	}
	tinyGoConfig.executable = filepath.Join(tinyGoConfig.toolchainDir, "bin", "tinygo")
	// Update the modified time of the TinyGo executable.
	// This is a hack to satisfy `make`.
	// The Makefile looks at the `tinygo` executable.  If its modified time
	// is current, then make will not invoke the target.  If its modified
	// time is not updated, then make will extract TinyGo every time the
	// TinyGo target is invoked.
	exists, err = fileExistsAtPath(tinyGoConfig.executable)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if exists {
		err = setFileModifiedTimeToNow(tinyGoConfig.executable)
	}
	if err != nil {
		return messages, err
	}
	err = updateTinyGoToolchainToml(buildToolConfig.Directories.ToolChainDir, tinyGoConfig.executable, tinyGoConfig.version)
	return messages, err
}

func filterTinyGoTarGz(filePath string) bool {
	acceptable := strings.HasPrefix(filePath, "tinygo/")
	if !acceptable {
		// TODO: Figure out how to get this in the messages slice.
		log.Printf("Rejecting file with invalid prefix: %s\n", filePath)
	}
	return acceptable
}

/**
 * getTinyGoConfig populates templates from the runtime configuration with
 * appropriate values.
 */
func getTinyGoConfig(buildToolConfig *RuntimeConfigBuildTool) (*tinyGoConfig, error) {
	if buildToolConfig.Directories == nil {
		return nil, fmt.Errorf("buildToolConfig.Directories is nil")
	}
	if buildToolConfig.TinyGo == nil {
		return nil, fmt.Errorf("buildToolConfig.TinyGo is nil")
	}
	// Version
	version := buildToolConfig.TinyGo.version
	// Download File
	filenameValues := tinyGoFilenameTemplateValues{
		runtime.GOARCH,
		version,
	}
	filenameTemplate, err := template.New("filename").Parse(buildToolConfig.TinyGo.filenameTemplate)
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
	downloadUrlValues := tinyGoDownloadUrlTemplateValues{
		downloadFile,
		version,
	}
	downloadUrlTemplate, err := template.New("downloadUrl").Parse(buildToolConfig.TinyGo.downloadUrlTemplate)
	if err != nil {
		return nil, err
	}
	var downloadUrlBuffer bytes.Buffer
	err = downloadUrlTemplate.Execute(&downloadUrlBuffer, downloadUrlValues)
	if err != nil {
		return nil, err
	}
	downloadUrl := downloadUrlBuffer.String()
	downloadFileInfo := buildToolConfig.TinyGo.files[downloadFile]
	if downloadFileInfo == (runtimeConfigFile{}) {
		return nil, fmt.Errorf("File size and SHA-256 not found in downloads.toml for %s", downloadFile)
	}
	// Expected file size and SHA-256
	expectedFileSize := downloadFileInfo.size
	expectedSha256 := downloadFileInfo.sha256
	// TinyGo executable
	executable := buildToolConfig.TinyGo.Executable
	// Toolchain directory
	tinyGoVersionedDir := "tinygo-" + version
	toolchainDir := filepath.Join(buildToolConfig.Directories.ToolChainDir, tinyGoVersionedDir)
	// Toolchain executable
	toolchainExecutable := buildToolConfig.TinyGo.toolchainExecutable
	// Toolchain version
	toolchainVersion := buildToolConfig.TinyGo.toolchainVersion

	tinyGoConfig := new(tinyGoConfig)
	tinyGoConfig.downloadFile = downloadFile
	tinyGoConfig.downloadUrl = downloadUrl
	tinyGoConfig.executable = executable
	tinyGoConfig.expectedFileSize = expectedFileSize
	tinyGoConfig.expectedSha256 = expectedSha256
	tinyGoConfig.toolchainDir = toolchainDir
	tinyGoConfig.toolchainVersion = toolchainVersion
	tinyGoConfig.toolchainExecutable = toolchainExecutable
	tinyGoConfig.version = version
	return tinyGoConfig, nil
}

func transformTinyGoTarGz(destinationDir string, filePath string) string {
	return filepath.Join(destinationDir, filePath[6:])
}

func transformTinyGoTarGzFactory(destinationDir string) fileTransformer {
	destinationDir = ensureTrailingSlash(destinationDir)
	return func(filePath string) string {
		return transformTinyGoTarGz(destinationDir, filePath)
	}
}

func updateTinyGoToolchainToml(toolchainDir string, executable string, version string) error {
	toolchainTomlTopLevel, err := ReadToolchainToml(toolchainDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		toolchainTomlTopLevel = new(ToolchainTomlTopLevel)
	}
	if toolchainTomlTopLevel.TinyGo == nil {
		toolchainTomlTopLevel.TinyGo = new(ToolchainTomlTool)
	}
	toolchainTomlTopLevel.TinyGo.Executable = executable
	toolchainTomlTopLevel.TinyGo.Version = version
	return WriteToolchainToml(toolchainDir, toolchainTomlTopLevel)
}
