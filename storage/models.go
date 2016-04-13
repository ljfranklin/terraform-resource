package storage

import (
	"fmt"
	"strings"
)

const (
	S3Driver = "s3"
)

type Model struct {
	Driver string `json:"driver"`

	// S3 driver
	Bucket          string `json:"bucket"`
	Key             string `json:"key"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	RegionName      string `json:"region_name,omitempty"` // optional
}

func (m Model) Validate() error {

	knownDrivers := []string{
		"",
		S3Driver,
	}
	isUnknownDriver := true
	for _, driver := range knownDrivers {
		if driver == m.Driver {
			isUnknownDriver = false
			break
		}
	}
	if isUnknownDriver {
		for i, value := range knownDrivers {
			knownDrivers[i] = fmt.Sprintf("'%s'", value)
		}
		return fmt.Errorf(
			"Unknown value for `storage.driver`: '%s', Supported driver values: %s",
			m.Driver,
			strings.Join(knownDrivers, ", "),
		)
	}

	missingFields := []string{}
	if m.Driver == "" || m.Driver == S3Driver {
		fieldPrefix := "storage"
		if m.Bucket == "" {
			missingFields = append(missingFields, fmt.Sprintf("%s.bucket", fieldPrefix))
		}
		if m.Key == "" {
			missingFields = append(missingFields, fmt.Sprintf("%s.key", fieldPrefix))
		}
		if m.AccessKeyID == "" {
			missingFields = append(missingFields, fmt.Sprintf("%s.access_key_id", fieldPrefix))
		}
		if m.SecretAccessKey == "" {
			missingFields = append(missingFields, fmt.Sprintf("%s.secret_access_key", fieldPrefix))
		}
	}

	if len(missingFields) > 0 {
		for i, value := range missingFields {
			missingFields[i] = fmt.Sprintf("'%s'", value)
		}
		return fmt.Errorf("Missing fields: %s", strings.Join(missingFields, ", "))
	}
	return nil
}
