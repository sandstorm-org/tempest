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

type bisonConfig struct {
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
type bisonDownloadUrlTemplateValues struct {
	Filename string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type bisonFilenameTemplateValues struct {
	Version string
}

func BootstrapBison(buildToolConfig *RuntimeConfigBuildTool) ([]string, error) {
	messages := make([]string, 0, 5)
	bisonConfig, err := getBisonConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get Bison configuration")
		return messages, err
	}
	exists, err := fileExistsAtPath(bisonConfig.executable)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if bisonConfig.executable == bisonConfig.toolchainExecutable {
		if bisonConfig.version == bisonConfig.toolchainVersion && exists {
			messages = append(messages, fmt.Sprintf("Skipping download and installation of Bison because %s exists", bisonConfig.executable))
			return messages, err
		}
	} else if bisonConfig.executable != "" {
		if exists {
			messages = append(messages, fmt.Sprintf("Skipping download and installation of Bison because %s exists", bisonConfig.executable))
			return messages, err
		} else {
			err = fmt.Errorf("Specified Bison executable %s is outside the toolchain and does not exist.")
			return messages, err
		}
	}
	err = ensureDownloadDirExists(buildToolConfig.Directories.DownloadDir)
	if err != nil {
		return messages, err
	}
	downloadPath := filepath.Join(buildToolConfig.Directories.DownloadDir, bisonConfig.downloadFile)
	exists, err = fileExistsAtPath(downloadPath)
	if err != nil {
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Skipping Bison download because %s exists", downloadPath))
	} else {
		err := downloadUrlToDir(bisonConfig.downloadUrl, buildToolConfig.Directories.DownloadDir, downloadPath)
		if err != nil {
			return messages, err
		}
	}
	err = verifyFileSize(bisonConfig.expectedFileSize, downloadPath)
	if err != nil {
		return messages, err
	}
	err = verifySha256(bisonConfig.expectedSha256, downloadPath)
	if err != nil {
		return messages, err
	}
	messages = append(messages, fmt.Sprintf("%s has the correct SHA-256", downloadPath))
	filterBisonTarXz := filterBisonTarXzFactory(bisonConfig.versionedDir)
	transformBisonTarXz := transformBisonTarXzFactory(buildToolConfig.Directories.ToolChainDir)
	err = extractTarXz(downloadPath, filterBisonTarXz, transformBisonTarXz)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Failed to extract %s", downloadPath))
		return messages, err
	}
	err = configureBison(bisonConfig.toolchainDir)
	if err != nil {
		return messages, err
	}
	err = makeBison(bisonConfig.toolchainDir)
	if err != nil {
		return messages, err
	}
	bisonConfig.executable = filepath.Join(bisonConfig.toolchainDir, "tests", "bison")
	err = updateBisonToolchainToml(buildToolConfig.Directories.ToolChainDir, bisonConfig.executable, bisonConfig.version)
	return messages, err
}

func configureBison(bisonDir string) error {
	cmd := exec.Command("./configure")
	cmd.Dir = bisonDir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func filterBisonTarXz(versionedDir string, filePath string) bool {
	acceptable := strings.HasPrefix(filePath, versionedDir)
	if !acceptable {
		// TODO: Figure out how to get this in the messages slice.
		log.Printf("Rejecting file with invalid prefix: %s\n", filePath)
	}
	return acceptable
}

func filterBisonTarXzFactory(versionedDir string) fileFilter {
	versionedDir = ensureTrailingSlash(versionedDir)
	return func(filePath string) bool {
		return filterBisonTarXz(versionedDir, filePath)
	}
}

/**
 * getBisonConfig populates templates from the runtime configuration with
 * appropriate values.
 */
func getBisonConfig(buildToolConfig *RuntimeConfigBuildTool) (*bisonConfig, error) {
	if buildToolConfig.Bison == nil {
		return nil, fmt.Errorf("buildToolConfig.Bison is nil")
	}
	if buildToolConfig.Directories == nil {
		return nil, fmt.Errorf("buildToolConfig.Directories is nil")
	}
	// Version
	version := buildToolConfig.Bison.version
	// Download File
	filenameValues := bisonFilenameTemplateValues{
		version,
	}
	filenameTemplate, err := template.New("filename").Parse(buildToolConfig.Bison.filenameTemplate)
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
	downloadUrlValues := bisonDownloadUrlTemplateValues{
		downloadFile,
	}
	downloadUrlTemplate, err := template.New("downloadUrl").Parse(buildToolConfig.Bison.downloadUrlTemplate)
	if err != nil {
		return nil, err
	}
	var downloadUrlBuffer bytes.Buffer
	err = downloadUrlTemplate.Execute(&downloadUrlBuffer, downloadUrlValues)
	if err != nil {
		return nil, err
	}
	downloadUrl := downloadUrlBuffer.String()
	downloadFileInfo := buildToolConfig.Bison.files[downloadFile]
	if downloadFileInfo == (runtimeConfigFile{}) {
		return nil, fmt.Errorf("File size and SHA-256 not found in downloads.toml for %s", downloadFile)
	}
	// Bison executable
	executable := buildToolConfig.Bison.Executable
	// Expected file size and SHA-256
	expectedFileSize := downloadFileInfo.size
	expectedSha256 := downloadFileInfo.sha256
	// Toolchain directory
	toolchainDir := buildToolConfig.Bison.toolchainDir
	// Toolchain executable
	toolchainExecutable := buildToolConfig.Bison.toolchainExecutable
	// Toolchain version
	toolchainVersion := buildToolConfig.Bison.toolchainVersion
	versionedDir := buildToolConfig.Bison.versionedDir

	bisonConfig := new(bisonConfig)
	bisonConfig.downloadFile = downloadFile
	bisonConfig.downloadUrl = downloadUrl
	bisonConfig.executable = executable
	bisonConfig.expectedFileSize = expectedFileSize
	bisonConfig.expectedSha256 = expectedSha256
	bisonConfig.toolchainDir = toolchainDir
	bisonConfig.toolchainExecutable = toolchainExecutable
	bisonConfig.toolchainVersion = toolchainVersion
	bisonConfig.version = version
	bisonConfig.versionedDir = versionedDir
	return bisonConfig, nil
}

func makeBison(bisonDir string) error {
	cmd := exec.Command("make")
	cmd.Dir = bisonDir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func transformBisonTarXz(toolchainDir string, filePath string) string {
	return filepath.Join(toolchainDir, filePath)
}

func transformBisonTarXzFactory(toolchainDir string) fileTransformer {
	return func(filePath string) string {
		return transformBisonTarXz(toolchainDir, filePath)
	}
}

func updateBisonToolchainToml(toolchainDir string, executable string, version string) error {
	toolchainTomlTopLevel, err := ReadToolchainToml(toolchainDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		toolchainTomlTopLevel = new(ToolchainTomlTopLevel)
	}
	if toolchainTomlTopLevel.Bison == nil {
		toolchainTomlTopLevel.Bison = new(ToolchainTomlTool)
	}
	toolchainTomlTopLevel.Bison.Executable = executable
	toolchainTomlTopLevel.Bison.Version = version
	return WriteToolchainToml(toolchainDir, toolchainTomlTopLevel)
}
