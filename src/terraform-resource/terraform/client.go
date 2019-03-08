package terraform

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"terraform-resource/models"

	yamlConverter "github.com/ghodss/yaml"
	yaml "gopkg.in/yaml.v2"
)

//go:generate counterfeiter . Client

type Client interface {
	InitWithBackend() error
	InitWithoutBackend() error
	Apply() error
	Destroy() error
	Plan() error
	Output(string) (map[string]map[string]interface{}, error)
	OutputWithLegacyStorage() (map[string]map[string]interface{}, error)
	Version() (string, error)
	Import(string) error
	ImportWithLegacyStorage() error
	WorkspaceList() ([]string, error)
	WorkspaceNewFromExistingStateFile(string, string) error
	WorkspaceNewIfNotExists(string) error
	WorkspaceSelect(string) error
	WorkspaceDelete(string) error
	WorkspaceDeleteWithForce(string) error
	StatePull(string) ([]byte, error)
	CurrentStateVersion(string) (StateVersion, error)
	SavePlanToBackend(string) error
	GetPlanFromBackend(string) error
}

type client struct {
	model     models.Terraform
	logWriter io.Writer
}

type StateVersion struct {
	Serial  int
	Lineage string
}

func NewClient(model models.Terraform, logWriter io.Writer) Client {
	return &client{
		model:     model,
		logWriter: logWriter,
	}
}

func (c *client) InitWithBackend() error {
	if err := c.writeBackendOverride(c.model.Source); err != nil {
		return err
	}
	backendConfigPath, err := c.writeBackendConfig(c.model.Source)
	if err != nil {
		return err
	}

	initArgs := []string{
		"init",
		"-input=false",
		"-get=true",
		"-backend=true",
		fmt.Sprintf("-backend-config=%s", backendConfigPath),
	}
	if c.model.PluginDir != "" {
		initArgs = append(initArgs, fmt.Sprintf("-plugin-dir=%s", c.model.PluginDir))
	}

	initCmd := c.terraformCmd(initArgs, nil)
	var output []byte
	if output, err = initCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", err, output)
	}

	return nil
}

func (c *client) writeBackendConfig(outputDir string) (string, error) {
	configContents, err := json.Marshal(c.model.BackendConfig)
	if err != nil {
		return "", err
	}

	backendPath, err := filepath.Abs(path.Join(outputDir, "resource_backend_config.json"))
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(backendPath, configContents, 0755)
	if err != nil {
		return "", err
	}
	return backendPath, nil
}

func (c *client) writePlanProviderConfig(outputDir string, planContents []byte) error {
	encodedPlan := base64.StdEncoding.EncodeToString(planContents)
	escapedPlan, err := json.Marshal(encodedPlan)
	if err != nil {
		return err
	}

	configContents := []byte(fmt.Sprintf(`
resource "stateful_string" "plan_output" {
  desired = %s
}
output "plan_content" {
  sensitive = true
  value = "${stateful_string.plan_output.desired}"
}
`, escapedPlan))

	configPath, err := filepath.Abs(path.Join(outputDir, "resource_plan_config.tf"))
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(configPath, configContents, 0755)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) writeBackendOverride(outputDir string) error {
	backendPath := path.Join(outputDir, "resource_backend_override.tf")
	backendContent := fmt.Sprintf(`terraform { backend "%s" {} }`, c.model.BackendType)
	return ioutil.WriteFile(backendPath, []byte(backendContent), 0755)
}

func (c *client) InitWithoutBackend() error {
	if err := c.clearTerraformState(); err != nil {
		return err
	}

	initArgs := []string{
		"init",
		"-input=false",
		"-get=true",
		"-backend=false",
	}
	if c.model.PluginDir != "" {
		initArgs = append(initArgs, fmt.Sprintf("-plugin-dir=%s", c.model.PluginDir))
	}
	initCmd := c.terraformCmd(initArgs, nil)

	if output, err := initCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", err, output)
	}

	return nil
}

// necessary to switch from backend to non-backend in `migrated_from_storage` code paths
func (c *client) clearTerraformState() error {
	configPath := path.Join(c.model.Source, ".terraform")
	if err := os.RemoveAll(configPath); err != nil {
		return err
	}

	backendConfig := path.Join(c.model.Source, "resource_backend_override.tf")
	return os.RemoveAll(backendConfig)
}

func (c *client) Apply() error {
	applyArgs := []string{
		"apply",
		"-backup='-'",  // no need to backup state file
		"-input=false", // do not prompt for inputs
		"-auto-approve",
	}

	if c.model.PlanRun == false {
		varFile, err := c.writeVarFile()
		if err != nil {
			return err
		}
		defer os.RemoveAll(varFile)

		applyArgs = append(applyArgs, fmt.Sprintf("-var-file=%s", varFile))
		if c.model.StateFileLocalPath != "" {
			applyArgs = append(applyArgs, fmt.Sprintf("-state=%s", c.model.StateFileLocalPath))
		}
	} else {
		// only used in non-backend flow
		applyArgs = append(applyArgs, fmt.Sprintf("-state-out=%s", c.model.StateFileLocalPath))
	}

	if c.model.PlanRun {
		applyArgs = append(applyArgs, c.model.PlanFileLocalPath)
	}

	applyCmd := c.terraformCmd(applyArgs, nil)
	applyCmd.Stdout = c.logWriter
	applyCmd.Stderr = c.logWriter
	err := applyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c *client) Destroy() error {
	destroyArgs := []string{
		"destroy",
		"-backup='-'", // no need to backup state file
		"-force",      // do not prompt for confirmation
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
	}

	varFile, err := c.writeVarFile()
	if err != nil {
		return err
	}
	defer os.RemoveAll(varFile)

	destroyArgs = append(destroyArgs, fmt.Sprintf("-var-file=%s", varFile))

	destroyCmd := c.terraformCmd(destroyArgs, nil)
	destroyCmd.Stdout = c.logWriter
	destroyCmd.Stderr = c.logWriter
	err = destroyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c *client) Plan() error {
	planArgs := []string{
		"plan",
		"-input=false", // do not prompt for inputs
		fmt.Sprintf("-out=%s", c.model.PlanFileLocalPath),
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
	}

	varFile, err := c.writeVarFile()
	if err != nil {
		return err
	}
	defer os.RemoveAll(varFile)

	planArgs = append(planArgs, fmt.Sprintf("-var-file=%s", varFile))

	planCmd := c.terraformCmd(planArgs, nil)
	planCmd.Stdout = c.logWriter
	planCmd.Stderr = c.logWriter
	err = planCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c *client) Output(envName string) (map[string]map[string]interface{}, error) {
	outputArgs := []string{
		"output",
		"-json",
	}
	if c.model.OutputModule != "" {
		outputArgs = append(outputArgs, fmt.Sprintf("-module=%s", c.model.OutputModule))
	}

	outputCmd := c.terraformCmd(outputArgs, []string{
		fmt.Sprintf("TF_WORKSPACE=%s", envName),
	})

	rawOutput, err := outputCmd.Output()
	if err != nil {
		// TF CLI currently doesn't provide a nice way to detect an empty set of outputs
		// https://github.com/hashicorp/terraform/issues/11696
		if exitErr, ok := err.(*exec.ExitError); ok && strings.Contains(string(exitErr.Stderr), "no outputs defined") {
			rawOutput = []byte("{}")
		} else {
			return nil, fmt.Errorf("Failed to retrieve output.\nError: %s\nOutput: %s", err, rawOutput)
		}
	}

	tfOutput := map[string]map[string]interface{}{}
	if err = json.Unmarshal(rawOutput, &tfOutput); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal JSON output.\nError: %s\nOutput: %s", err, rawOutput)
	}

	return tfOutput, nil
}

func (c *client) OutputWithLegacyStorage() (map[string]map[string]interface{}, error) {
	outputArgs := []string{
		"output",
		"-json",
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
	}

	if c.model.OutputModule != "" {
		outputArgs = append(outputArgs, fmt.Sprintf("-module=%s", c.model.OutputModule))
	}

	outputCmd := c.terraformCmd(outputArgs, nil)

	rawOutput, err := outputCmd.Output()
	if err != nil {
		// TF CLI currently doesn't provide a nice way to detect an empty set of outputs
		// https://github.com/hashicorp/terraform/issues/11696
		if exitErr, ok := err.(*exec.ExitError); ok && strings.Contains(string(exitErr.Stderr), "no outputs defined") {
			rawOutput = []byte("{}")
		} else {
			return nil, fmt.Errorf("Failed to retrieve output.\nError: %s\nOutput: %s", err, rawOutput)
		}
	}

	tfOutput := map[string]map[string]interface{}{}
	if err = json.Unmarshal(rawOutput, &tfOutput); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal JSON output.\nError: %s\nOutput: %s", err, rawOutput)
	}

	return tfOutput, nil
}

func (c *client) Version() (string, error) {
	outputCmd := c.terraformCmd([]string{
		"-v",
	}, nil)
	output, err := outputCmd.Output()
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve version.\nError: %s\nOutput: %s", err, output)
	}

	return strings.TrimSpace(string(output)), nil
}

func (c *client) Import(envName string) error {
	if len(c.model.Imports) == 0 {
		return nil
	}

	for tfID, iaasID := range c.model.Imports {
		exists, err := c.resourceExists(tfID, envName)
		if err != nil {
			return fmt.Errorf("Failed to check for existence of resource %s %s.\nError: %s", tfID, iaasID, err)
		}
		if exists {
			c.logWriter.Write([]byte(fmt.Sprintf("Skipping import of `%s: %s` as it already exists in the statefile...\n", tfID, iaasID)))
			continue
		}

		c.logWriter.Write([]byte(fmt.Sprintf("Importing `%s: %s`...\n", tfID, iaasID)))
		importArgs := []string{
			"import",
		}

		varFile, err := c.writeVarFile()
		if err != nil {
			return err
		}
		defer os.RemoveAll(varFile)

		importArgs = append(importArgs, fmt.Sprintf("-var-file=%s", varFile))
		importArgs = append(importArgs, tfID)
		importArgs = append(importArgs, iaasID)

		importCmd := c.terraformCmd(importArgs, nil)
		rawOutput, err := importCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Failed to import resource %s %s.\nError: %s\nOutput: %s", tfID, iaasID, err, rawOutput)
		}
	}

	return nil
}

func (c *client) ImportWithLegacyStorage() error {
	if len(c.model.Imports) == 0 {
		return nil
	}

	for tfID, iaasID := range c.model.Imports {
		exists, err := c.resourceExistsLegacyStorage(tfID)
		if err != nil {
			return fmt.Errorf("Failed to check for existence of resource %s %s.\nError: %s", tfID, iaasID, err)
		}
		if exists {
			c.logWriter.Write([]byte(fmt.Sprintf("Skipping import of `%s: %s` as it already exists in the statefile...\n", tfID, iaasID)))
			continue
		}

		c.logWriter.Write([]byte(fmt.Sprintf("Importing `%s: %s`...\n", tfID, iaasID)))
		importArgs := []string{
			"import",
			fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
		}

		varFile, err := c.writeVarFile()
		if err != nil {
			return err
		}
		defer os.RemoveAll(varFile)

		importArgs = append(importArgs, fmt.Sprintf("-var-file=%s", varFile))
		importArgs = append(importArgs, tfID)
		importArgs = append(importArgs, iaasID)

		importCmd := c.terraformCmd(importArgs, nil)
		rawOutput, err := importCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Failed to import resource %s %s.\nError: %s\nOutput: %s", tfID, iaasID, err, rawOutput)
		}
	}

	return nil
}

func (c *client) WorkspaceList() ([]string, error) {
	cmd := c.terraformCmd([]string{
		"workspace",
		"list",
	}, nil)
	rawOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Error running `workspace list`: %s, Output: %s", err, rawOutput)
	}

	envs := []string{}
	scanner := bufio.NewScanner(bytes.NewReader(rawOutput))
	for scanner.Scan() {
		env := strings.TrimPrefix(scanner.Text(), "*")
		env = strings.TrimSpace(env)
		if len(env) > 0 {
			envs = append(envs, env)
		}
	}

	return envs, nil
}

func (c *client) WorkspaceSelect(envName string) error {
	cmd := c.terraformCmd([]string{
		"workspace",
		"select",
		envName,
	}, nil)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error running `workspace select`: %s, Output: %s", err, output)
	}

	return nil
}

func (c *client) WorkspaceNewIfNotExists(envName string) error {
	workspaces, err := c.WorkspaceList()

	if err != nil {
		return err
	}

	workspaceExists := false
	for _, space := range workspaces {
		if space == envName {
			workspaceExists = true
		}
	}

	if workspaceExists {
		return c.WorkspaceSelect(envName)
	}

	cmd := c.terraformCmd([]string{
		"workspace",
		"new",
		envName,
	}, nil)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error running `workspace new`: %s, Output: %s", err, output)
	}

	return nil
}

func (c *client) WorkspaceNewFromExistingStateFile(envName string, localStateFilePath string) error {
	cmd := c.terraformCmd([]string{
		"workspace",
		"new",
		fmt.Sprintf("-state=%s", localStateFilePath),
		envName,
	}, nil)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error running `workspace new -state`: %s, Output: %s", err, output)
	}

	return nil
}

func (c *client) WorkspaceDelete(envName string) error {
	cmd := c.terraformCmd([]string{
		"workspace",
		"delete",
		envName,
	}, []string{
		"TF_WORKSPACE=default",
	})

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error running `workspace delete`: %s, Output: %s", err, output)
	}

	return nil
}

func (c *client) WorkspaceDeleteWithForce(envName string) error {
	cmd := c.terraformCmd([]string{
		"workspace",
		"delete",
		"-force",
		envName,
	}, []string{
		"TF_WORKSPACE=default",
	})

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error running `workspace delete -force`: %s, Output: %s", err, output)
	}

	return nil
}

func (c *client) StatePull(envName string) ([]byte, error) {
	cmd := c.terraformCmd([]string{
		"state",
		"pull",
	}, []string{
		fmt.Sprintf("TF_WORKSPACE=%s", envName),
	})

	rawOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Error running `state pull`: %s, Output: %s", err, rawOutput)
	}

	return rawOutput, nil
}

func (c *client) CurrentStateVersion(envName string) (StateVersion, error) {
	rawState, err := c.StatePull(envName)
	if err != nil {
		return StateVersion{}, err
	}

	tfState := map[string]interface{}{}
	if err = json.Unmarshal(rawState, &tfState); err != nil {
		return StateVersion{}, fmt.Errorf("Failed to unmarshal JSON output.\nError: %s\nOutput: %s", err, rawState)
	}

	serial, ok := tfState["serial"].(float64)
	if !ok {
		return StateVersion{}, fmt.Errorf("Expected number value for 'serial' but got '%#v'", tfState["serial"])
	}
	lineage, ok := tfState["lineage"].(string)
	if !ok {
		return StateVersion{}, fmt.Errorf("Expected string value for 'lineage' but got '%#v'", tfState["lineage"])
	}

	return StateVersion{
		Serial:  int(serial),
		Lineage: lineage,
	}, nil
}

func (c *client) SavePlanToBackend(planEnvName string) error {
	planContents, err := ioutil.ReadFile(c.model.PlanFileLocalPath)
	if err != nil {
		return err
	}

	tmpDir, err := ioutil.TempDir("", "tf-resource-plan")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// TODO: this stateful set and reset isn't great
	origDir, err := os.Getwd()
	if err != nil {
		return err
	}
	origSource := c.model.Source
	origLogger := c.logWriter

	err = os.Chdir(tmpDir)
	if err != nil {
		return err
	}
	c.model.Source = tmpDir
	c.logWriter = ioutil.Discard // prevent provider from logging creds

	defer func() {
		os.Chdir(origDir)
		c.model.Source = origSource
		c.logWriter = origLogger
	}()

	err = c.writePlanProviderConfig(tmpDir, planContents)
	if err != nil {
		return err
	}

	err = c.InitWithBackend()
	if err != nil {
		return err
	}

	err = c.WorkspaceNewIfNotExists(planEnvName)
	if err != nil {
		return err
	}

	err = c.Apply()
	if err != nil {
		return err
	}

	return nil
}

func (c *client) GetPlanFromBackend(planEnvName string) error {
	if err := c.WorkspaceSelect(planEnvName); err != nil {
		return err
	}

	outputs, err := c.Output(planEnvName)
	if err != nil {
		return err
	}
	encodedPlan := outputs["plan_content"]["value"].(string)

	decodedPlan, err := base64.StdEncoding.DecodeString(encodedPlan)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(c.model.PlanFileLocalPath, []byte(decodedPlan), 0755); err != nil {
		return err
	}

	return nil
}

func (c *client) resourceExists(tfID string, envName string) (bool, error) {
	cmd := c.terraformCmd([]string{
		"state",
		"list",
		tfID,
	}, []string{
		fmt.Sprintf("TF_WORKSPACE=%s", envName),
	})
	rawOutput, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("Error running `state list`: %s, Output: %s", err, rawOutput)
	}

	// command returns the ID of the resource if it exists
	return (len(strings.TrimSpace(string(rawOutput))) > 0), nil
}

func (c *client) resourceExistsLegacyStorage(tfID string) (bool, error) {
	if _, err := os.Stat(c.model.StateFileLocalPath); os.IsNotExist(err) {
		return false, nil
	}

	cmd := c.terraformCmd([]string{
		"state",
		"list",
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
		tfID,
	}, nil)
	rawOutput, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("Error running `state list -state`: %s, Output: %s", err, rawOutput)
	}

	// command returns the ID of the resource if it exists
	return (len(strings.TrimSpace(string(rawOutput))) > 0), nil
}

func (c *client) terraformCmd(args []string, env []string) *exec.Cmd {
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("terraform %s", strings.Join(args, " ")))

	cmd.Dir = c.model.Source
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CHECKPOINT_DISABLE=1")
	// TODO: remove the following line once this issue is fixed:
	// https://github.com/hashicorp/terraform/issues/17655
	cmd.Env = append(cmd.Env, "TF_WARN_OUTPUT_ERRORS=1")
	for _, e := range env {
		cmd.Env = append(cmd.Env, e)
	}

	for key, value := range c.model.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	return cmd
}

func (c *client) writeVarFile() (string, error) {
	yamlContents, err := yaml.Marshal(c.model.Vars)
	if err != nil {
		return "", err
	}

	// avoids marshalling errors around map[interface{}]interface{}
	jsonFileContents, err := yamlConverter.YAMLToJSON(yamlContents)
	if err != nil {
		return "", err
	}

	varsFile, err := ioutil.TempFile("", "vars-file")
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
