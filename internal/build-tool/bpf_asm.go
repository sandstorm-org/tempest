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
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type bpfAsmConfig struct {
	bisonExecutable     string
	executable          string
	flexExecutable      string
	makePath            string
	toolchainDir        string
	toolchainExecutable string
	toolchainVersion    string
	version             string
}

func BootstrapBpfAsm(buildToolConfig *RuntimeConfigBuildTool) ([]string, error) {
	messages := make([]string, 0, 5)
	bpfAsmConfig, err := getBpfAsmConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get bpf_asm configuration")
		return messages, err
	}
	if bpfAsmConfig.executable != "" {
		executableExists, err := fileExistsAtPath(bpfAsmConfig.executable)
		if err != nil {
			log.Printf("fileExistsAtPath err\n")
			return messages, err
		}
		if executableExists {
			messages = append(messages, fmt.Sprintf("Skipping download and installation of bpf_asm because %s (from config.toml) exists", bpfAsmConfig.executable))
			return messages, nil
		} else {
			err = fmt.Errorf("User-specified bpf_asm executable %s does not exist.", bpfAsmConfig.executable)
			return messages, err
		}
	}
	if bpfAsmConfig.toolchainExecutable != "" {
		executableExists, err := fileExistsAtPath(bpfAsmConfig.toolchainExecutable)
		if err != nil {
			log.Printf("fileExistsAtPath err\n")
			return messages, err
		}
		if executableExists {
			if bpfAsmConfig.version == bpfAsmConfig.toolchainVersion {
				messages = append(messages, fmt.Sprintf("Skipping download and installation of bpf_asm because %s (from toolchain) exists", bpfAsmConfig.toolchainExecutable))
				return messages, nil
			} else {
				messages = append(messages, fmt.Sprintf("The toolchain executable does not match the desired version.  Continuing."))
			}
		}
	}
	var downloadMessages []string
	var downloadPath string
	downloadPath, downloadMessages, err = downloadAndVerifyLinuxTarball(buildToolConfig)
	if err != nil {
		messages = append(messages, downloadMessages[:]...)
		return messages, err
	}
	desiredPrefixes := make([]string, 0, 3)
	desiredPrefixes = append(desiredPrefixes, "linux-"+buildToolConfig.linux.version+"/tools/bpf/")
	desiredPrefixes = append(desiredPrefixes, "linux-"+buildToolConfig.linux.version+"/tools/build/")
	desiredPrefixes = append(desiredPrefixes, "linux-"+buildToolConfig.linux.version+"/tools/scripts/")
	commonPrefix := "linux-" + buildToolConfig.linux.version
	filterLinuxTarXz := filterLinuxTarXzFactory(desiredPrefixes)
	transformLinuxTarXz := transformLinuxTarXzFactory(bpfAsmConfig.toolchainDir, len(commonPrefix))
	err = extractTarXz(downloadPath, filterLinuxTarXz, transformLinuxTarXz)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Failed to extract %s", downloadPath))
		return messages, err
	}
	err = makeBpfAsm(bpfAsmConfig)
	if err != nil {
		return messages, err
	}
	toolchainTomlExecutable, err := filepath.Rel(buildToolConfig.Directories.ToolChainDir, filepath.Join(bpfAsmConfig.makePath, "bpf_asm"))
	if err != nil {
		return messages, err
	}
	err = updateBpfAsmToolchainToml(buildToolConfig.Directories.ToolChainDir, toolchainTomlExecutable, bpfAsmConfig.version)
	return messages, err
}

func getBpfAsmConfig(buildToolConfig *RuntimeConfigBuildTool) (*bpfAsmConfig, error) {
	if buildToolConfig.Bison == nil {
		return nil, fmt.Errorf("buildToolConfig.Bison is nil")
	} else if buildToolConfig.BpfAsm == nil {
		return nil, fmt.Errorf("buildToolConfig.BpfAsm is nil")
	} else if buildToolConfig.Directories == nil {
		return nil, fmt.Errorf("buildToolConfig.Directories is nil")
	} else if buildToolConfig.Flex == nil {
		return nil, fmt.Errorf("buildToolConfig.Flex is nil")
	} else if buildToolConfig.linux == nil {
		return nil, fmt.Errorf("buildToolConfig.linux is nil")
	}
	// BpfAsm version
	version := buildToolConfig.BpfAsm.version
	// Bison executable
	bisonExecutable := ""
	if buildToolConfig.Bison.Executable != "" {
		bisonExecutable = buildToolConfig.Bison.Executable
	} else if buildToolConfig.Bison.ToolChainExecutable != "" {
		bisonExecutable = buildToolConfig.Bison.ToolChainExecutable
	} else {
		return nil, fmt.Errorf("Unable to find Bison executable")
	}
	// Flex executable
	flexExecutable := ""
	if buildToolConfig.Flex.Executable != "" {
		flexExecutable = buildToolConfig.Flex.Executable
	} else if buildToolConfig.Flex.ToolChainExecutable != "" {
		flexExecutable = buildToolConfig.Flex.ToolChainExecutable
	} else {
		return nil, fmt.Errorf("Unable to find Flex executable")
	}
	// Install directory
	bpfAsmVersionedDir := "bpf_asm-" + version
	toolchainDir := filepath.Join(buildToolConfig.Directories.ToolChainDir, bpfAsmVersionedDir)
	// BpfAsm executable
	executable := buildToolConfig.BpfAsm.Executable
	// BpfAsm make path
	makePath := filepath.Join(toolchainDir, "tools", "bpf")
	// Toolchain executable
	toolchainExecutable := buildToolConfig.BpfAsm.ToolChainExecutable
	// Toolchain version
	toolchainVersion := buildToolConfig.BpfAsm.toolchainVersion

	bpfAsmConfig := new(bpfAsmConfig)
	bpfAsmConfig.bisonExecutable = bisonExecutable
	bpfAsmConfig.executable = executable
	bpfAsmConfig.flexExecutable = flexExecutable
	bpfAsmConfig.makePath = makePath
	bpfAsmConfig.toolchainDir = toolchainDir
	bpfAsmConfig.toolchainExecutable = toolchainExecutable
	bpfAsmConfig.toolchainVersion = toolchainVersion
	bpfAsmConfig.version = version
	return bpfAsmConfig, nil
}

func makeBpfAsm(config *bpfAsmConfig) error {
	cmd := exec.Command("make")
	cmd.Dir = config.makePath
	lex := config.flexExecutable
	if lex != "flex" {
		lexVar := "LEX=" + lex
		cmd.Args = append(cmd.Args, lexVar)
	}
	yacc := config.bisonExecutable
	if yacc != "bison" {
		yaccVar := "YACC=" + yacc
		cmd.Args = append(cmd.Args, yaccVar)
	}
	cmd.Args = append(cmd.Args, "bpf_asm")
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func updateBpfAsmToolchainToml(toolchainDir string, executable string, version string) error {
	toolchainTomlTopLevel, err := ReadToolchainToml(toolchainDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		toolchainTomlTopLevel = new(ToolchainTomlTopLevel)
	}
	if toolchainTomlTopLevel.BpfAsm == nil {
		toolchainTomlTopLevel.BpfAsm = new(ToolchainTomlTool)
	}
	toolchainTomlTopLevel.BpfAsm.Executable = executable
	toolchainTomlTopLevel.BpfAsm.Version = version
	return WriteToolchainToml(toolchainDir, toolchainTomlTopLevel)
}
