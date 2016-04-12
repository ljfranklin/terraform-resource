package models

import (
	"fmt"
	"strings"
)

type Version struct {
	Version string `json:"version"`
}

type InRequest struct {
	Source  Source  `json:"source"`
	Version Version `json:"version,omitempty"` // absent on initial request
	Params  Params  `json:"params,omitempty"`  // used to specify 'destroy' action
}

func (r InRequest) Validate() error {
	if err := r.Source.Storage.Validate(); err != nil {
		return err
	}
	return nil
}

type InResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata"`
}

type OutRequest struct {
	Source Source `json:"source"`
	Params Params `json:"params"`
}

func (r OutRequest) Validate() error {
	if err := r.Source.Storage.Validate(); err != nil {
		return err
	}
	return nil
}

type OutResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata"`
}

type Metadata []MetadataField

type MetadataField struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

type Source struct {
	Storage Storage `json:"storage"`

	TerraformSource string        `json:"terraform_source"`
	TerraformVars   TerraformVars `json:"terraform_vars"`
}

type Storage struct {
	Driver string `json:"driver"`

	// S3 driver
	Bucket          string `json:"bucket"`
	Key             string `json:"key"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	RegionName      string `json:"region_name,omitempty"` // optional
}

func (s Storage) Validate() error {

	knownDrivers := []string{
		"",
		S3Driver,
	}
	isUnknownDriver := true
	for _, driver := range knownDrivers {
		if driver == s.Driver {
			isUnknownDriver = false
			break
		}
	}
	if isUnknownDriver {
		for i, value := range knownDrivers {
			knownDrivers[i] = fmt.Sprintf("'%s'", value)
		}
		return fmt.Errorf(
			"Unknown value for `source.storage.driver`: '%s', Supported driver values: %s",
			s.Driver,
			strings.Join(knownDrivers, ", "),
		)
	}

	missingFields := []string{}
	if s.Driver == "" || s.Driver == S3Driver {
		fieldPrefix := "source.storage"
		if s.Bucket == "" {
			missingFields = append(missingFields, fmt.Sprintf("%s.bucket", fieldPrefix))
		}
		if s.Key == "" {
			missingFields = append(missingFields, fmt.Sprintf("%s.key", fieldPrefix))
		}
		if s.AccessKeyID == "" {
			missingFields = append(missingFields, fmt.Sprintf("%s.access_key_id", fieldPrefix))
		}
		if s.SecretAccessKey == "" {
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

type Params struct {
	TerraformSource  string        `json:"terraform_source"`
	TerraformVars    TerraformVars `json:"terraform_vars,omitempty"`     // optional
	TerraformVarFile string        `json:"terraform_var_file,omitempty"` // optional
	Action           string        `json:"action,omitempty"`             // optional
}

type TerraformVars map[string]interface{}

const (
	S3Driver      = "s3"
	DestroyAction = "destroy"
)
