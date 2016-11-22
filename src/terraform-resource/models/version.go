package models

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"terraform-resource/storage"
)

const (
	// e.g. "2006-01-02T15:04:05Z"
	TimeFormat = time.RFC3339
)

type Version struct {
	LastModified string `json:"last_modified"`
	EnvName      string `json:"env_name"`
	PlanOnly     string `json:"plan_only,omitempty"` //optional
}

func NewVersion(storageVersion storage.Version) Version {
	basename := path.Base(storageVersion.StateFile)
	envName := strings.TrimSuffix(basename, filepath.Ext(basename))
	return Version{
		LastModified: storageVersion.LastModified.Format(TimeFormat),
		EnvName:      envName,
	}
}

func (r Version) Validate() error {
	missingFields := []string{}
	fieldPrefix := "version"
	if r.LastModified == "" {
		missingFields = append(missingFields, fmt.Sprintf("%s.last_modified", fieldPrefix))
	}
	if r.EnvName == "" {
		missingFields = append(missingFields, fmt.Sprintf("%s.env_name", fieldPrefix))
	}

	if len(missingFields) > 0 {
		for i, value := range missingFields {
			missingFields[i] = fmt.Sprintf("'%s'", value)
		}
		return fmt.Errorf("Missing fields: %s", strings.Join(missingFields, ", "))
	}

	_, err := time.Parse(TimeFormat, r.LastModified)
	if err != nil {
		return fmt.Errorf("LastModified field is in invalid format: %s", err)
	}

	return nil
}

func (r Version) IsZero() bool {
	return r == Version{}
}

func (r Version) IsPlan() bool {
	return r.PlanOnly == "true"
}

func (r Version) LastModifiedTime() time.Time {
	// assumes Validate has already been called
	lastModified, _ := time.Parse(TimeFormat, r.LastModified)
	return lastModified
}
