package buildtool

import (
	"bytes"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

type tinyGoConfig struct {
	downloadFile     string
	downloadUrl      string
	expectedFileSize int64
	expectedSha256   string
	installDir       string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type tinygoDownloadUrlTemplateValues struct {
	Filename string
	Version  string
}

// text/template uses these struct fields from a separate package, so they must be in PascalCase.
type tinygoFilenameTemplateValues struct {
	Arch    string
	Version string
}

func BootstrapTinyGo(buildToolConfig *RuntimeConfigBuildTool) ([]string, error) {
	messages := make([]string, 0, 5)
	tinyGoConfig, err := getTinyGoConfig(buildToolConfig)
	if err != nil {
		messages = append(messages, "Failed to get TinyGo configuration")
		return messages, err
	}
	err = ensureDownloadDirExists(buildToolConfig.downloadDir)
	if err != nil {
		return messages, err
	}
	downloadPath := filepath.Join(buildToolConfig.downloadDir, tinyGoConfig.downloadFile)
	exists, err := fileExistsAtPath(downloadPath)
	if err != nil {
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Skipping TinyGo download because %s exists", downloadPath))
	} else {
		err := downloadUrlToDir(tinyGoConfig.downloadUrl, buildToolConfig.downloadDir, downloadPath)
		if err != nil {
			return messages, err
		}
	}
	err = verifyFileSize(tinyGoConfig.expectedFileSize, downloadPath)
	if err != nil {
		return messages, err
	}
	err = verifySha256(tinyGoConfig.expectedSha256, downloadPath)
	if err != nil {
		return messages, err
	}
	messages = append(messages, fmt.Sprintf("%s has the correct SHA-256", downloadPath))
	exists, err = fileExistsAtPath(tinyGoConfig.installDir)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if exists {
		messages = append(messages, fmt.Sprintf("Refusing to install TinyGo because %s exists", tinyGoConfig.installDir))
	} else {
		transformTinyGoTarGz := transformTinyGoTarGzFactory(tinyGoConfig.installDir)
		err = extractTarGz(downloadPath, filterTinyGoTarGz, transformTinyGoTarGz)
	}
	// Update the modified time of the TinyGo executable.
	// This is a hack to satisfy `make`.
	// The Makefile looks at the `tinygo` executable.  If its modified time
	// is current, then make will not invoke the target.  If its modified
	// time is not updated, then make will extract TinyGo every time the
	// TinyGo target is invoked.
	tinyGoBinPath := filepath.Join(tinyGoConfig.installDir, "bin/tinygo")
	exists, err = fileExistsAtPath(tinyGoBinPath)
	if err != nil {
		log.Printf("fileExistsAtPath err\n")
		return messages, err
	}
	if exists {
		err = setFileModifiedTimeToNow(tinyGoBinPath)
	}
	return messages, err
}

func filterTinyGoTarGz(filePath string) bool {
	acceptable := strings.HasPrefix(filePath, "tinygo/")
	if !acceptable {
		// TODO: Figure out how to get this in the messages slice.
		log.Printf("Rejecting file with invalid prefix: %s\n", filePath)
	}
	return acceptable
}

/**
 * getTinyGoConfig populates templates from the runtime configuration with
 * appropriate values.
 */
func getTinyGoConfig(buildToolConfig *RuntimeConfigBuildTool) (*tinyGoConfig, error) {
	// Download File
	filenameValues := tinygoFilenameTemplateValues{
		runtime.GOARCH,
		buildToolConfig.tinyGo.version,
	}
	filenameTemplate, err := template.New("filename").Parse(buildToolConfig.tinyGo.filenameTemplate)
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
	downloadUrlValues := tinygoDownloadUrlTemplateValues{
		downloadFile,
		buildToolConfig.tinyGo.version,
	}
	downloadUrlTemplate, err := template.New("downloadUrl").Parse(buildToolConfig.tinyGo.downloadUrlTemplate)
	if err != nil {
		return nil, err
	}
	var downloadUrlBuffer bytes.Buffer
	err = downloadUrlTemplate.Execute(&downloadUrlBuffer, downloadUrlValues)
	if err != nil {
		return nil, err
	}
	downloadUrl := downloadUrlBuffer.String()
	downloadFileInfo := buildToolConfig.tinyGo.files[downloadFile]
	if downloadFileInfo == (runtimeConfigFile{}) {
		return nil, fmt.Errorf("File size and SHA-256 not found in downloads.toml for %s", downloadFile)
	}
	// Expected file size and SHA-256
	expectedFileSize := downloadFileInfo.size
	expectedSha256 := downloadFileInfo.sha256
	// Install directory
	tinyGoVersionedDir := "tinygo-" + buildToolConfig.tinyGo.version
	installDir := filepath.Join(buildToolConfig.toolChainDir, tinyGoVersionedDir)

	tinyGoConfig := new(tinyGoConfig)
	tinyGoConfig.downloadFile = downloadFile
	tinyGoConfig.downloadUrl = downloadUrl
	tinyGoConfig.expectedFileSize = expectedFileSize
	tinyGoConfig.expectedSha256 = expectedSha256
	tinyGoConfig.installDir = installDir
	return tinyGoConfig, nil
}

func transformTinyGoTarGz(destinationDir string, filePath string) string {
	return filepath.Join(destinationDir, filePath[6:])
}

func transformTinyGoTarGzFactory(destinationDir string) fileTransformer {
	destinationDir = ensureTrailingSlash(destinationDir)
	return func(filePath string) string {
		return transformTinyGoTarGz(destinationDir, filePath)
	}
}
