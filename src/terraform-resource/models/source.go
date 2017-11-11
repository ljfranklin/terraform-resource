package models

import (
	"errors"
	"terraform-resource/storage"
)

type Source struct {
	Terraform
	Storage             storage.Model `json:"storage,omitempty"`               // optional
	MigratedFromStorage storage.Model `json:"migrated_from_storage,omitempty"` // optional
	EnvName             string        `json:"env_name,omitempty"`              // optional
}

func (s Source) Validate() error {
	if s.Storage != (storage.Model{}) && s.Terraform.BackendType != "" {
		return errors.New("Cannot specify both `backend_type` and `storage`. If you have existing environments in `storage`, rename `storage` to `migrated_from_storage` to have the resource move those environments into the Backend.")
	}

	if s.MigratedFromStorage != (storage.Model{}) && s.Storage != (storage.Model{}) {
		return errors.New("Cannot specify both `migrated_from_storage` and `storage`.")
	}

	if s.MigratedFromStorage != (storage.Model{}) && s.Terraform.BackendType == "" {
		return errors.New("Must specify `backend_type` and `backend_config` when using `migrated_from_storage`.")
	}

	if err := s.Terraform.Validate(); err != nil {
		return err
	}

	if s.Storage != (storage.Model{}) {
		if err := s.Storage.Validate(); err != nil {
			return err
		}
	}

	if s.MigratedFromStorage != (storage.Model{}) {
		if err := s.MigratedFromStorage.Validate(); err != nil {
			return err
		}
	}

	return nil
}
