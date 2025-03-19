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

type flexConfig struct {
	downloadFile     string
	downloadUrl      string
	executable       string
	expectedFileSize int64
	expectedSha256   string
	installDir       string
	versionedDir     string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type flexDownloadUrlTemplateValues struct {
	Filename string
	Version  string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type flexFilenameTemplateValues struct {
	Version string
}

func BootstrapFlex(buildToolConfig *RuntimeConfigBuildTool) ([]string, error) {
	messages := make([]string, 0, 5)
	flexConfig, err := getFlexConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get Flex configuration")
		return messages, err
	}
	err = ensureDownloadDirExists(buildToolConfig.downloadDir)
	if err != nil {
		return messages, err
	}
	downloadPath := filepath.Join(buildToolConfig.downloadDir, flexConfig.downloadFile)
	exists, err := fileExistsAtPath(downloadPath)
	if err != nil {
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Skipping Flex download because %s exists", downloadPath))
	} else {
		err := downloadUrlToDir(flexConfig.downloadUrl, buildToolConfig.downloadDir, downloadPath)
		if err != nil {
			return messages, err
		}
	}
	err = verifyFileSize(flexConfig.expectedFileSize, downloadPath)
	if err != nil {
		return messages, err
	}
	err = verifySha256(flexConfig.expectedSha256, downloadPath)
	if err != nil {
		return messages, err
	}
	messages = append(messages, fmt.Sprintf("%s has the correct SHA-256", downloadPath))
	exists, err = fileExistsAtPath(flexConfig.executable)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Refusing to install Flex because %s exists", flexConfig.executable))
	} else {
		filterFlexTarGz := filterFlexTarGzFactory(flexConfig.versionedDir)
		transformFlexTarGz := transformFlexTarGzFactory(buildToolConfig.toolChainDir)
		err = extractTarGz(downloadPath, filterFlexTarGz, transformFlexTarGz)
		if err != nil {
			return messages, err
		}
		err = configureFlex(flexConfig.installDir)
		if err != nil {
			return messages, err
		}
		err = makeFlex(flexConfig.installDir)
		if err != nil {
			return messages, err
		}
	}
	err = updateFlexToolchainToml(buildToolConfig.toolChainDir, flexConfig.executable)
	return messages, err
}

func configureFlex(flexDir string) error {
	cmd := exec.Command("./configure")
	cmd.Dir = flexDir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func filterFlexTarGz(versionedDir string, filePath string) bool {
	acceptable := strings.HasPrefix(filePath, versionedDir)
	if !acceptable {
		// TODO: Figure out how to get this in the messages slice.
		log.Printf("Rejecting file with invalid prefix: %s\n", filePath)
	}
	return acceptable
}

func filterFlexTarGzFactory(versionedDir string) fileFilter {
	versionedDir = ensureTrailingSlash(versionedDir)
	return func(filePath string) bool {
		return filterFlexTarGz(versionedDir, filePath)
	}
}

/**
 * getFlexConfig populates templates from the runtime configuration with
 * appropriate values.
 */
func getFlexConfig(buildToolConfig *RuntimeConfigBuildTool) (*flexConfig, error) {
	// Download File
	filenameValues := flexFilenameTemplateValues{
		buildToolConfig.flex.version,
	}
	filenameTemplate, err := template.New("filename").Parse(buildToolConfig.flex.filenameTemplate)
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
	downloadUrlValues := flexDownloadUrlTemplateValues{
		downloadFile,
		buildToolConfig.flex.version,
	}
	downloadUrlTemplate, err := template.New("downloadUrl").Parse(buildToolConfig.flex.downloadUrlTemplate)
	if err != nil {
		return nil, err
	}
	var downloadUrlBuffer bytes.Buffer
	err = downloadUrlTemplate.Execute(&downloadUrlBuffer, downloadUrlValues)
	if err != nil {
		return nil, err
	}
	downloadUrl := downloadUrlBuffer.String()
	downloadFileInfo := buildToolConfig.flex.files[downloadFile]
	if downloadFileInfo == (runtimeConfigFile{}) {
		return nil, fmt.Errorf("File size and SHA-256 not found in downloads.toml for %s", downloadFile)
	}
	// Expected file size and SHA-256
	expectedFileSize := downloadFileInfo.size
	expectedSha256 := downloadFileInfo.sha256
	// Install directory
	versionedDir := "flex-" + buildToolConfig.flex.version
	installDir := filepath.Join(buildToolConfig.toolChainDir, versionedDir)
	// Flex executable
	executable := filepath.Join(installDir, "src", "flex")

	flexConfig := new(flexConfig)
	flexConfig.downloadFile = downloadFile
	flexConfig.downloadUrl = downloadUrl
	flexConfig.executable = executable
	flexConfig.expectedFileSize = expectedFileSize
	flexConfig.expectedSha256 = expectedSha256
	flexConfig.installDir = installDir
	flexConfig.versionedDir = versionedDir
	return flexConfig, nil
}

func makeFlex(flexDir string) error {
	cmd := exec.Command("make")
	cmd.Dir = flexDir
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func transformFlexTarGz(toolchainDir string, filePath string) string {
	return filepath.Join(toolchainDir, filePath)
}

func transformFlexTarGzFactory(toolchainDir string) fileTransformer {
	return func(filePath string) string {
		return transformFlexTarGz(toolchainDir, filePath)
	}
}

func updateFlexToolchainToml(toolchainDir string, executable string) error {
	toolchainTomlTopLevel, err := ReadToolchainToml(toolchainDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		toolchainTomlTopLevel = new(ToolchainTomlTopLevel)
	}
	if toolchainTomlTopLevel.Flex == nil {
		toolchainTomlTopLevel.Flex = new(ToolchainTomlTool)
	}
	toolchainTomlTopLevel.Flex.Executable = executable
	return WriteToolchainToml(toolchainDir, toolchainTomlTopLevel)
}
