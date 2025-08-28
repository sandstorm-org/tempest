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
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/xi2/xz"
)

func downloadUrlToDir(downloadUrl string, downloadDir string, downloadPath string) error {
	tempFile, err := os.CreateTemp(downloadDir, "download-")
	if err != nil {
		return err
	}
	defer tempFile.Close()

	response, err := http.Get(downloadUrl)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s => %s", downloadUrl, response.Status)
	}

	progressBar := progressbar.DefaultBytes(
		response.ContentLength,
		fmt.Sprintf("Downloading %s", downloadUrl),
	)

	_, err = io.Copy(io.MultiWriter(tempFile, progressBar), response.Body)
	if err != nil {
		return err
	}
	err = os.Rename(tempFile.Name(), downloadPath)
	return err
}

func envMap() map[string]string {
	result := make(map[string]string)
	for _, envLine := range os.Environ() {
		if i := strings.Index(envLine, "="); i > 0 {
			result[envLine[:i]] = envLine[i+1:]
		}
	}
	return result
}

func ensureDownloadDirExists(downloadDir string) error {

	err := os.MkdirAll(downloadDir, 0750)
	return err
}

func ensureTrailingSlash(filePath string) string {
	if strings.HasSuffix(filePath, string(os.PathSeparator)) {
		return filePath
	}
	return filePath + string(os.PathSeparator)
}

// Filter files by their names
type fileFilter func(string) bool

// Transform file names before writing them
type fileTransformer func(string) string

func extractTar(tarReader *tar.Reader, fileName string, filter fileFilter, transform fileTransformer) error {
	// Save directory access and modification times to update at the end.
	type dirTime struct {
		dirName          string
		accessTime       time.Time
		modificationTime time.Time
	}
	dirTimes := make([]*dirTime, 0, 100)
	for true {
		next, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if next.Typeflag == tar.TypeDir {
			if filter(next.Name) {
				newDir := transform(next.Name)
				dirMode := fs.FileMode(next.Mode)
				err := os.MkdirAll(newDir, dirMode)
				if err != nil {
					return err
				}
				aDirTime := dirTime{newDir, next.AccessTime, next.ModTime}
				dirTimes = append(dirTimes, &aDirTime)
			} else {
				// TODO: Add this to messages
				//				log.Printf("Ignoring rejected directory: %s", next.Name)
			}
		} else if next.Typeflag == tar.TypeReg {
			if filter(next.Name) {
				newFile := transform(next.Name)
				aFile, err := os.Create(newFile)
				if err != nil {
					return err
				}
				if _, err := io.Copy(aFile, tarReader); err != nil {
					return err
				}
				fileMode := fs.FileMode(next.Mode)
				err = aFile.Chmod(fileMode)
				if err != nil {
					return err
				}
				aFile.Close()
				err = os.Chtimes(newFile, next.AccessTime, next.ModTime)
				if err != nil {
					return err
				}
			} else {
				// TODO: Add this to messages
				//				log.Printf("Ignoring rejected file: %s", next.Name)
			}
		} else if next.Typeflag == tar.TypeSymlink {
			// Ignore symlinks (which occur in the Linux kernel source tarball)
		} else if next.Typeflag == tar.TypeXGlobalHeader {
			// Ignore the PAX format global header
		} else {
			return fmt.Errorf("Unexpected type in tar header: %s (%s)", next.Typeflag, fileName)
		}
	}
	// Fix the directory access times and modification times
	slices.Reverse(dirTimes)
	for dirTimeIndex := range dirTimes {
		aDirTime := dirTimes[dirTimeIndex]
		err := os.Chtimes(aDirTime.dirName, aDirTime.accessTime, aDirTime.modificationTime)
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(tgzArchive string, filter fileFilter, transform fileTransformer) error {
	tgzFile, err := os.Open(tgzArchive)
	if err != nil {
		return err
	}
	defer tgzFile.Close()
	gzipReader, err := gzip.NewReader(tgzFile)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(gzipReader)
	err = extractTar(tarReader, tgzArchive, filter, transform)
	return err
}

func extractTarXz(txzArchive string, filter fileFilter, transform fileTransformer) error {
	txzFile, err := os.Open(txzArchive)
	if err != nil {
		return err
	}
	defer txzFile.Close()
	xzReader, err := xz.NewReader(txzFile, 0)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(xzReader)
	err = extractTar(tarReader, txzArchive, filter, transform)
	return err
}

// Return true if a file exists at the path and the file is not a directory.
//
// The default executable for Cap'n Proto is "capnp".  This is also the name of
// a directory in the root of the project.  fileExistsAtPath() was finding that
// "capnp" (the directory) existed instead of noting that the Cap'n Proto
// compiler "capnp" did not exist, thus it was refusing to build the Cap'n
// Proto compiler.
func fileExistsAtPath(filePath string) (bool, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return fileInfo != nil && !fileInfo.IsDir(), nil
}

func setFileModifiedTimeToNow(filePath string) error {
	now := time.Now().Local()
	err := os.Chtimes(filePath, time.Time{}, now)
	return err
}

func verifyFileSize(expectedFileSize int64, pathToVerify string) error {
	fileInfo, err := os.Stat(pathToVerify)
	if err != nil {
		return err
	}
	fileSize := fileInfo.Size()
	if fileSize == expectedFileSize {
		return nil
	}
	return fmt.Errorf("%s: Expected size %d found size %d", pathToVerify, expectedFileSize, fileSize)
}

func verifySha256(expectedSha256 string, pathToVerify string) error {
	fileToVerify, err := os.Open(pathToVerify)
	if err != nil {
		return err
	}
	defer fileToVerify.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, fileToVerify); err != nil {
		return err
	}
	sha256String := hex.EncodeToString(hash.Sum(nil))
	if sha256String == expectedSha256 {
		return nil
	}
	return fmt.Errorf("%s: Expected SHA-256 %s found SHA-256 %s", pathToVerify, expectedSha256, sha256String)
}
