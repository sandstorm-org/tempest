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
	bisonExecutable string
	executable      string
	flexExecutable  string
	makePath        string
	installDir      string
}

func BootstrapBpfAsm(buildToolConfig *RuntimeConfigBuildTool) ([]string, error) {
	messages := make([]string, 0, 5)
	bpfAsmConfig, err := getBpfAsmConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get bpf_asm configuration")
		return messages, err
	}
	var downloadMessages []string
	var downloadPath string
	downloadPath, downloadMessages, err = downloadAndVerifyLinuxTarball(buildToolConfig)
	if err != nil {
		messages = append(messages, downloadMessages[:]...)
		return messages, err
	}
	var exists bool
	exists, err = fileExistsAtPath(bpfAsmConfig.executable)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Refusing to unpack bpf_asm because %s exists", bpfAsmConfig.executable))
	} else {
		desiredPrefixes := make([]string, 0, 3)
		desiredPrefixes = append(desiredPrefixes, "linux-"+buildToolConfig.linux.version+"/tools/bpf/")
		desiredPrefixes = append(desiredPrefixes, "linux-"+buildToolConfig.linux.version+"/tools/build/")
		desiredPrefixes = append(desiredPrefixes, "linux-"+buildToolConfig.linux.version+"/tools/scripts/")
		commonPrefix := "linux-" + buildToolConfig.linux.version
		filterLinuxTarXz := filterLinuxTarXzFactory(desiredPrefixes)
		transformLinuxTarXz := transformLinuxTarXzFactory(bpfAsmConfig.installDir, len(commonPrefix))
		err = extractTarXz(downloadPath, filterLinuxTarXz, transformLinuxTarXz)
		if err != nil {
			return messages, err
		}
		err = makeBpfAsm(bpfAsmConfig)
	}
	err = updateBpfAsmToolchainToml(buildToolConfig.toolChainDir, bpfAsmConfig.executable)
	return messages, err
}

func getBpfAsmConfig(buildToolConfig *RuntimeConfigBuildTool) (*bpfAsmConfig, error) {
	if buildToolConfig.bison == nil {
		return nil, fmt.Errorf("buildToolConfig.bison is nil")
	} else if buildToolConfig.bpfAsm == nil {
		return nil, fmt.Errorf("buildToolConfig.bpfAsm is nil")
	} else if buildToolConfig.flex == nil {
		return nil, fmt.Errorf("buildToolConfig.flex is nil")
	} else if buildToolConfig.linux == nil {
		return nil, fmt.Errorf("buildToolConfig.linux is nil")
	}
	bisonExecutable := buildToolConfig.bison.executable
	flexExecutable := buildToolConfig.flex.executable
	// Install directory
	bpfAsmVersionedDir := "bpf_asm-" + buildToolConfig.linux.version
	installDir := filepath.Join(buildToolConfig.toolChainDir, bpfAsmVersionedDir)
	// BpfAsm executable
	executable := filepath.Join(installDir, "tools", "bpf", "bpf_asm")
	// BpfAsm make path
	makePath := filepath.Join(installDir, "tools", "bpf")

	bpfAsmConfig := new(bpfAsmConfig)
	bpfAsmConfig.bisonExecutable = bisonExecutable
	bpfAsmConfig.executable = executable
	bpfAsmConfig.flexExecutable = flexExecutable
	bpfAsmConfig.installDir = installDir
	bpfAsmConfig.makePath = makePath
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

func updateBpfAsmToolchainToml(toolchainDir string, executable string) error {
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
	return WriteToolchainToml(toolchainDir, toolchainTomlTopLevel)
}
