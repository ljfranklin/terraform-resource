package models

import (
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

type Terraform struct {
	Source              string                 `json:"terraform_source"`
	Vars                map[string]interface{} `json:"vars,omitempty"`              // optional
	VarFile             string                 `json:"var_file,omitempty"`          // optional
	VarFiles            []string               `json:"var_files,omitempty"`         // optional
	Env                 map[string]string      `json:"env,omitempty"`               // optional
	DeleteOnFailure     bool                   `json:"delete_on_failure,omitempty"` // optional
	PlanOnly            bool                   `json:"plan_only,omitempty"`         // optional
	PlanRun             bool                   `json:"plan_run,omitempty"`          // optional
	OutputModule        string                 `json:"output_module,omitempty"`     // optional
	PlanFileLocalPath   string                 `json:"-"`                           // not specified pipeline
	PlanFileRemotePath  string                 `json:"-"`                           // not specified pipeline
	StateFileLocalPath  string                 `json:"-"`                           // not specified pipeline
	StateFileRemotePath string                 `json:"-"`                           // not specified pipeline
}

func (m Terraform) Validate() error {
	missingFields := []string{}
	if m.StateFileLocalPath == "" {
		missingFields = append(missingFields, "state_file_local_path")
	}
	if m.StateFileRemotePath == "" {
		missingFields = append(missingFields, "state_file_remote_path")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("Missing required terraform fields: %s", strings.Join(missingFields, ", "))
	}
	return nil
}

func (m Terraform) Merge(other Terraform) Terraform {
	mergedVars := map[string]interface{}{}
	for key, value := range m.Vars {
		mergedVars[key] = value
	}
	for key, value := range other.Vars {
		mergedVars[key] = value
	}
	m.Vars = mergedVars

	mergedEnv := map[string]string{}
	for key, value := range m.Env {
		mergedEnv[key] = value
	}
	for key, value := range other.Env {
		mergedEnv[key] = value
	}
	m.Env = mergedEnv

	if other.Source != "" {
		m.Source = other.Source
	}

	if other.VarFiles != nil {
		m.VarFiles = other.VarFiles
	}

	if other.VarFile != "" {
		m.VarFile = other.VarFile
	}

	if other.PlanFileLocalPath != "" {
		m.PlanFileLocalPath = other.PlanFileLocalPath
	}

	if other.PlanFileRemotePath != "" {
		m.PlanFileRemotePath = other.PlanFileRemotePath
	}

	if other.StateFileLocalPath != "" {
		m.StateFileLocalPath = other.StateFileLocalPath
	}

	if other.StateFileRemotePath != "" {
		m.StateFileRemotePath = other.StateFileRemotePath
	}

	if other.PlanOnly {
		m.PlanOnly = true
	}

	if other.OutputModule != "" {
		m.OutputModule = other.OutputModule
	}

	if other.PlanRun {
		m.PlanRun = true
	}

	if other.DeleteOnFailure {
		m.DeleteOnFailure = true
	}

	return m
}

func (m *Terraform) ParseVarsFromFiles() error {
	terraformVars := map[string]interface{}{}
	for key, value := range m.Vars {
		terraformVars[key] = value
	}

	if m.VarFile != "" {
		newVars, err := m.parseVarsFromFiles(m.VarFile)
		if err != nil {
			return err
		}

		for key, value := range newVars {
			terraformVars[key] = value
		}
	}

	if m.VarFiles != nil {
		for _, varFile := range m.VarFiles {
			newVars, err := m.parseVarsFromFiles(varFile)
			if err != nil {
				return err
			}

			for key, value := range newVars {
				terraformVars[key] = value
			}
		}
	}

	m.Vars = terraformVars

	return nil
}

func (m *Terraform) parseVarsFromFiles(filepath string) (map[string]interface{}, error) {
	terraformVars := map[string]interface{}{}

	fileContents, readErr := ioutil.ReadFile(filepath)
	if readErr != nil {
		return nil, fmt.Errorf("Failed to read TerraformVarFile at '%s': %s", filepath, readErr)
	}

	fileVars := map[string]interface{}{}
	readErr = yaml.Unmarshal(fileContents, &fileVars)
	if readErr != nil {
		return nil, fmt.Errorf("Failed to parse TerraformVarFile at '%s': %s", filepath, readErr)
	}

	for key, value := range fileVars {
		terraformVars[key] = value
	}

	return terraformVars, nil
}
