package terraform

import (
	"errors"
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

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

func (m Model) Merge(other Model) Model {
	mergedVars := map[string]interface{}{}
	for key, value := range m.Vars {
		mergedVars[key] = value
	}
	for key, value := range other.Vars {
		mergedVars[key] = value
	}
	m.Vars = mergedVars
	if other.Source != "" {
		m.Source = other.Source
	}
	if other.VarFile != "" {
		m.VarFile = other.VarFile
	}

	return m
}

func (m *Model) ParseVarsFromFile() error {
	terraformVars := map[string]interface{}{}
	for key, value := range m.Vars {
		terraformVars[key] = value
	}

	if m.VarFile != "" {
		fileContents, readErr := ioutil.ReadFile(m.VarFile)
		if readErr != nil {
			return fmt.Errorf("Failed to read TerraformVarFile at '%s': %s", m.VarFile, readErr)
		}

		fileVars := map[string]interface{}{}
		readErr = yaml.Unmarshal(fileContents, &fileVars)
		if readErr != nil {
			return fmt.Errorf("Failed to parse TerraformVarFile at '%s': %s", m.VarFile, readErr)
		}

		for key, value := range fileVars {
			terraformVars[key] = value
		}
	}

	m.Vars = terraformVars

	return nil
}
