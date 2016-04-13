package terraform

import "errors"

type Model struct {
	Source  string                 `json:"source"`
	Vars    map[string]interface{} `json:"vars,omitempty"`     // optional
	VarFile string                 `json:"var_file,omitempty"` // optional
}

func (m Model) Validate() error {
	if m.Source == "" {
		return errors.New("Missing required terraform field 'source'")
	}

	return nil
}
