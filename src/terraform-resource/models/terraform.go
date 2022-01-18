package models

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	yamlConverter "github.com/ghodss/yaml"
	yaml "gopkg.in/yaml.v2"
)

type Terraform struct {
	Source                string                 `json:"terraform_source"`
	Vars                  map[string]interface{} `json:"vars,omitempty"`                  // optional
	VarFiles              []string               `json:"var_files,omitempty"`             // optional
	Env                   map[string]string      `json:"env,omitempty"`                   // optional
	DeleteOnFailure       bool                   `json:"delete_on_failure,omitempty"`     // optional
	PlanOnly              bool                   `json:"plan_only,omitempty"`             // optional
	PlanRun               bool                   `json:"plan_run,omitempty"`              // optional
	OutputModule          string                 `json:"output_module,omitempty"`         // optional
	ImportFiles           []string               `json:"import_files,omitempty"`          // optional
	OverrideFiles         []string               `json:"override_files,omitempty"`        // optional
	ModuleOverrideFiles   []map[string]string    `json:"module_override_files,omitempty"` // optional
	PluginDir             string                 `json:"plugin_dir,omitempty"`            // optional
	BackendType           string                 `json:"backend_type,omitempty"`          // optional
	BackendConfig         map[string]interface{} `json:"backend_config,omitempty"`        // optional
	Parallelism           int                    `json:"parallelism,omitempty"`           // optional
	PrivateKey            string                 `json:"private_key,omitempty"`
	PlanFileLocalPath     string                 `json:"-"` // not specified pipeline
	JSONPlanFileLocalPath string                 `json:"-"` // not specified pipeline
	PlanFileRemotePath    string                 `json:"-"` // not specified pipeline
	StateFileLocalPath    string                 `json:"-"` // not specified pipeline
	StateFileRemotePath   string                 `json:"-"` // not specified pipeline
	Imports               map[string]string      `json:"-"` // not specified pipeline
	ConvertedVarFiles     []string               `json:"-"` // not specified pipeline
	DownloadPlugins       bool                   `json:"-"` // not specified pipeline
}

const (
	PlanContent     = "plan_content"
	PlanContentJSON = "plan_content_json"
)

func (m Terraform) Validate() error {
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

	if other.PlanFileLocalPath != "" {
		m.PlanFileLocalPath = other.PlanFileLocalPath
	}

	if other.JSONPlanFileLocalPath != "" {
		m.JSONPlanFileLocalPath = other.JSONPlanFileLocalPath
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

	if other.PrivateKey != "" {
		m.PrivateKey = other.PrivateKey
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

	if other.ImportFiles != nil {
		m.ImportFiles = other.ImportFiles
	}

	if other.OverrideFiles != nil {
		m.OverrideFiles = other.OverrideFiles
	}

	if other.ModuleOverrideFiles != nil {
		m.ModuleOverrideFiles = other.ModuleOverrideFiles
	}

	if other.PluginDir != "" {
		m.PluginDir = other.PluginDir
	}

	if other.Imports != nil {
		m.Imports = other.Imports
	}

	if other.BackendType != "" {
		m.BackendType = other.BackendType
	}

	if other.BackendConfig != nil {
		m.BackendConfig = other.BackendConfig
	}

	if other.Parallelism > 0 {
		m.Parallelism = other.Parallelism
	}

	return m
}

// The resource supports input files in JSON, YAML, and HCL formats.
// Terraform supports JSON and HCL but not YAML.
// This method converts all YAML files to JSON and writes Vars to the
// first file to ensure precedence rules are respected.
func (m *Terraform) ConvertVarFiles(tmpDir string) error {
	vars, err := m.collectVars()
	if err != nil {
		return err
	}

	varsContents, err := yaml.Marshal(vars)
	if err != nil {
		return err
	}

	varsFile, err := m.writeJSONFile(tmpDir, varsContents)
	if err != nil {
		return err
	}
	m.ConvertedVarFiles = append(m.ConvertedVarFiles, varsFile)

	for _, inputVarFile := range m.VarFiles {
		fileContents, err := ioutil.ReadFile(inputVarFile)
		if err != nil {
			return err
		}
		var outputVarFile string
		if strings.HasSuffix(inputVarFile, ".tfvars") {
			outputVarFile, err = m.writeToTempFile(tmpDir, fileContents)
			if err != nil {
				return err
			}
		} else {
			outputVarFile, err = m.writeJSONFile(tmpDir, fileContents)
			if err != nil {
				return err
			}
		}
		m.ConvertedVarFiles = append(m.ConvertedVarFiles, outputVarFile)
	}

	return nil
}

func (m *Terraform) writeJSONFile(tmpDir string, contents []byte) (string, error) {
	// avoids marshalling errors around map[interface{}]interface{}
	jsonFileContents, err := yamlConverter.YAMLToJSON(contents)
	if err != nil {
		return "", err
	}

	varsFile, err := ioutil.TempFile(tmpDir, "*vars-file.tfvars.json")
	if err != nil {
		return "", err
	}
	if _, err := varsFile.Write(jsonFileContents); err != nil {
		return "", err
	}
	if err := varsFile.Close(); err != nil {
		return "", err
	}

	return varsFile.Name(), nil
}

func (m *Terraform) writeToTempFile(tmpDir string, contents []byte) (string, error) {
	varsFile, err := ioutil.TempFile(tmpDir, "*.tfvars")
	if err != nil {
		return "", err
	}
	if _, err := varsFile.Write(contents); err != nil {
		return "", err
	}
	if err := varsFile.Close(); err != nil {
		return "", err
	}

	return varsFile.Name(), nil
}

func (m *Terraform) ParseImportsFromFile() error {
	if m.Imports == nil {
		m.Imports = map[string]string{}
	}

	if m.ImportFiles != nil {
		for _, file := range m.ImportFiles {
			fileContents, readErr := ioutil.ReadFile(file)
			if readErr != nil {
				return fmt.Errorf("Failed to read Terraform ImportsFile at '%s': %s", file, readErr)
			}

			fileImports := map[string]string{}
			readErr = yaml.Unmarshal(fileContents, &fileImports)
			if readErr != nil {
				return fmt.Errorf("Failed to parse Terraform ImportsFile at '%s': %s", file, readErr)
			}

			for key, value := range fileImports {
				m.Imports[key] = value
			}
		}
	}

	return nil
}

func (m *Terraform) collectVars() (map[string]interface{}, error) {
	terraformVars := map[string]interface{}{}

	for key, value := range m.Vars {
		path, ok := value.(string)
		if ok {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				terraformVars[key] = value
			} else {
				fileContents, readErr := ioutil.ReadFile(path)
				if readErr != nil {
					return nil, fmt.Errorf("Failed to read Terraform Vars file at '%s': %s", path, readErr)
				}
				terraformVars[key] = string(fileContents)
			}
		} else {
			terraformVars[key] = value
		}
	}

	return terraformVars, nil
}
