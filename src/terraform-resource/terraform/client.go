package terraform

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
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

	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/runner"
)

const defaultWorkspace = "default"

//go:generate counterfeiter . Client

type Client interface {
	InitWithBackend() error
	InitWithoutBackend() error
	Apply() error
	Destroy() error
	Plan() (string, error)
	JSONPlan() error
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
	SetModel(models.Terraform)
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
	if err := c.clearTerraformState(); err != nil {
		return err
	}
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

	initCmd, err := c.terraformCmd(initArgs, nil)
	if err != nil {
		return err
	}
	var output []byte
	if output, err = initCmd.CombinedOutput(); err != nil {
		// Terraform 0.15.0 removes the -get-plugins=false flag, it will return
		// an error if the user has previously uploaded a "default" workspace which uses
		// custom provider plugins. Despite the error message the initialization has otherwise
		// succeeded so we swallow this error.
		if !c.model.DownloadPlugins {
			downloadErrsToIgnore := []string{
				"Failed to install provider",
				"Failed to query available provider packages",
				"Invalid provider registry host",
			}
			for _, errSnippet := range downloadErrsToIgnore {
				if bytes.Contains(output, []byte(errSnippet)) {
					return nil
				}
			}
		}
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

func (c *client) writePlanProviderConfig(outputDir string, planContents, planContentsJSON []byte) error {
	// GZip JSON plan to save space:
	// https://github.com/ljfranklin/terraform-resource/issues/115#issuecomment-619525494
	// Not gzipping the binary plan for now to avoid migration issues.

	encodedPlan := base64.StdEncoding.EncodeToString(planContents)
	escapedPlan, err := json.Marshal(encodedPlan)
	if err != nil {
		return err
	}

	var encodedJSONBuffer bytes.Buffer
	baseEncoder := base64.NewEncoder(base64.StdEncoding, &encodedJSONBuffer)
	zw := gzip.NewWriter(baseEncoder)
	if _, err = zw.Write(planContentsJSON); err != nil {
		return err
	}
	if err = zw.Close(); err != nil {
		return err
	}
	if err = baseEncoder.Close(); err != nil {
		return err
	}
	escapedJSONPlan, err := json.Marshal(encodedJSONBuffer.String())
	if err != nil {
		return err
	}

	configContents := []byte(fmt.Sprintf(`
terraform {
  required_providers {
    stateful = {
      source = "github.com/ashald/stateful"
      version = "~> 1.0"
    }
  }
}
resource "stateful_string" "plan_output" {
  desired = %s
}
resource "stateful_string" "plan_output_json" {
  desired = %s
}
output "%s" {
  sensitive = true
  value = stateful_string.plan_output.desired
}
output "%s" {
  sensitive = true
  value = stateful_string.plan_output_json.desired
}
`, escapedPlan, escapedJSONPlan, models.PlanContent, models.PlanContentJSON))

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
	backendContent := fmt.Sprintf(`terraform {
		backend "%s" {}
	}`, c.model.BackendType)
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
	initCmd, err := c.terraformCmd(initArgs, nil)
	if err != nil {
		return err
	}

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

	// only used in non-backend flow
	if c.model.StateFileLocalPath != "" {
		applyArgs = append(applyArgs, fmt.Sprintf("-state=%s", c.model.StateFileLocalPath))
	}

	if c.model.PlanRun == false {
		for _, varFile := range c.model.ConvertedVarFiles {
			applyArgs = append(applyArgs, fmt.Sprintf("-var-file=%s", varFile))
		}
	}

	if c.model.Parallelism > 0 {
		applyArgs = append(applyArgs, fmt.Sprintf("-parallelism=%d", c.model.Parallelism))
	}

	if c.model.LockTimeout != "" {
		applyArgs = append(applyArgs, fmt.Sprintf("-lock-timeout=%s", c.model.LockTimeout))
	}

	if c.model.PlanRun {
		// Since the plan path is a positional arg it must come last.
		applyArgs = append(applyArgs, c.model.PlanFileLocalPath)
	}

	applyCmd, err := c.terraformCmd(applyArgs, nil)
	if err != nil {
		return err
	}
	applyCmd.Stdout = c.logWriter
	applyCmd.Stderr = c.logWriter
	err = applyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c *client) Destroy() error {
	destroyArgs := []string{
		"destroy",
		"-backup='-'",   // no need to backup state file
		"-auto-approve", // do not prompt for confirmation
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
	}

	if c.model.LockTimeout != "" {
		destroyArgs = append(destroyArgs, fmt.Sprintf("-lock-timeout=%s", c.model.LockTimeout))
	}

	for _, varFile := range c.model.ConvertedVarFiles {
		destroyArgs = append(destroyArgs, fmt.Sprintf("-var-file=%s", varFile))
	}

	destroyCmd, err := c.terraformCmd(destroyArgs, nil)
	if err != nil {
		return err
	}
	destroyCmd.Stdout = c.logWriter
	destroyCmd.Stderr = c.logWriter
	err = destroyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c *client) Plan() (string, error) {
	planArgs := []string{
		"plan",
		"-input=false", // do not prompt for inputs
		fmt.Sprintf("-out=%s", c.model.PlanFileLocalPath),
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
	}

	if c.model.LockTimeout != "" {
		planArgs = append(planArgs, fmt.Sprintf("-lock-timeout=%s", c.model.LockTimeout))
	}

	for _, varFile := range c.model.ConvertedVarFiles {
		planArgs = append(planArgs, fmt.Sprintf("-var-file=%s", varFile))
	}

	planCmd, err := c.terraformCmd(planArgs, nil)
	if err != nil {
		return "", err
	}
	planCmd.Stdout = c.logWriter
	planCmd.Stderr = c.logWriter
	err = planCmd.Run()
	if err != nil {
		return "", fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	planFile, err := os.Open(c.model.PlanFileLocalPath)
	if err != nil {
		return "", fmt.Errorf("Failed to open planfile: %s", err)
	}
	defer planFile.Close()

	h := sha256.New()
	if _, err := io.Copy(h, planFile); err != nil {
		return "", fmt.Errorf("Failed to get planfile checksum: %s", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (c *client) JSONPlan() error {
	// terraform show -json tfplan.binary > tfplan.json
	planArgs := []string{
		"show",
		"-json",
		fmt.Sprintf("%s", c.model.PlanFileLocalPath),
	}

	showCmd, err := c.terraformCmd(planArgs, nil)
	if err != nil {
		return err
	}
	rawOutput, err := showCmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to retrieve output.\nError: %s\nOutput: %s", err, rawOutput)
	}

	err = ioutil.WriteFile(c.model.JSONPlanFileLocalPath, rawOutput, 0644)
	if err != nil {
		return fmt.Errorf("Failed to write JSON planfile to %s: %s", c.model.JSONPlanFileLocalPath, err)
	}

	return nil
}

func (c *client) Output(envName string) (map[string]map[string]interface{}, error) {
	outputArgs := []string{
		"output",
		"-json",
	}
	outputCmd, err := c.terraformCmd(outputArgs, []string{
		fmt.Sprintf("TF_WORKSPACE=%s", envName),
	})
	if err != nil {
		return nil, err
	}

	rawOutput, err := outputCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve output.\nError: %s\nOutput: %s", err, rawOutput)
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

	outputCmd, err := c.terraformCmd(outputArgs, nil)
	if err != nil {
		return nil, err
	}

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
	outputCmd, err := c.terraformCmd([]string{
		"-v",
	}, nil)
	if err != nil {
		return "", err
	}
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

		for _, varFile := range c.model.ConvertedVarFiles {
			importArgs = append(importArgs, fmt.Sprintf("-var-file=%s", varFile))
		}

		importArgs = append(importArgs, tfID)
		importArgs = append(importArgs, iaasID)

		importCmd, err := c.terraformCmd(importArgs, nil)
		if err != nil {
			return err
		}
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

		for _, varFile := range c.model.ConvertedVarFiles {
			importArgs = append(importArgs, fmt.Sprintf("-var-file=%s", varFile))
		}

		importArgs = append(importArgs, tfID)
		importArgs = append(importArgs, iaasID)

		importCmd, err := c.terraformCmd(importArgs, nil)
		if err != nil {
			return err
		}
		rawOutput, err := importCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Failed to import resource %s %s.\nError: %s\nOutput: %s", tfID, iaasID, err, rawOutput)
		}
	}

	return nil
}

func (c *client) WorkspaceList() ([]string, error) {
	cmd, err := c.terraformCmd([]string{
		"workspace",
		"list",
	}, nil)
	if err != nil {
		return nil, err
	}
	rawOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Error running `workspace list`: %s, Output: %s", err, err.(*exec.ExitError).Stderr)
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
	cmd, err := c.terraformCmd([]string{
		"workspace",
		"select",
		envName,
	}, nil)
	if err != nil {
		return err
	}

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

	cmd, err := c.terraformCmd([]string{
		"workspace",
		"new",
		envName,
	}, nil)
	if err != nil {
		return err
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error running `workspace new`: %s, Output: %s", err, output)
	}

	return nil
}

func (c *client) WorkspaceNewFromExistingStateFile(envName string, localStateFilePath string) error {
	cmd, err := c.terraformCmd([]string{
		"workspace",
		"new",
		fmt.Sprintf("-state=%s", localStateFilePath),
		envName,
	}, nil)
	if err != nil {
		return err
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error running `workspace new -state`: %s, Output: %s", err, output)
	}

	cmd, err = c.terraformCmd([]string{
		"state",
		"push",
		localStateFilePath,
	}, nil)
	if err != nil {
		return err
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error running `state push`: %s, Output: %s", err, output)
	}

	return nil
}

func (c *client) WorkspaceDelete(envName string) error {
	if envName == defaultWorkspace {
		return nil
	}

	cmd, err := c.terraformCmd([]string{
		"workspace",
		"delete",
		envName,
	}, []string{
		fmt.Sprintf("TF_WORKSPACE=%s", defaultWorkspace),
	})
	if err != nil {
		return err
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error running `workspace delete`: %s, Output: %s", err, output)
	}

	return nil
}

func (c *client) WorkspaceDeleteWithForce(envName string) error {
	if envName == defaultWorkspace {
		return nil
	}

	cmd, err := c.terraformCmd([]string{
		"workspace",
		"delete",
		"-force",
		envName,
	}, []string{
		fmt.Sprintf("TF_WORKSPACE=%s", defaultWorkspace),
	})
	if err != nil {
		return err
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error running `workspace delete -force`: %s, Output: %s", err, output)
	}

	return nil
}

func (c *client) StatePull(envName string) ([]byte, error) {
	cmd, err := c.terraformCmd([]string{
		"state",
		"pull",
	}, []string{
		fmt.Sprintf("TF_WORKSPACE=%s", envName),
	})
	if err != nil {
		return nil, err
	}

	rawOutput, err := cmd.Output()
	if err != nil {
		errOutput := rawOutput
		if exitErr, ok := err.(*exec.ExitError); ok {
			errOutput = exitErr.Stderr
		}
		return nil, fmt.Errorf("Error running `state pull`: %s, Output: %s", err, errOutput)
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
	planContentsJSON, err := ioutil.ReadFile(c.model.JSONPlanFileLocalPath)
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

	logPath := path.Join(os.TempDir(), "tf-plan.log")
	logFile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	c.logWriter = logFile // prevent provider from logging creds

	defer func() {
		os.Chdir(origDir)
		c.model.Source = origSource
		c.logWriter = origLogger
	}()

	// The /tmp/tf-plan.log file can contain credentials, so we tell the user to
	// SSH into the container to view it rather than printing logs directly to the build logs.
	errPrefix := "Failed to upload plan file to TF backend. Use `fly intercept` to SSH into this container and view %s for more logs. Error: %s"
	err = c.writePlanProviderConfig(tmpDir, planContents, planContentsJSON)
	if err != nil {
		return fmt.Errorf(errPrefix, logPath, err)
	}

	err = c.InitWithBackend()
	if err != nil {
		return fmt.Errorf(errPrefix, logPath, err)
	}

	err = c.WorkspaceNewIfNotExists(planEnvName)
	if err != nil {
		return fmt.Errorf(errPrefix, logPath, err)
	}

	err = c.Apply()
	if err != nil {
		return fmt.Errorf(errPrefix, logPath, err)
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

	var encodedPlan string
	if val, ok := outputs[models.PlanContent]; ok {
		encodedPlan = val["value"].(string)
	} else {
		return fmt.Errorf("state has no output for key %s", models.PlanContent)
	}

	decodedPlan, err := base64.StdEncoding.DecodeString(encodedPlan)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(c.model.PlanFileLocalPath, []byte(decodedPlan), 0755); err != nil {
		return err
	}

	return nil
}

func (c *client) SetModel(model models.Terraform) {
	c.model = model
}

func (c *client) resourceExists(tfID string, envName string) (bool, error) {
	cmd, err := c.terraformCmd([]string{
		"state",
		"list",
		tfID,
	}, []string{
		fmt.Sprintf("TF_WORKSPACE=%s", envName),
	})
	if err != nil {
		return false, err
	}
	rawOutput, err := cmd.Output()
	if err != nil {
		return false, nil
	}

	// command returns the ID of the resource if it exists
	return (len(strings.TrimSpace(string(rawOutput))) > 0), nil
}

func (c *client) resourceExistsLegacyStorage(tfID string) (bool, error) {
	if _, err := os.Stat(c.model.StateFileLocalPath); os.IsNotExist(err) {
		return false, nil
	}

	cmd, err := c.terraformCmd([]string{
		"state",
		"list",
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
		tfID,
	}, nil)
	if err != nil {
		return false, err
	}
	rawOutput, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("Error running `state list -state`: %s, Output: %s", err, rawOutput)
	}

	// command returns the ID of the resource if it exists
	return (len(strings.TrimSpace(string(rawOutput))) > 0), nil
}

func (c *client) terraformCmd(args []string, env []string) (*runner.Runner, error) {
	cmdPath, err := exec.LookPath("terraform")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(cmdPath, args...)

	cmd.Dir = c.model.Source
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CHECKPOINT_DISABLE=1")
	// TODO: remove the following line once this issue is fixed:
	// https://github.com/hashicorp/terraform/issues/17655
	cmd.Env = append(cmd.Env, "TF_WARN_OUTPUT_ERRORS=1")
	// To control terraform output in automation.
	// As suggested in https://learn.hashicorp.com/terraform/development/running-terraform-in-automation#controlling-terraform-output-in-automation
	cmd.Env = append(cmd.Env, "TF_IN_AUTOMATION=1")
	for _, e := range env {
		cmd.Env = append(cmd.Env, e)
	}

	for key, value := range c.model.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	return runner.New(cmd, c.logWriter), nil
}
