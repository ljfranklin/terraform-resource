package terraform

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-client")
	if err != nil {
		return fmt.Errorf("Failed to create temporary working dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	initCmd := terraformCmd([]string{
		"init",
		c.Model.Source,
		tmpDir,
	})
	if initOutput, initErr := initCmd.CombinedOutput(); initErr != nil {
		return fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", initErr, initOutput)
	}

	getCmd := terraformCmd([]string{
		"get",
		"-update",
		tmpDir,
	})
	if getOutput, getErr := getCmd.CombinedOutput(); getErr != nil {
		return fmt.Errorf("terraform get command failed.\nError: %s\nOutput: %s", getErr, getOutput)
	}

	applyArgs := []string{
		"apply",
		"-backup='-'",  // no need to backup state file
		"-input=false", // do not prompt for inputs
	}
	if c.Model.PlanRun {
		applyArgs = append(applyArgs, fmt.Sprintf("%s", c.Model.PlanFileLocalPath))
	} else {
		applyArgs = append(applyArgs, fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath))
	}

	applyArgs = append(applyArgs, c.varFlags()...)
	applyArgs = append(applyArgs, tmpDir)

	applyCmd := terraformCmd(applyArgs)
	applyCmd.Stdout = c.LogWriter
	applyCmd.Stderr = c.LogWriter
	err = applyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c Client) Destroy() error {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-client")
	if err != nil {
		return fmt.Errorf("Failed to create temporary working dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	initCmd := terraformCmd([]string{
		"init",
		c.Model.Source,
		tmpDir,
	})
	if initOutput, initErr := initCmd.CombinedOutput(); initErr != nil {
		return fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", initErr, initOutput)
	}

	getCmd := terraformCmd([]string{
		"get",
		"-update",
		tmpDir,
	})
	if getOutput, getErr := getCmd.CombinedOutput(); getErr != nil {
		return fmt.Errorf("terraform get command failed.\nError: %s\nOutput: %s", getErr, getOutput)
	}

	destroyArgs := []string{
		"destroy",
		"-backup='-'", // no need to backup state file
		"-force",      // do not prompt for confirmation
		fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
	}
	destroyArgs = append(destroyArgs, c.varFlags()...)
	destroyArgs = append(destroyArgs, tmpDir)

	destroyCmd := terraformCmd(destroyArgs)
	destroyCmd.Stdout = c.LogWriter
	destroyCmd.Stderr = c.LogWriter
	err = destroyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c Client) Plan() error {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-client")
	if err != nil {
		return fmt.Errorf("Failed to create temporary working dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	initCmd := terraformCmd([]string{
		"init",
		c.Model.Source,
		tmpDir,
	})
	if initOutput, initErr := initCmd.CombinedOutput(); initErr != nil {
		return fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", initErr, initOutput)
	}

	getCmd := terraformCmd([]string{
		"get",
		"-update",
		tmpDir,
	})
	if getOutput, getErr := getCmd.CombinedOutput(); getErr != nil {
		return fmt.Errorf("terraform get command failed.\nError: %s\nOutput: %s", getErr, getOutput)
	}

	planArgs := []string{
		"plan",
		"-input=false", // do not prompt for inputs
		fmt.Sprintf("-out=%s", c.Model.PlanFileLocalPath),
		fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
	}
	planArgs = append(planArgs, c.varFlags()...)
	planArgs = append(planArgs, tmpDir)

	planCmd := terraformCmd(planArgs)
	planCmd.Stdout = c.LogWriter
	planCmd.Stderr = c.LogWriter
	err = planCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c Client) Output() (map[string]interface{}, error) {
	outputCmd := terraformCmd([]string{
		"output",
		"-json",
		fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
	})
	rawOutput, err := outputCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve output.\nError: %s\nOutput: %s", err, rawOutput)
	}

	tfOutput := map[string]map[string]interface{}{}
	if err = json.Unmarshal(rawOutput, &tfOutput); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal JSON output.\nError: %s\nOutput: %s", err, rawOutput)
	}

	output := map[string]interface{}{}
	for key, value := range tfOutput {
		output[key] = value["value"]
	}

	return output, nil
}

func terraformCmd(args []string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", fmt.Sprintf("terraform %s", strings.Join(args, " ")))
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
