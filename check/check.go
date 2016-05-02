package check

import (
	"fmt"
	"time"

	inModels "github.com/ljfranklin/terraform-resource/in/models"
	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/storage"
)

type Runner struct{}

func (r Runner) Run(req inModels.InRequest) ([]models.Version, error) {
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

	storageVersion, err := storageDriver.LatestVersion()
	if err != nil {
		return nil, fmt.Errorf("Failed to check storage backend for latest version: %s", err)
	}

	resp := []models.Version{}
	if storageVersion.IsZero() == false && storageVersion.LastModified.After(currentVersionTime) {
		version := models.NewVersion(storageVersion)
		resp = append(resp, version)
	}

	return resp, nil
}
