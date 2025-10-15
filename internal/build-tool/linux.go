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
	"path/filepath"
	"strings"
	"text/template"
)

type linuxConfig struct {
	downloadFile     string
	downloadUrl      string
	expectedFileSize int64
	expectedSha256   string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type linuxDownloadUrlTemplateValues struct {
	Filename     string
	MajorVersion string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type linuxFilenameTemplateValues struct {
	Version string
}

func downloadAndVerifyLinuxTarball(buildToolConfig *RuntimeConfigBuildTool) (string, []string, error) {
	messages := make([]string, 0, 5)
	linuxConfig, err := getLinuxConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get Linux configuration")
		return "", messages, err
	}
	err = ensureDownloadDirExists(buildToolConfig.Directories.DownloadDir)
	if err != nil {
		return "", messages, err
	}
	downloadPath := filepath.Join(buildToolConfig.Directories.DownloadDir, linuxConfig.downloadFile)
	downloadPathExists, err := fileExistsAtPath(downloadPath)
	if err != nil {
		return "", messages, err
	}
	if downloadPathExists {
		messages = append(messages, fmt.Sprintf("Skipping Linux download because %s exists", downloadPath))
	} else {
		err := downloadUrlToDir(linuxConfig.downloadUrl, buildToolConfig.Directories.DownloadDir, downloadPath)
		if err != nil {
			return "", messages, err
		}
	}
	err = verifyFileSize(linuxConfig.expectedFileSize, downloadPath)
	if err != nil {
		return "", messages, err
	}
	err = verifySha256(linuxConfig.expectedSha256, downloadPath)
	if err != nil {
		return "", messages, err
	}
	messages = append(messages, fmt.Sprintf("%s has the correct SHA-256", downloadPath))
	return downloadPath, messages, err
}

func filterLinuxTarXz(prefixes []string, filePath string) bool {
	acceptable := false
	for _, prefix := range prefixes {
		maxPathLength := min(len(prefix), len(filePath))
		acceptable = strings.HasPrefix(filePath, prefix[:maxPathLength])
		if acceptable {
			break
		}
	}
	if !acceptable {
		// TODO: Figure out how to get this in the messages slice.
		//		log.Printf("Rejecting file with invalid prefix: %s\n", filePath)
	}
	return acceptable
}

func filterLinuxTarXzFactory(desiredPrefixes []string) fileFilter {
	return func(filePath string) bool {
		return filterLinuxTarXz(desiredPrefixes, filePath)
	}
}

/**
 * getLinuxConfig populates templates from the runtime configuration with
 * appropriate values.
 */
func getLinuxConfig(buildToolConfig *RuntimeConfigBuildTool) (*linuxConfig, error) {
	if buildToolConfig.Directories == nil {
		return nil, fmt.Errorf("buildToolConfig.Directories is nil")
	}
	if buildToolConfig.linux == nil {
		return nil, fmt.Errorf("buildToolConfig.linux is nil")
	}
	// Download File
	filenameValues := linuxFilenameTemplateValues{
		buildToolConfig.linux.version,
	}
	filenameTemplate, err := template.New("filename").Parse(buildToolConfig.linux.filenameTemplate)
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
	majorVersion, _, found := strings.Cut(buildToolConfig.linux.version, ".")
	if !found {
		return nil, fmt.Errorf("Unable to extract major version from Linux version: %s", buildToolConfig.linux.version)
	}
	downloadUrlValues := linuxDownloadUrlTemplateValues{
		downloadFile,
		majorVersion,
	}
	downloadUrlTemplate, err := template.New("downloadUrl").Parse(buildToolConfig.linux.downloadUrlTemplate)
	if err != nil {
		return nil, err
	}
	var downloadUrlBuffer bytes.Buffer
	err = downloadUrlTemplate.Execute(&downloadUrlBuffer, downloadUrlValues)
	if err != nil {
		return nil, err
	}
	downloadUrl := downloadUrlBuffer.String()
	downloadFileInfo := buildToolConfig.linux.files[downloadFile]
	if downloadFileInfo == (runtimeConfigFile{}) {
		return nil, fmt.Errorf("File size and SHA-256 not found in downloads.toml for %s", downloadFile)
	}
	// Expected file size and SHA-256
	expectedFileSize := downloadFileInfo.size
	expectedSha256 := downloadFileInfo.sha256

	linuxConfig := new(linuxConfig)
	linuxConfig.downloadFile = downloadFile
	linuxConfig.downloadUrl = downloadUrl
	linuxConfig.expectedFileSize = expectedFileSize
	linuxConfig.expectedSha256 = expectedSha256
	return linuxConfig, nil
}

func transformLinuxTarXz(destinationDir string, filePath string, prefixLength int) string {
	maxLength := min(len(filePath), prefixLength)
	return filepath.Join(destinationDir, filePath[maxLength:])
}

func transformLinuxTarXzFactory(destinationDir string, prefixLength int) fileTransformer {
	destinationDir = ensureTrailingSlash(destinationDir)
	return func(filePath string) string {
		return transformLinuxTarXz(destinationDir, filePath, prefixLength)
	}
}
