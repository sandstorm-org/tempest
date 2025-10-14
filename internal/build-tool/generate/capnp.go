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

package generate

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	buildtool "sandstorm.org/go/tempest/internal/build-tool"
)

type generateCapnpConfig struct {
	capnpDirs         []string
	capnpExecutable   string
	goCapnpExecutable string
	incrementalDir    string
	stdDir            string
}

func GenerateCapnp(buildToolConfig *buildtool.RuntimeConfigBuildTool) ([]string, error) {
	messages := make([]string, 0, 5)
	config, err := getGenerateCapnpConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get the Generate Cap'n Proto configuration")
		return messages, err
	}
	log.Printf("GenerateCapnp\n")
	log.Printf("%+v", buildToolConfig)
	capnpFilepaths, err := getGlobbedCapnpFilePaths(config)
	log.Printf("%+v", capnpFilepaths)
	for _, capnpFilepath := range capnpFilepaths {
		cgr, err := codeGeneratorRequestWithCapnp(config, capnpFilepath)
		if err != nil {
			messages = append(messages, "Failed to create CodeGeneratorRequest for file "+capnpFilepath)
			return messages, err
		}
		writeGoCapnpFileWithCGR(config, capnpFilepath, cgr)
	}
	return messages, nil
}

func codeGeneratorRequestWithCapnp(config *generateCapnpConfig, capnpFilepath string) ([]byte, error) {
	cmd := exec.Command(config.capnpExecutable)
	capnpDirectory := filepath.Dir(capnpFilepath)
	cmd.Args = append(
		cmd.Args,
		"compile",
		"--output=-", // output CodeGeneratorRequest messages to stdout
		"--src-prefix="+capnpDirectory+"/",
		"--import-path="+config.stdDir,
		"--import-path=capnp",
		capnpFilepath,
	)
	log.Printf("%+v", cmd.Args)
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Stderr = os.Stderr
	codeGeneratorRequest, err := cmd.Output()
	return codeGeneratorRequest, err
}

func getGenerateCapnpConfig(buildToolConfig *buildtool.RuntimeConfigBuildTool) (*generateCapnpConfig, error) {
	if buildToolConfig.CapnProto == nil {
		return nil, fmt.Errorf("buildToolConfig.CapnProto is nil")
	}
	if buildToolConfig.Directories == nil {
		return nil, fmt.Errorf("buildToolConfig.Directories is nil")
	}
	if buildToolConfig.Generate == nil {
		return nil, fmt.Errorf("buildToolConfig.Generate is nil")
	}
	if buildToolConfig.Generate.Capnp == nil {
		return nil, fmt.Errorf("buildToolConfig.Generate.Capnp is nil")
	}
	if buildToolConfig.GoCapnp == nil {
		return nil, fmt.Errorf("buildToolConfig.GoCapnp is nil")
	}
	capnpExecutable := buildToolConfig.CapnProto.Executable
	capnpDirs := buildToolConfig.Generate.Capnp.CapnpDirs
	goCapnpExecutable := buildToolConfig.GoCapnp.Executable
	//	incrementalDir := buildToolConfig.Directories.IncrementalDir
	stdDir := buildToolConfig.Generate.Capnp.StdDir

	result := new(generateCapnpConfig)
	result.capnpDirs = capnpDirs
	result.capnpExecutable = capnpExecutable
	result.goCapnpExecutable = goCapnpExecutable
	result.stdDir = stdDir
	return result, nil
}

func getGlobbedCapnpFilePaths(config *generateCapnpConfig) ([]string, error) {
	result := make([]string, 0, 0)
	for _, dir := range config.capnpDirs {
		files, err := filepath.Glob(dir + "/*.capnp")
		if err != nil {
			return result, err
		}
		result = append(result, files...)
	}
	return result, nil
}

func writeGoCapnpFileWithCGR(config *generateCapnpConfig, capnpFilepath string, codeGeneratorRequest []byte) error {
	capnpDirectory := filepath.Dir(capnpFilepath)
	cmd := exec.Command(config.goCapnpExecutable)
	cmd.Dir = capnpDirectory
	cmd.Stdin = bytes.NewReader(codeGeneratorRequest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}
