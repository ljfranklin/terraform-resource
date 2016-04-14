package models

import (
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
