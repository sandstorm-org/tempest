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
	"path/filepath"
)

type bpfAsmConfig struct {
	installDir string
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
	exists, err = fileExistsAtPath(bpfAsmConfig.installDir)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Refusing to unpack bpf_asm because %s exists", bpfAsmConfig.installDir))
	} else {
		desiredPrefixes := make([]string, 0, 3)
		desiredPrefixes = append(desiredPrefixes, "linux-"+buildToolConfig.linux.version+"/tools/bpf/")
		desiredPrefixes = append(desiredPrefixes, "linux-"+buildToolConfig.linux.version+"/tools/build/")
		desiredPrefixes = append(desiredPrefixes, "linux-"+buildToolConfig.linux.version+"/tools/scripts/")
		commonPrefix := "linux-" + buildToolConfig.linux.version
		filterLinuxTarXz := filterLinuxTarXzFactory(desiredPrefixes)
		transformLinuxTarXz := transformLinuxTarXzFactory(bpfAsmConfig.installDir, len(commonPrefix))
		err = extractTarXz(downloadPath, filterLinuxTarXz, transformLinuxTarXz)
	}
	return messages, err
}

func getBpfAsmConfig(buildToolConfig *RuntimeConfigBuildTool) (*bpfAsmConfig, error) {
	bpfAsmVersionedDir := "bpf_asm-" + buildToolConfig.linux.version
	installDir := filepath.Join(buildToolConfig.toolChainDir, bpfAsmVersionedDir)

	bpfAsmConfig := new(bpfAsmConfig)
	bpfAsmConfig.installDir = installDir
	return bpfAsmConfig, nil
}
