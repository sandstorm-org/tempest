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

type capnProtoConfig struct {
	downloadFile        string
	downloadUrl         string
	executable          string
	expectedFileSize    int64
	expectedSha256      string
	tarGzDir            string
	toolchainDir        string
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type capnProtoDownloadUrlTemplateValues struct {
	Filename string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type capnProtoFilenameTemplateValues struct {
	Version string
}

func BootstrapCapnProto(buildToolConfig *RuntimeConfigBuildTool) ([]string, error) {
	messages := make([]string, 0, 5)
	capnProtoConfig, err := getCapnProtoConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get Cap'n Proto configuration")
		return messages, err
	}
	exists, err := fileExistsAtPath(capnProtoConfig.executable)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if capnProtoConfig.executable == capnProtoConfig.toolchainExecutable {
		if capnProtoConfig.version == capnProtoConfig.toolchainVersion && exists {
			messages = append(messages, fmt.Sprintf("Skipping download and installation of Cap'n Proto because %s exists", capnProtoConfig.executable))
			return messages, err
		}
	} else if capnProtoConfig.executable != "" {
		if exists {
			messages = append(messages, fmt.Sprintf("Skipping download and installation of Cap'n Proto because %s exists", capnProtoConfig.executable))
			return messages, err
		} else {
			err = fmt.Errorf("Specified Cap'n Proto executable %s is outside the toolchain and does not exist.")
			return messages, err
		}
	}
	err = ensureDownloadDirExists(buildToolConfig.Directories.DownloadDir)
	if err != nil {
		return messages, err
	}
	downloadPath := filepath.Join(buildToolConfig.Directories.DownloadDir, capnProtoConfig.downloadFile)
	exists, err = fileExistsAtPath(downloadPath)
	if err != nil {
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Skipping Cap'n Proto download because %s exists", downloadPath))
	} else {
		err := downloadUrlToDir(capnProtoConfig.downloadUrl, buildToolConfig.Directories.DownloadDir, downloadPath)
		if err != nil {
			return messages, err
		}
	}
	err = verifyFileSize(capnProtoConfig.expectedFileSize, downloadPath)
	if err != nil {
		return messages, err
	}
	err = verifySha256(capnProtoConfig.expectedSha256, downloadPath)
	if err != nil {
		return messages, err
	}
	messages = append(messages, fmt.Sprintf("%s has the correct SHA-256", downloadPath))
	filterCapnProtoTarGz := filterCapnProtoTarGzFactory(capnProtoConfig.tarGzDir)
	transformCapnProtoTarGz := transformCapnProtoTarGzFactory(capnProtoConfig.toolchainDir, len(capnProtoConfig.tarGzDir))
	err = extractTarGz(downloadPath, filterCapnProtoTarGz, transformCapnProtoTarGz)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Failed to extract %s", downloadPath))
		return messages, err
	}
	err = configureCapnProto(capnProtoConfig.toolchainDir)
	if err != nil {
		messages = append(messages, "Failed while running ./configure for Cap'n Proto")
		return messages, err
	}
	err = makeCapnProto(capnProtoConfig.toolchainDir)
	if err != nil {
		messages = append(messages, "Failed while running make for Cap'n Proto")
		return messages, err
	}
	capnProtoConfig.executable = filepath.Join(capnProtoConfig.toolchainDir, "capnp")
	err = updateCapnProtoToolchainToml(buildToolConfig.Directories.ToolChainDir, capnProtoConfig.executable, capnProtoConfig.version)
	return messages, err
}

func configureCapnProto(capnProtoDir string) error {
	cmd := exec.Command("./configure")
	cmd.Dir = capnProtoDir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func filterCapnProtoTarGz(tarGzDir string, filePath string) bool {
	acceptable := strings.HasPrefix(filePath, tarGzDir)
	if !acceptable {
		// TODO: Figure out how to get this in the messages slice.
		log.Printf("Rejecting file with invalid prefix: %s\n", filePath)
	}
	return acceptable
}

func filterCapnProtoTarGzFactory(tarGzDir string) fileFilter {
	tarGzDir = ensureTrailingSlash(tarGzDir)
	return func(filePath string) bool {
		return filterCapnProtoTarGz(tarGzDir, filePath)
	}
}

func getCapnProtoConfig(buildToolConfig *RuntimeConfigBuildTool) (*capnProtoConfig, error) {
	if buildToolConfig.CapnProto == nil {
		return nil, fmt.Errorf("buildToolConfig.CapnProto is nil")
	}
	// Version
	version := buildToolConfig.CapnProto.version
	// Download File
	filenameValues := capnProtoFilenameTemplateValues{
		version,
	}
	filenameTemplate, err := template.New("filename").Parse(buildToolConfig.CapnProto.filenameTemplate)
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
	downloadUrlValues := capnProtoDownloadUrlTemplateValues{
		downloadFile,
	}
	downloadUrlTemplate, err := template.New("downloadUrl").Parse(buildToolConfig.CapnProto.downloadUrlTemplate)
	if err != nil {
		return nil, err
	}
	var downloadUrlBuffer bytes.Buffer
	err = downloadUrlTemplate.Execute(&downloadUrlBuffer, downloadUrlValues)
	if err != nil {
		return nil, err
	}
	downloadUrl := downloadUrlBuffer.String()
	downloadFileInfo := buildToolConfig.CapnProto.files[downloadFile]
	if downloadFileInfo == (runtimeConfigFile{}) {
		return nil, fmt.Errorf("File size and SHA-256 not found in downloads.toml for %s", downloadFile)
	}
	// Expected file size and SHA-256
	expectedFileSize := downloadFileInfo.size
	expectedSha256 := downloadFileInfo.sha256
	// Install directory
	capnProtoVersionedDir := "capnproto-" + version
	toolchainDir := filepath.Join(buildToolConfig.Directories.ToolChainDir, capnProtoVersionedDir)
	// TarGz directory
	tarGzDir := "capnproto-c++-" + version
	// capnp executable
	executable := buildToolConfig.CapnProto.Executable
	// Toolchain executable
	toolchainExecutable := buildToolConfig.CapnProto.toolchainExecutable
	// Toolchain version
	toolchainVersion := buildToolConfig.CapnProto.toolchainVersion

	capnProtoConfig := new(capnProtoConfig)
	capnProtoConfig.downloadFile = downloadFile
	capnProtoConfig.downloadUrl = downloadUrl
	capnProtoConfig.executable = executable
	capnProtoConfig.expectedFileSize = expectedFileSize
	capnProtoConfig.expectedSha256 = expectedSha256
	capnProtoConfig.tarGzDir = tarGzDir
	capnProtoConfig.toolchainDir = toolchainDir
	capnProtoConfig.toolchainExecutable = toolchainExecutable
	capnProtoConfig.toolchainVersion = toolchainVersion
	capnProtoConfig.version = version
	return capnProtoConfig, nil
}

func makeCapnProto(capnProtoDir string) error {
	cmd := exec.Command("make")
	cmd.Args = append(cmd.Args, "check")
	cmd.Dir = capnProtoDir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func transformCapnProtoTarGz(toolchainDir string, filePath string, prefixLength int) string {
	maxLength := min(len(filePath), prefixLength)
	return filepath.Join(toolchainDir, filePath[maxLength:])
}

func transformCapnProtoTarGzFactory(destinationDir string, prefixLength int) fileTransformer {
	destinationDir = ensureTrailingSlash(destinationDir)
	return func(filePath string) string {
		return transformCapnProtoTarGz(destinationDir, filePath, prefixLength)
	}
}

func updateCapnProtoToolchainToml(toolchainDir string, executable string, version string) error {
	toolchainTomlTopLevel, err := ReadToolchainToml(toolchainDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		toolchainTomlTopLevel = new(ToolchainTomlTopLevel)
	}
	if toolchainTomlTopLevel.CapnProto == nil {
		toolchainTomlTopLevel.CapnProto = new(ToolchainTomlTool)
	}
	toolchainTomlTopLevel.CapnProto.Executable = executable
	toolchainTomlTopLevel.CapnProto.Version = version
	return WriteToolchainToml(toolchainDir, toolchainTomlTopLevel)
}
