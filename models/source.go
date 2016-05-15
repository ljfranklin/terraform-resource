package models

import (
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/terraform"
)

type Source struct {
	Storage   storage.Model   `json:"storage"`
	Terraform terraform.Model `json:"terraform"`
}
