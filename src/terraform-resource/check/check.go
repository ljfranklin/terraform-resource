package check

import (
	"fmt"
	"io"
	"time"

	"terraform-resource/workspaces"

	"terraform-resource/models"
	"terraform-resource/storage"
	"terraform-resource/terraform"
)

type Runner struct {
	LogWriter io.Writer
}

func (r Runner) Run(req models.InRequest) ([]models.Version, error) {
	if err := req.Source.Validate(); err != nil {
		return []models.Version{}, err
	}

	if req.Source.BackendType != "" && req.Source.MigratedFromStorage != (storage.Model{}) {
		if req.Version.IsZero() && req.Source.EnvName == "" {
			// Triggering on new versions is only supported in single-env mode:
			// - expensive to check for changes across all statefiles
			// - triggering on changes to any environment doesn't seem very useful
			return []models.Version{}, nil
		}

		backendVersions, err := r.runWithBackend(req)
		if err != nil {
			return []models.Version{}, err
		}

		if len(backendVersions) > 0 {
			return backendVersions, nil
		}

		req.Source.Storage = req.Source.MigratedFromStorage
		return r.runWithLegacyStorage(req)
	} else if req.Source.BackendType != "" {
		return r.runWithBackend(req)
	}
	return r.runWithLegacyStorage(req)
}

func (r Runner) runWithBackend(req models.InRequest) ([]models.Version, error) {
	// TODO: do we need to handle IsPlan() versions?
	if req.Version.IsZero() && req.Source.EnvName == "" {
		// Triggering on new versions is only supported in single-env mode:
		// - expensive to check for changes across all statefiles
		// - triggering on changes to any environment doesn't seem very useful
		return []models.Version{}, nil
	}

	if req.Version.IsZero() == false {
		if err := req.Version.Validate(); err != nil {
			return nil, fmt.Errorf("Failed to validate provided version: %s", err)
		}
	}

	terraformModel := req.Source.Terraform
	if err := terraformModel.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate terraform Model: %s", err)
	}

	client := terraform.NewClient(
		terraformModel,
		r.LogWriter,
	)

	workspaces := workspaces.New(client)

	var targetEnvName string
	if req.Source.EnvName != "" {
		targetEnvName = req.Source.EnvName
	} else {
		targetEnvName = req.Version.EnvName
	}
	latestVersion, err := workspaces.LatestVersionForEnv(targetEnvName)
	if err != nil {
		return nil, fmt.Errorf("Failed to check backend for latest version of '%s': %s", targetEnvName, err)
	}

	resp := []models.Version{}
	// TODO: need to handle cases where requested version has LastModified?
	if latestVersion.IsZero() == false && latestVersion.Serial >= req.Version.Serial {
		resp = append(resp, latestVersion)
	}

	return resp, nil
}

func (r Runner) runWithLegacyStorage(req models.InRequest) ([]models.Version, error) {
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

	stateFile := storage.StateFile{
		StorageDriver: storageDriver,
	}

	storageVersion, err := stateFile.LatestVersion()
	if err != nil {
		return nil, fmt.Errorf("Failed to check storage backend for latest version: %s", err)
	}

	resp := []models.Version{}
	if storageVersion.IsZero() == false && !storageVersion.LastModified.Before(currentVersionTime) {
		version := models.NewVersionFromLegacyStorage(storageVersion)
		resp = append(resp, version)
	}

	return resp, nil
}
