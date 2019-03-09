package storage

import (
	"fmt"
	"os"
	"time"
)

type PlanFile struct {
	LocalPath     string
	RemotePath    string
	StorageDriver Storage
}

func (p PlanFile) Exists() (bool, error) {
	version, err := p.StorageDriver.Version(p.RemotePath)
	if err != nil {
		return false, fmt.Errorf("Failed to check for existing plan file from '%s': %s", p.RemotePath, err)
	}
	return version.IsZero() == false, nil
}

func (p PlanFile) LatestVersion() (Version, error) {
	return p.StorageDriver.LatestVersion(`.*\.tfplan$`)
}

func (p PlanFile) Download() (Version, error) {
	planFile, createErr := os.Create(p.LocalPath)
	if createErr != nil {
		return Version{}, fmt.Errorf("Failed to create plan file at '%s': %s", p.LocalPath, createErr)
	}
	defer planFile.Close()

	version, err := p.StorageDriver.Download(p.RemotePath, planFile)
	if err != nil {
		return Version{}, err
	}
	planFile.Close()

	return version, nil
}

func (p PlanFile) Upload() (Version, error) {
	planFile, err := os.Open(p.LocalPath)
	if err != nil {
		return Version{}, fmt.Errorf("Failed to open plan file at '%s'", p.LocalPath)
	}
	defer planFile.Close()

	_, err = p.StorageDriver.Upload(p.RemotePath, planFile)
	if err != nil {
		return Version{}, fmt.Errorf("Failed to upload plan file: %s", err)
	}

	version, err := p.StorageDriver.Version(p.RemotePath)
	if err != nil {
		return Version{}, fmt.Errorf("Failed to retrieve version from '%s': %s", p.RemotePath, err)
	}
	if version.IsZero() {
		return Version{}, fmt.Errorf("Couldn't find plan file at: %s", p.RemotePath)
	}

	return version, nil
}

func (p PlanFile) Delete() (Version, error) {
	if err := p.StorageDriver.Delete(p.RemotePath); err != nil {
		return Version{}, fmt.Errorf("Failed to delete plan file: %s", err)
	}

	// use current time rather than plan file LastModified time
	version := Version{
		LastModified: time.Now().UTC(),
		PlanFile:     p.RemotePath,
	}
	return version, nil
}
