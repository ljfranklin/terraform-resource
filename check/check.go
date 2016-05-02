package check

import (
	"fmt"
	"time"

	"github.com/ljfranklin/terraform-resource/in/models"
	"github.com/ljfranklin/terraform-resource/storage"
)

type Runner struct{}

func (r Runner) Run(req models.InRequest) ([]storage.Version, error) {
	currentVersionTime := time.Time{}
	if req.Version.IsZero() == false {
		if err := req.Version.Validate(); err != nil {
			return nil, fmt.Errorf("Failed to validate provided version: %s", err)
		}
		currentVersionTime = req.Version.LastModifiedTime()
	}

	storageModel := req.Source.Storage
	if err := storageModel.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	version, err := storageDriver.LatestVersion()
	if err != nil {
		return nil, fmt.Errorf("Failed to check storage backend for latest version: %s", err)
	}

	resp := []storage.Version{}
	if version.IsZero() == false && version.LastModifiedTime().After(currentVersionTime) {
		resp = append(resp, version)
	}

	return resp, nil
}
