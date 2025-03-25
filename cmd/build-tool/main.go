package main

import (
	"log"
	"os"

	"github.com/alecthomas/kong"
	buildtool "sandstorm.org/go/tempest/internal/build-tool"
)

const DefaultConfigPath = "./config.toml"
const DefaultDownloadsFilePath = "./internal/build-tool/downloads.toml"

var CLI struct {
	BootstrapBison     struct{} `cmd:"" help:"Bootstrap Bison"`
	BootstrapBpfAsm    struct{} `cmd:"" help:"Bootstrap bpf_asm" name:"bootstrap-bpf_asm"`
	BootstrapCapnProto struct{} `cmd:"" help:"Bootstrap Cap'n Proto" name:"bootstrap-capnproto"`
	BootstrapFlex      struct{} `cmd:"" help:"Bootstrap Flex"`
	BootstrapTinygo    struct{} `cmd:"" help:"Bootstrap TinyGo"`

	Config        string `default:"./config.toml" help:"path to the config file"`
	DownloadsFile string `default:"./internal/build-tool/downloads.toml" help:"path to the downloads information file"`
	Verbose       bool   `help:"verbose output"`
}

func main() {
	context := kong.Parse(&CLI)

	config, err := loadConfiguration(&CLI.Config, &CLI.DownloadsFile)
	if err != nil {
		log.Fatal(err)
	}

	switch context.Command() {
	case "bootstrap-bison":
		messages, err := buildtool.BootstrapBison(config)
		logMessages(CLI.Verbose, messages)
		if err != nil {
			log.Fatal(err)
		}
		break
	case "bootstrap-bpf_asm":
		messages, err := buildtool.BootstrapBpfAsm(config)
		logMessages(CLI.Verbose, messages)
		if err != nil {
			log.Fatal(err)
		}
		break
	case "bootstrap-capnproto":
		messages, err := buildtool.BootstrapCapnProto(config)
		logMessages(CLI.Verbose, messages)
		if err != nil {
			log.Fatal(err)
		}
		break
	case "bootstrap-flex":
		messages, err := buildtool.BootstrapFlex(config)
		logMessages(CLI.Verbose, messages)
		if err != nil {
			log.Fatal(err)
		}
		break
	case "bootstrap-tinygo":
		messages, err := buildtool.BootstrapTinyGo(config)
		logMessages(CLI.Verbose, messages)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func loadConfiguration(configFileFlag *string, downloadsFileFlag *string) (*buildtool.RuntimeConfigBuildTool, error) {
	// Config file
	configFilePath := selectConfigFile(configFileFlag)
	configFile, err := buildtool.ReadConfigFile(configFilePath)
	if err != nil {
		return nil, err
	}

	// Downloads file
	if downloadsFileFlag == nil || downloadsFileFlag != nil && *downloadsFileFlag == "" {
		if configFile.BuildTool.DownloadsFile == "" {
			*downloadsFileFlag = DefaultDownloadsFilePath
		} else {
			downloadsFileFlag = &configFile.BuildTool.DownloadsFile
		}
	}
	var downloadsFile *buildtool.DownloadsTomlTopLevel
	downloadsFile, err = buildtool.ReadDownloadsFile(downloadsFileFlag)
	if err != nil {
		return nil, err
	}

	// Runtime configuration
	var config *buildtool.RuntimeConfigBuildTool
	config, err = buildtool.BuildConfiguration(configFile, downloadsFile)
	if err != nil {
		return nil, err
	}

	return config, err
}

func logMessages(writeOutput bool, messages []string) {
	if writeOutput && messages != nil {
		for messageIndex := range messages {
			log.Print(messages[messageIndex])
		}
	}
}

// Select a configuration file.  Use, in order of preference, the file specified by:
//  1. the --config command-line flag,
//  2. the CONFIG environment variable, or
//  3. the default path of "./config.toml".
func selectConfigFile(configFileFlag *string) *string {
	if configFileFlag != nil {
		return configFileFlag
	}
	configEnvVar := os.Getenv("CONFIG")
	if configEnvVar != "" {
		return &configEnvVar
	}
	var result *string
	*result = DefaultConfigPath
	return result
}
