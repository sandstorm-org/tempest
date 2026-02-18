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
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ToolchainTomlTopLevel struct {
	Binaryen  *ToolchainTomlTool `toml:"binaryen"`
	Bison     *ToolchainTomlTool `toml:"bison"`
	BpfAsm    *ToolchainTomlTool `toml:"bpf-asm"`
	CapnProto *ToolchainTomlTool `toml:"capnproto"`
	Flex      *ToolchainTomlTool `toml:"flex"`
	Go        *ToolchainTomlTool `toml:"go"`
	GoCapnp   *ToolchainTomlTool `toml:"go-capnp"`
	TinyGo    *ToolchainTomlTool `toml:"tinygo"`
}

type ToolchainTomlTool struct {
	Executable string `toml:"Executable,omitempty"`
	Version    string `toml:"Version,omitempty"`
}

func ReadToolchainToml(toolchainDir string) (*ToolchainTomlTopLevel, error) {
	toolchainToml := new(ToolchainTomlTopLevel)
	toolchainTomlFilePath := toolchainTomlFilePathWithToolchainDir(toolchainDir)
	_, err := toml.DecodeFile(toolchainTomlFilePath, toolchainToml)
	if err != nil {
		return nil, err
	}
	return toolchainToml, nil
}

func WriteToolchainToml(toolchainDir string, toolchainTomlTopLevel *ToolchainTomlTopLevel) error {
	toolchainTomlFilePath := toolchainTomlFilePathWithToolchainDir(toolchainDir)
	fp, err := os.Create(toolchainTomlFilePath)
	if err != nil {
		return err
	}
	defer fp.Close()
	fp.WriteString("# This file is managed by the Tempest build-tool.\n")
	fp.WriteString("# See internal/build-tool/toolchain.go\n")
	fp.WriteString("\n")
	return toml.NewEncoder(fp).Encode(toolchainTomlTopLevel)
}

func toolchainTomlFilePathWithToolchainDir(toolchainDir string) string {
	return filepath.Join(toolchainDir, "toolchain.toml")
}
