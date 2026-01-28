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

type binaryenConfig struct {
	downloadFile        string
	downloadUrl         string
	executable          string
	expectedFileSize    int64
	expectedSha256      string
	toolchainDir        string
	toolchainExecutable string
	toolchainVersion    string
	version             string
	versionedDir        string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type binaryenDownloadUrlTemplateValues struct {
	Filename string
	Version  string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type binaryenFilenameTemplateValues struct {
	Arch    string
	OS      string
	Version string
}

// getBinaryenArch maps Go's GOARCH to binaryen's architecture naming
func getBinaryenArch() string {
	switch runtime.GOARCH {
	case "arm64":
		if runtime.GOOS == "darwin" {
			return "arm64"
		}
		return "aarch64"
	case "amd64":
		return "x86_64"
	default:
		return runtime.GOARCH
	}
}

// getBinaryenOS maps Go's GOOS to binaryen's OS naming
func getBinaryenOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	default:
		return runtime.GOOS
	}
}

func BootstrapBinaryen(buildToolConfig *RuntimeConfigBuildTool) ([]string, error) {
	messages := make([]string, 0, 5)
	binaryenConfig, err := getBinaryenConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get Binaryen configuration")
		return messages, err
	}
	if binaryenConfig.executable != "" {
		executableExists, err := fileExistsAtPath(binaryenConfig.executable)
		if err != nil {
			log.Printf("fileExistsAtPath err\n")
			return messages, err
		}
		if executableExists {
			messages = append(messages, fmt.Sprintf("Skipping download and installation of Binaryen because %s (from config.toml) exists", binaryenConfig.executable))
			return messages, nil
		} else {
			err = fmt.Errorf("User-specified Binaryen executable %s does not exist.", binaryenConfig.executable)
			return messages, err
		}
	}
	if binaryenConfig.toolchainExecutable != "" {
		executableExists, err := fileExistsAtPath(binaryenConfig.toolchainExecutable)
		if err != nil {
			log.Printf("fileExistsAtPath err\n")
			return messages, err
		}
		if executableExists {
			if binaryenConfig.version == binaryenConfig.toolchainVersion {
				messages = append(messages, fmt.Sprintf("Skipping download and installation of Binaryen because %s (toolchain) exists", binaryenConfig.toolchainExecutable))
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
	downloadPath := filepath.Join(buildToolConfig.Directories.DownloadDir, binaryenConfig.downloadFile)
	downloadPathExists, err := fileExistsAtPath(downloadPath)
	if err != nil {
		return messages, err
	}
	if downloadPathExists {
		messages = append(messages, fmt.Sprintf("Skipping Binaryen download because %s exists", downloadPath))
	} else {
		err := downloadUrlToDir(binaryenConfig.downloadUrl, buildToolConfig.Directories.DownloadDir, downloadPath)
		if err != nil {
			return messages, err
		}
	}
	err = verifyFileSize(binaryenConfig.expectedFileSize, downloadPath)
	if err != nil {
		return messages, err
	}
	err = verifySha256(binaryenConfig.expectedSha256, downloadPath)
	if err != nil {
		return messages, err
	}
	messages = append(messages, fmt.Sprintf("%s has the correct SHA-256", downloadPath))
	executableExists, err := fileExistsAtPath(binaryenConfig.toolchainExecutable)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if executableExists {
		messages = append(messages, fmt.Sprintf("Refusing to install Binaryen because %s exists", binaryenConfig.toolchainExecutable))
	} else {
		transformBinaryenTarGz := transformBinaryenTarGzFactory(binaryenConfig.toolchainDir, binaryenConfig.versionedDir)
		err = extractTarGz(downloadPath, filterBinaryenTarGz(binaryenConfig.versionedDir), transformBinaryenTarGz)
	}
	binaryenConfig.executable = filepath.Join(binaryenConfig.toolchainDir, "bin", "wasm-opt")
	// Update the modified time of the Binaryen executable.
	executableExists, err = fileExistsAtPath(binaryenConfig.executable)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if executableExists {
		err = setFileModifiedTimeToNow(binaryenConfig.executable)
	}
	if err != nil {
		return messages, err
	}
	toolchainTomlExecutable := filepath.Join(binaryenConfig.versionedDir, "bin", "wasm-opt")
	err = updateBinaryenToolchainToml(buildToolConfig.Directories.ToolChainDir, toolchainTomlExecutable, binaryenConfig.version)
	return messages, err
}

func filterBinaryenTarGz(versionedDir string) func(string) bool {
	prefix := versionedDir + "/"
	return func(filePath string) bool {
		acceptable := strings.HasPrefix(filePath, prefix)
		if !acceptable {
			log.Printf("Rejecting file with invalid prefix: %s\n", filePath)
		}
		return acceptable
	}
}

func getBinaryenConfig(buildToolConfig *RuntimeConfigBuildTool) (*binaryenConfig, error) {
	if buildToolConfig.Directories == nil {
		return nil, fmt.Errorf("buildToolConfig.Directories is nil")
	}
	if buildToolConfig.Binaryen == nil {
		return nil, fmt.Errorf("buildToolConfig.Binaryen is nil")
	}
	// Version
	version := buildToolConfig.Binaryen.version
	// Download File
	filenameValues := binaryenFilenameTemplateValues{
		Arch:    getBinaryenArch(),
		OS:      getBinaryenOS(),
		Version: version,
	}
	filenameTemplate, err := template.New("filename").Parse(buildToolConfig.Binaryen.filenameTemplate)
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
	downloadUrlValues := binaryenDownloadUrlTemplateValues{
		downloadFile,
		version,
	}
	downloadUrlTemplate, err := template.New("downloadUrl").Parse(buildToolConfig.Binaryen.downloadUrlTemplate)
	if err != nil {
		return nil, err
	}
	var downloadUrlBuffer bytes.Buffer
	err = downloadUrlTemplate.Execute(&downloadUrlBuffer, downloadUrlValues)
	if err != nil {
		return nil, err
	}
	downloadUrl := downloadUrlBuffer.String()
	downloadFileInfo := buildToolConfig.Binaryen.files[downloadFile]
	if downloadFileInfo == (runtimeConfigFile{}) {
		return nil, fmt.Errorf("File size and SHA-256 not found in downloads.toml for %s", downloadFile)
	}
	// Expected file size and SHA-256
	expectedFileSize := downloadFileInfo.size
	expectedSha256 := downloadFileInfo.sha256
	// Binaryen executable
	executable := buildToolConfig.Binaryen.Executable
	// Toolchain directory
	toolchainDir := buildToolConfig.Binaryen.toolchainDir
	// Toolchain executable
	toolchainExecutable := buildToolConfig.Binaryen.ToolChainExecutable
	// Toolchain version
	toolchainVersion := buildToolConfig.Binaryen.toolchainVersion
	// Versioned directory
	versionedDir := buildToolConfig.Binaryen.versionedDir

	binaryenConfig := new(binaryenConfig)
	binaryenConfig.downloadFile = downloadFile
	binaryenConfig.downloadUrl = downloadUrl
	binaryenConfig.executable = executable
	binaryenConfig.expectedFileSize = expectedFileSize
	binaryenConfig.expectedSha256 = expectedSha256
	binaryenConfig.toolchainDir = toolchainDir
	binaryenConfig.toolchainVersion = toolchainVersion
	binaryenConfig.toolchainExecutable = toolchainExecutable
	binaryenConfig.version = version
	binaryenConfig.versionedDir = versionedDir
	return binaryenConfig, nil
}

func transformBinaryenTarGz(destinationDir string, versionedDir string, filePath string) string {
	// Strip the versioned directory prefix (e.g., "binaryen-version_125/")
	prefix := versionedDir + "/"
	return filepath.Join(destinationDir, strings.TrimPrefix(filePath, prefix))
}

func transformBinaryenTarGzFactory(destinationDir string, versionedDir string) fileTransformer {
	destinationDir = ensureTrailingSlash(destinationDir)
	return func(filePath string) string {
		return transformBinaryenTarGz(destinationDir, versionedDir, filePath)
	}
}

func updateBinaryenToolchainToml(toolchainDir string, executable string, version string) error {
	toolchainTomlTopLevel, err := ReadToolchainToml(toolchainDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		toolchainTomlTopLevel = new(ToolchainTomlTopLevel)
	}
	if toolchainTomlTopLevel.Binaryen == nil {
		toolchainTomlTopLevel.Binaryen = new(ToolchainTomlTool)
	}
	toolchainTomlTopLevel.Binaryen.Executable = executable
	toolchainTomlTopLevel.Binaryen.Version = version
	return WriteToolchainToml(toolchainDir, toolchainTomlTopLevel)
}
