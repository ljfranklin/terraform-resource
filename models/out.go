package models

import "github.com/ljfranklin/terraform-resource/terraform"

type OutRequest struct {
	Source Source    `json:"source"`
	Params OutParams `json:"params"`
}

type OutResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata"`
}

type OutParams struct {
	EnvName            string          `json:"env_name"`
	EnvNameFile        string          `json:"env_name_file"`
	GenerateRandomName bool            `json:"generate_random_name"`
	Terraform          terraform.Model `json:"terraform"`
	Action             string          `json:"action,omitempty"` // optional
}

const (
	DestroyAction = "destroy"
)
