package models

import "terraform-resource/storage"

type Source struct {
	Storage storage.Model `json:"storage"`
	Terraform
}
