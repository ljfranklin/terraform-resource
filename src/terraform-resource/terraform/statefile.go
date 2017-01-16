package terraform

import (
	"fmt"
	"os"
	"strings"
	"terraform-resource/storage"
	"time"
)

type StateFile struct {
	LocalPath     string
	RemotePath    string
	StorageDriver storage.Storage
	isTainted     bool
}

func (s StateFile) Exists() (bool, error) {
	version, err := s.StorageDriver.Version(s.RemotePath)
	if err != nil {
		return false, fmt.Errorf("Failed to check for existing state file from '%s': %s", s.RemotePath, err)
	}
	return version.IsZero() == false, nil
}

func (s StateFile) ExistsAsTainted() (bool, error) {
	version, err := s.StorageDriver.Version(s.taintedRemotePath())
	if err != nil {
		return false, fmt.Errorf("Failed to check for existing state file from '%s': %s", s.RemotePath, err)
	}
	return version.IsZero() == false, nil
}

func (s StateFile) ConvertToTainted() StateFile {
	return StateFile{
		LocalPath:     s.LocalPath,
		RemotePath:    s.taintedRemotePath(),
		StorageDriver: s.StorageDriver,
		isTainted:     true,
	}
}

func (s StateFile) ConvertFromTainted() StateFile {
	return StateFile{
		LocalPath:     s.LocalPath,
		RemotePath:    s.untaintedRemotePath(),
		StorageDriver: s.StorageDriver,
		isTainted:     false,
	}
}

func (s StateFile) LatestVersion() (storage.Version, error) {
	return s.StorageDriver.LatestVersion(`.*\.tfstate$`)
}

func (s StateFile) Download() (storage.Version, error) {
	stateFile, createErr := os.Create(s.LocalPath)
	if createErr != nil {
		return storage.Version{}, fmt.Errorf("Failed to create state file at '%s': %s", s.LocalPath, createErr)
	}
	defer stateFile.Close()

	version, err := s.StorageDriver.Download(s.RemotePath, stateFile)
	if err != nil {
		return storage.Version{}, err
	}
	stateFile.Close()

	return version, nil
}

func (s StateFile) Upload() (storage.Version, error) {
	stateFile, err := os.Open(s.LocalPath)
	if err != nil {
		return storage.Version{}, fmt.Errorf("Failed to open state file at '%s'", s.LocalPath)
	}
	defer stateFile.Close()

	_, err = s.StorageDriver.Upload(s.RemotePath, stateFile)
	if err != nil {
		return storage.Version{}, fmt.Errorf("Failed to upload state file: %s", err)
	}

	// handle AWS eventual consistency errors
	retryAttempts := 5
	var version storage.Version
	for i := 0; i < retryAttempts; i++ {
		version, err = s.StorageDriver.Version(s.RemotePath)
		if err != nil {
			return storage.Version{}, fmt.Errorf("Failed to retrieve version from '%s': %s", s.RemotePath, err)
		}
		if !version.IsZero() {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if version.IsZero() {
		return storage.Version{}, fmt.Errorf("Couldn't find state file after %d retries at: %s", retryAttempts, s.RemotePath)
	}

	return version, nil
}

func (s StateFile) UploadTainted() error {
	if _, err := os.Stat(s.LocalPath); os.IsNotExist(err) {
		// no-op if local file doesn't exist
		return nil
	}

	stateFile, err := os.Open(s.LocalPath)
	if err != nil {
		return fmt.Errorf("Failed to open state file at '%s'", s.LocalPath)
	}
	defer stateFile.Close()

	_, err = s.StorageDriver.Upload(s.taintedRemotePath(), stateFile)
	if err != nil {
		return fmt.Errorf("Failed to upload tainted state file: %s", err)
	}

	return nil
}

func (s StateFile) Delete() (storage.Version, error) {
	if err := s.StorageDriver.Delete(s.RemotePath); err != nil {
		return storage.Version{}, fmt.Errorf("Failed to delete state file: %s", err)
	}

	// use current time rather than state file LastModified time
	version := storage.Version{
		LastModified: time.Now().UTC(),
		StateFile:    s.RemotePath,
	}
	return version, nil
}

func (s StateFile) IsTainted() bool {
	return s.isTainted
}

func (s StateFile) taintedRemotePath() string {
	if strings.HasSuffix(s.RemotePath, ".tainted") {
		return s.RemotePath
	}
	return fmt.Sprintf("%s.tainted", s.RemotePath)
}

func (s StateFile) untaintedRemotePath() string {
	return strings.TrimSuffix(s.RemotePath, ".tainted")
}
