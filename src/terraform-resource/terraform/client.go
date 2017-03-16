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
	outputCmd := c.terraformCmd([]string{
		"output",
		"-json",
		fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
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
	var sourceDir string
	if c.useLocalSource() {
		sourceDir = c.Model.Source
	} else {
		tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-client")
		if err != nil {
			return "", fmt.Errorf("Failed to create temporary working dir at '%s'", os.TempDir())
		}
		initCmd := c.terraformCmd([]string{
			"init",
			c.Model.Source,
			tmpDir,
		})
		if initOutput, initErr := initCmd.CombinedOutput(); initErr != nil {
			return "", fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", initErr, initOutput)
		}
		sourceDir = tmpDir
	}

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

func (c Client) useLocalSource() bool {
	if info, err := os.Stat(c.Model.Source); err == nil && info.IsDir() {
		return true
	}
	return false
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
