package models

type OutRequest struct {
	Source Source    `json:"source"`
	Params OutParams `json:"params"`
}

type OutResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata"`
}

type OutParams struct {
	EnvName            string `json:"env_name"`
	EnvNameFile        string `json:"env_name_file"`
	GenerateRandomName bool   `json:"generate_random_name"`
	PlanOnly           bool   `json:"plan_only,omitempty"` // optional
	PlanRun            bool   `json:"plan_run,omitempty"`  // optional
	Action             string `json:"action,omitempty"`    // optional
	Terraform
}

const (
	DestroyAction = "destroy"
)
