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
	"github.com/BurntSushi/toml"
)

type DownloadsTomlTopLevel struct {
	Bison     DownloadsTomlTool `toml:"bison"`
	CapnProto DownloadsTomlTool `toml:"capnproto"`
	Flex      DownloadsTomlTool `toml:"flex"`
	GoCapnp   DownloadsTomlTool `toml:"go-capnp"`
	Linux     DownloadsTomlTool `toml:"linux"`
	TinyGo    DownloadsTomlTool `toml:"tinygo"`
}

type DownloadsTomlTool struct {
	DownloadUrlTemplate string
	FilenameTemplate    string
	Files               map[string]DownloadsTomlFile
	PreferredVersion    string
}

type DownloadsTomlFile struct {
	Sha256 string `toml:"SHA-256"`
	Size   int64
}

func ReadDownloadsFile(downloadsFilePath *string) (*DownloadsTomlTopLevel, error) {
	downloads := new(DownloadsTomlTopLevel)
	_, err := toml.DecodeFile(*downloadsFilePath, downloads)
	if err != nil {
		return nil, err
	}
	return downloads, nil
}
