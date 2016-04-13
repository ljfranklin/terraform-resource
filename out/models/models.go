package models

import (
	"errors"
	"fmt"

	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/terraform"
)

type OutRequest struct {
	Source Source `json:"source"`
	Params Params `json:"params"`
}

type OutResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata"`
}

type Version struct {
	Version string `json:"version"`
}

type Metadata []MetadataField

type MetadataField struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

type Source struct {
	Storage   storage.Model   `json:"storage"`
	Terraform terraform.Model `json:"terraform"`
}

type Params struct {
	Terraform terraform.Model `json:"terraform"`
	Action    string          `json:"action,omitempty"` // optional
}

const (
	DestroyAction = "destroy"
)

func (r OutRequest) TerraformSource() string {
	if r.Params.Terraform.Source != "" {
		return r.Params.Terraform.Source
	}
	return r.Source.Terraform.Source
}

func (r OutRequest) Validate() error {
	errMsg := ""
	if err := r.Source.Storage.Validate(); err != nil {
		errMsg += fmt.Sprintf("%s\n", err)
	}

	if r.TerraformSource() == "" {
		errMsg += "Must specify either `put.params.terraform_source` or `source.terraform_source`\n"
	}

	if len(errMsg) > 0 {
		return errors.New(errMsg)
	}
	return nil
}
