package models

import "terraform-resource/storage"

type Source struct {
	Storage   storage.Model `json:"storage"`
	EnvName   string        `json:"env_name,omitempty"` // optional
	Terraform               // optional
}
