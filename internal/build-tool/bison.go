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
	downloadFile     string
	downloadUrl      string
	executable       string
	expectedFileSize int64
	expectedSha256   string
	installDir       string
	versionedDir     string
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
	if exists {
		messages = append(messages, fmt.Sprintf("Refusing to download and install Bison because %s exists", bisonConfig.executable))
		return messages, err
	}
	err = ensureDownloadDirExists(buildToolConfig.downloadDir)
	if err != nil {
		return messages, err
	}
	downloadPath := filepath.Join(buildToolConfig.downloadDir, bisonConfig.downloadFile)
	exists, err = fileExistsAtPath(downloadPath)
	if err != nil {
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Skipping Bison download because %s exists", downloadPath))
	} else {
		err := downloadUrlToDir(bisonConfig.downloadUrl, buildToolConfig.downloadDir, downloadPath)
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
	transformBisonTarXz := transformBisonTarXzFactory(buildToolConfig.toolChainDir)
	err = extractTarXz(downloadPath, filterBisonTarXz, transformBisonTarXz)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Failed to extract %s", downloadPath))
		return messages, err
	}
	err = configureBison(bisonConfig.installDir)
	if err != nil {
		return messages, err
	}
	err = makeBison(bisonConfig.installDir)
	if err != nil {
		return messages, err
	}
	bisonConfig.executable = filepath.Join(bisonConfig.installDir, "tests", "bison")
	err = updateBisonToolchainToml(buildToolConfig.toolChainDir, bisonConfig.executable)
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
	// Download File
	if buildToolConfig.bison == nil {
		return nil, fmt.Errorf("buildToolConfig.bison is nil")
	}
	filenameValues := bisonFilenameTemplateValues{
		buildToolConfig.bison.version,
	}
	filenameTemplate, err := template.New("filename").Parse(buildToolConfig.bison.filenameTemplate)
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
	downloadUrlTemplate, err := template.New("downloadUrl").Parse(buildToolConfig.bison.downloadUrlTemplate)
	if err != nil {
		return nil, err
	}
	var downloadUrlBuffer bytes.Buffer
	err = downloadUrlTemplate.Execute(&downloadUrlBuffer, downloadUrlValues)
	if err != nil {
		return nil, err
	}
	downloadUrl := downloadUrlBuffer.String()
	downloadFileInfo := buildToolConfig.bison.files[downloadFile]
	if downloadFileInfo == (runtimeConfigFile{}) {
		return nil, fmt.Errorf("File size and SHA-256 not found in downloads.toml for %s", downloadFile)
	}
	// Expected file size and SHA-256
	expectedFileSize := downloadFileInfo.size
	expectedSha256 := downloadFileInfo.sha256
	// Install directory
	versionedDir := "bison-" + buildToolConfig.bison.version
	installDir := filepath.Join(buildToolConfig.toolChainDir, versionedDir)
	// Bison executable
	executable := buildToolConfig.bison.executable

	bisonConfig := new(bisonConfig)
	bisonConfig.downloadFile = downloadFile
	bisonConfig.downloadUrl = downloadUrl
	bisonConfig.executable = executable
	bisonConfig.expectedFileSize = expectedFileSize
	bisonConfig.expectedSha256 = expectedSha256
	bisonConfig.installDir = installDir
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

func updateBisonToolchainToml(toolchainDir string, executable string) error {
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
	return WriteToolchainToml(toolchainDir, toolchainTomlTopLevel)
}
