package models

type InRequest struct {
	Source  Source   `json:"source"`
	Version Version  `json:"version,omitempty"` // absent on initial request
	Params  InParams `json:"params,omitempty"`  // used to specify 'destroy' action
}

type InResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata"`
}

type InParams struct {
	Action             string `json:"action,omitempty"`           // optional
	OutputStatefile    bool   `json:"output_statefile,omitempty"` // optional
	OutputJSONPlanfile bool   `json:"output_planfile,omitempty"`  // optional
	Terraform
}

type TfState struct {
	Outputs map[string]map[string]interface{} `json:"outputs,omitempty"`
}
