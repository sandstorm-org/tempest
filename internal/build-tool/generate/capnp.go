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
	capnpFilepaths, err := getGlobbedCapnpFilePaths(config)
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
	capnpDirs := buildToolConfig.Generate.Capnp.CapnpDirs
	// Cap'n Proto executable
	capnpExecutable := ""
	if buildToolConfig.CapnProto.Executable != "" {
		capnpExecutable = buildToolConfig.CapnProto.Executable
	} else if buildToolConfig.CapnProto.ToolChainExecutable != "" {
		capnpExecutable = buildToolConfig.CapnProto.ToolChainExecutable
	} else {
		return nil, fmt.Errorf("Unable to find Cap'n Proto executable")
	}
	// go-capnp executable
	goCapnpExecutable := ""
	if buildToolConfig.GoCapnp.Executable != "" {
		goCapnpExecutable = buildToolConfig.GoCapnp.Executable
	} else if buildToolConfig.GoCapnp.ToolChainExecutable != "" {
		goCapnpExecutable = buildToolConfig.GoCapnp.ToolChainExecutable
	} else {
		return nil, fmt.Errorf("Unable to find go-capnp executable")
	}
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
	// The CodeGeneratorRequest contains the name of the source file, which
	// is used to create the destination file.  We have to put it in the
	// correct directory.
	cmd.Dir = capnpDirectory
	cmd.Stdin = bytes.NewReader(codeGeneratorRequest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}
