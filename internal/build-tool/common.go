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
	"time"

	"github.com/schollz/progressbar/v3"
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
		return fmt.Errorf("Response status: %s", response.Status)
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

func ensureDownloadDirExists(downloadDir string) error {

	err := os.MkdirAll(downloadDir, 0750)
	return err
}

// Filter files by their names
type fileFilter func(string) bool

// Transform file names before writing them
type fileTransformer func(string) string

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
				return fmt.Errorf("Directory in archive has unexpected path: %s", next.Name)
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
				return fmt.Errorf("File in archive has unexpected path: %s", next.Name)
			}
		} else {
			return fmt.Errorf("Unexpected type in tar header: %s (%s)", next.Typeflag, tgzArchive)
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

func fileExistsAtPath(filePath string) (bool, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return fileInfo != nil, nil
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
