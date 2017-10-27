package models

import "terraform-resource/storage"

type Source struct {
	Terraform
	Storage             storage.Model `json:"storage,omitempty"`               // optional
	MigratedFromStorage storage.Model `json:"migrated_from_storage,omitempty"` // optional
	EnvName             string        `json:"env_name,omitempty"`              // optional
}
