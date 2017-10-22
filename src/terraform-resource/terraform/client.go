package terraform

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"reflect"
	"strings"

	"terraform-resource/models"
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
	WorkspaceNew(string) error
	WorkspaceDelete(string) error
	StatePull(string) ([]byte, error)
}

type client struct {
	model     models.Terraform
	logWriter io.Writer
}

func NewClient(model models.Terraform, logWriter io.Writer) Client {
	return client{
		model:     model,
		logWriter: logWriter,
	}
}

func (c client) InitWithBackend() error {
	if err := c.writeBackendOverride(); err != nil {
		return err
	}

	initArgs := []string{
		"init",
		"-input=false",
		"-get=true",
		"-backend=true",
	}
	for key, value := range c.model.BackendConfig {
		initArgs = append(initArgs, fmt.Sprintf("-backend-config='%s=%v'", key, value))
	}
	if c.model.PluginDir != "" {
		initArgs = append(initArgs, fmt.Sprintf("-plugin-dir=%s", c.model.PluginDir))
	}
	initArgs = append(initArgs, c.model.Source)

	initCmd := c.terraformCmd(initArgs, nil)
	var err error
	var output []byte
	if output, err = initCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", err, output)
	}

	return nil
}

func (c client) writeBackendOverride() error {
	// TODO: use an override file once this PR is merged:
	// https://github.com/hashicorp/terraform/pull/16415
	// backendPath := path.Join(c.model.Source, "resource_backend_override.tf")
	backendPath := path.Join(c.model.Source, "resource_backend.tf")
	backendContent := fmt.Sprintf(`terraform { backend "%s" {} }`, c.model.BackendType)
	return ioutil.WriteFile(backendPath, []byte(backendContent), 0755)
}

func (c client) InitWithoutBackend() error {
	initArgs := []string{
		"init",
		"-input=false",
		"-get=true",
		"-backend=false",
	}
	if c.model.PluginDir != "" {
		initArgs = append(initArgs, fmt.Sprintf("-plugin-dir=%s", c.model.PluginDir))
	}
	initArgs = append(initArgs, c.model.Source)
	initCmd := c.terraformCmd(initArgs, nil)

	if output, err := initCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", err, output)
	}

	return nil
}

func (c client) Apply() error {
	applyArgs := []string{
		"apply",
		"-backup='-'",  // no need to backup state file
		"-input=false", // do not prompt for inputs
		"-auto-approve",
	}
	if c.model.PlanRun == false {
		applyArgs = append(applyArgs, c.varFlags()...)
		// TODO: remove state flag for backend?
		applyArgs = append(applyArgs, fmt.Sprintf("-state=%s", c.model.StateFileLocalPath))
	} else {
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

func (c client) Destroy() error {
	destroyArgs := []string{
		"destroy",
		"-backup='-'", // no need to backup state file
		"-force",      // do not prompt for confirmation
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
	}
	destroyArgs = append(destroyArgs, c.varFlags()...)

	destroyCmd := c.terraformCmd(destroyArgs, nil)
	destroyCmd.Stdout = c.logWriter
	destroyCmd.Stderr = c.logWriter
	err := destroyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c client) Plan() error {
	planArgs := []string{
		"plan",
		"-input=false", // do not prompt for inputs
		fmt.Sprintf("-out=%s", c.model.PlanFileLocalPath),
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
	}
	planArgs = append(planArgs, c.varFlags()...)

	planCmd := c.terraformCmd(planArgs, nil)
	planCmd.Stdout = c.logWriter
	planCmd.Stderr = c.logWriter
	err := planCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c client) Output(envName string) (map[string]map[string]interface{}, error) {
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

	rawOutput, err := outputCmd.CombinedOutput()
	if err != nil {
		// TF CLI currently doesn't provide a nice way to detect an empty set of outputs
		// https://github.com/hashicorp/terraform/issues/11696
		if strings.Contains(string(rawOutput), "no outputs defined") {
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

func (c client) OutputWithLegacyStorage() (map[string]map[string]interface{}, error) {
	outputArgs := []string{
		"output",
		"-json",
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
	}

	if c.model.OutputModule != "" {
		outputArgs = append(outputArgs, fmt.Sprintf("-module=%s", c.model.OutputModule))
	}

	outputCmd := c.terraformCmd(outputArgs, nil)

	rawOutput, err := outputCmd.CombinedOutput()
	if err != nil {
		// TF CLI currently doesn't provide a nice way to detect an empty set of outputs
		// https://github.com/hashicorp/terraform/issues/11696
		if strings.Contains(string(rawOutput), "no outputs defined") {
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

func (c client) Version() (string, error) {
	outputCmd := c.terraformCmd([]string{
		"-v",
	}, nil)
	output, err := outputCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve version.\nError: %s\nOutput: %s", err, output)
	}

	return strings.TrimSpace(string(output)), nil
}

func (c client) Import(envName string) error {
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
		importArgs = append(importArgs, c.varFlags()...)
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

func (c client) ImportWithLegacyStorage() error {
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
		importArgs = append(importArgs, c.varFlags()...)
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

func (c client) WorkspaceList() ([]string, error) {
	cmd := c.terraformCmd([]string{
		"workspace",
		"list",
	}, nil)
	rawOutput, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Error: %s, Output: %s", err, rawOutput)
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

func (c client) WorkspaceNew(envName string) error {
	cmd := c.terraformCmd([]string{
		"workspace",
		"new",
		envName,
	}, nil)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error: %s, Output: %s", err, output)
	}

	return nil
}

func (c client) WorkspaceDelete(envName string) error {
	cmd := c.terraformCmd([]string{
		"workspace",
		"delete",
		envName,
	}, []string{
		"TF_WORKSPACE=default",
	})

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Error: %s, Output: %s", err, output)
	}

	return nil
}

func (c client) StatePull(envName string) ([]byte, error) {
	cmd := c.terraformCmd([]string{
		"state",
		"pull",
	}, []string{
		fmt.Sprintf("TF_WORKSPACE=%s", envName),
	})

	rawOutput, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Error: %s, Output: %s", err, rawOutput)
	}

	return rawOutput, nil
}

func (c client) resourceExists(tfID string, envName string) (bool, error) {
	cmd := c.terraformCmd([]string{
		"state",
		"list",
		tfID,
	}, []string{
		fmt.Sprintf("TF_WORKSPACE=%s", envName),
	})
	rawOutput, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("Error: %s, Output: %s", err, rawOutput)
	}

	// command returns the ID of the resource if it exists
	return (len(strings.TrimSpace(string(rawOutput))) > 0), nil
}

func (c client) resourceExistsLegacyStorage(tfID string) (bool, error) {
	if _, err := os.Stat(c.model.StateFileLocalPath); os.IsNotExist(err) {
		return false, nil
	}

	cmd := c.terraformCmd([]string{
		"state",
		"list",
		fmt.Sprintf("-state=%s", c.model.StateFileLocalPath),
		tfID,
	}, nil)
	rawOutput, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("Error: %s, Output: %s", err, rawOutput)
	}

	// command returns the ID of the resource if it exists
	return (len(strings.TrimSpace(string(rawOutput))) > 0), nil
}

func (c client) terraformCmd(args []string, env []string) *exec.Cmd {
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("terraform %s", strings.Join(args, " ")))

	cmd.Dir = c.model.Source
	cmd.Env = []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		"CHECKPOINT_DISABLE=1",
	}
	for _, e := range env {
		cmd.Env = append(cmd.Env, e)
	}

	return cmd
}

func (c client) varFlags() []string {
	args := []string{}
	for key, val := range c.model.Vars {
		args = append(args, "-var", fmt.Sprintf("'%s=%s'", key, formatVar(val)))
	}
	return args
}

func formatVar(value interface{}) string {
	valType := reflect.TypeOf(value)
	if valType == nil {
		return "null"
	}

	switch valType.Kind() {
	case reflect.Slice:
		valSlice, _ := value.([]interface{})
		sliceVars := []string{}
		for _, v := range valSlice {
			sliceVars = append(sliceVars, formatVar(v))
		}
		return fmt.Sprintf("[%s]", strings.Join(sliceVars, ","))
	case reflect.Map:
		valMap, _ := value.(map[string]interface{})
		mapVars := []string{}
		for k, v := range valMap {
			mapVars = append(mapVars, fmt.Sprintf("%s=%s", k, formatVar(v)))
		}
		return fmt.Sprintf("{%s}", strings.Join(mapVars, ","))
	}
	return fmt.Sprintf("%#v", value)
}
