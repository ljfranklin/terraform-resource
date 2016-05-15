package models

import "github.com/ljfranklin/terraform-resource/storage"

type Source struct {
	Storage storage.Model `json:"storage"`
	Terraform
}
