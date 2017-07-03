package terraform

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"terraform-resource/models"
	"terraform-resource/storage"
)

type Client struct {
	Model         models.Terraform
	StorageDriver storage.Storage
	LogWriter     io.Writer
}

func (c Client) Apply() error {
	var sourcePath string
	if c.Model.PlanRun {
		sourcePath = c.Model.PlanFileLocalPath
	} else {
		var err error
		sourcePath, err = c.fetchSource()
		if err != nil {
			return err
		}
	}

	applyArgs := []string{
		"apply",
		"-backup='-'",  // no need to backup state file
		"-input=false", // do not prompt for inputs
	}
	if c.Model.PlanRun == false {
		applyArgs = append(applyArgs, c.varFlags()...)
		applyArgs = append(applyArgs, fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath))
	} else {
		applyArgs = append(applyArgs, fmt.Sprintf("-state-out=%s", c.Model.StateFileLocalPath))
	}

	applyArgs = append(applyArgs, sourcePath)

	applyCmd := c.terraformCmd(applyArgs)
	applyCmd.Stdout = c.LogWriter
	applyCmd.Stderr = c.LogWriter
	err := applyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c Client) Destroy() error {
	sourcePath, err := c.fetchSource()
	if err != nil {
		return err
	}

	destroyArgs := []string{
		"destroy",
		"-backup='-'", // no need to backup state file
		"-force",      // do not prompt for confirmation
		fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
	}
	destroyArgs = append(destroyArgs, c.varFlags()...)
	destroyArgs = append(destroyArgs, sourcePath)

	destroyCmd := c.terraformCmd(destroyArgs)
	destroyCmd.Stdout = c.LogWriter
	destroyCmd.Stderr = c.LogWriter
	err = destroyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c Client) Plan() error {
	sourcePath, err := c.fetchSource()
	if err != nil {
		return err
	}

	planArgs := []string{
		"plan",
		"-input=false", // do not prompt for inputs
		fmt.Sprintf("-out=%s", c.Model.PlanFileLocalPath),
		fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
	}
	planArgs = append(planArgs, c.varFlags()...)
	planArgs = append(planArgs, sourcePath)

	planCmd := c.terraformCmd(planArgs)
	planCmd.Stdout = c.LogWriter
	planCmd.Stderr = c.LogWriter
	err = planCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c Client) Output() (map[string]map[string]interface{}, error) {
	outputArgs := []string{
		"output",
		"-json",
		fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
	}

	if c.Model.OutputModule != "" {
		outputArgs = append(outputArgs, fmt.Sprintf("-module=%s", c.Model.OutputModule))
	}

	outputCmd := c.terraformCmd(outputArgs)

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

func (c Client) Version() (string, error) {
	outputCmd := c.terraformCmd([]string{
		"-v",
	})
	output, err := outputCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve version.\nError: %s\nOutput: %s", err, output)
	}

	return strings.TrimSpace(string(output)), nil
}

func (c Client) Import() error {
	if len(c.Model.Imports) == 0 {
		return nil
	}

	sourcePath, err := c.fetchSource()
	if err != nil {
		return err
	}

	for tfID, iaasID := range c.Model.Imports {
		exists, err := c.resourceExists(tfID)
		if err != nil {
			return fmt.Errorf("Failed to check for existence of resource %s %s.\nError: %s", tfID, iaasID, err)
		}
		if exists {
			c.LogWriter.Write([]byte(fmt.Sprintf("Skipping import of `%s: %s` as it already exists in the statefile...\n", tfID, iaasID)))
			continue
		}

		c.LogWriter.Write([]byte(fmt.Sprintf("Importing `%s: %s`...\n", tfID, iaasID)))
		importArgs := []string{
			"import",
			fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
			fmt.Sprintf("-config=%s", sourcePath),
		}
		importArgs = append(importArgs, c.varFlags()...)
		importArgs = append(importArgs, tfID)
		importArgs = append(importArgs, iaasID)

		importCmd := c.terraformCmd(importArgs)
		rawOutput, err := importCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Failed to import resource %s %s.\nError: %s\nOutput: %s", tfID, iaasID, err, rawOutput)
		}
	}

	return nil
}

func (c Client) resourceExists(tfID string) (bool, error) {
	if _, err := os.Stat(c.Model.StateFileLocalPath); os.IsNotExist(err) {
		return false, nil
	}

	cmd := c.terraformCmd([]string{
		"state",
		"list",
		fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
		tfID,
	})
	rawOutput, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("Error: %s, Output: %s", err, rawOutput)
	}

	// command returns the ID of the resource if it exists
	return (len(strings.TrimSpace(string(rawOutput))) > 0), nil
}

func (c Client) terraformCmd(args []string) *exec.Cmd {
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("terraform %s", strings.Join(args, " ")))

	cmd.Env = []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}
	for key, value := range c.Model.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	return cmd
}

func (c Client) fetchSource() (string, error) {
	sourceDir := c.Model.Source
	getCmd := c.terraformCmd([]string{
		"get",
		"-update",
		sourceDir,
	})
	if getOutput, getErr := getCmd.CombinedOutput(); getErr != nil {
		return "", fmt.Errorf("terraform get command failed.\nError: %s\nOutput: %s", getErr, getOutput)
	}

	return sourceDir, nil
}

func (c Client) varFlags() []string {
	args := []string{}
	for key, val := range c.Model.Vars {
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
