package terraform

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
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
		fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
	}
	for key, val := range c.Model.Vars {
		applyArgs = append(applyArgs, "-var", fmt.Sprintf("'%s=%v'", key, val))
	}
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
	for key, val := range c.Model.Vars {
		destroyArgs = append(destroyArgs, "-var", fmt.Sprintf("'%s=%v'", key, val))
	}
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

func (c Client) Output() (map[string]interface{}, error) {
	outputCmd := terraformCmd([]string{
		"output",
		fmt.Sprintf("-state=%s", c.Model.StateFileLocalPath),
	})
	rawOutput, err := outputCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve output.\nError: %s\nOutput: %s", err, rawOutput)
	}

	output := map[string]interface{}{}
	scanner := bufio.NewScanner(bytes.NewReader(rawOutput))
	for scanner.Scan() {
		thisLine := strings.Split(scanner.Text(), " = ")
		key, value := thisLine[0], thisLine[1]
		output[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Failed to parse output.\nError: %s\nOutput: %s", err, rawOutput)
	}

	return output, nil
}

func terraformCmd(args []string) *exec.Cmd {
	return exec.Command("/bin/bash", "-c", fmt.Sprintf("terraform %s", strings.Join(args, " ")))
}
