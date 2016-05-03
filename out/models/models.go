package models

import (
	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/terraform"
)

type OutRequest struct {
	Source Source `json:"source"`
	Params Params `json:"params"`
}

type OutResponse struct {
	Version  models.Version `json:"version"`
	Metadata Metadata       `json:"metadata"`
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
	EnvName            string          `json:"env_name"`
	EnvNameFile        string          `json:"env_name_file"`
	GenerateRandomName bool            `json:"generate_random_name"`
	Terraform          terraform.Model `json:"terraform"`
	Action             string          `json:"action,omitempty"` // optional
}

const (
	DestroyAction = "destroy"
)
