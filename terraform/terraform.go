package terraform

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

type Client struct {
	// Source can be a local directory or a valid Terraform module source:
	// https://www.terraform.io/docs/modules/
	Source        string
	StateFilePath string
}

func (c Client) Apply(inputs map[string]interface{}) error {

	if c.Source == "" {
		return errors.New("Client.source can not be empty")
	}
	if c.StateFilePath == "" {
		return errors.New("Client.StateFilePath can not be empty")
	}

	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-client")
	if err != nil {
		return fmt.Errorf("Failed to create temporary working dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	initArgs := []string{
		"terraform",
		"init",
		c.Source,
		tmpDir,
	}
	if initOutput, initErr := runCmd(initArgs); initErr != nil {
		return fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", initErr, initOutput)
	}

	applyArgs := []string{
		"terraform",
		"apply",
		"-backup='-'",  // no need to backup state file
		"-input=false", // do not prompt for inputs
		fmt.Sprintf("-state=%s", c.StateFilePath),
	}
	for key, val := range inputs {
		applyArgs = append(applyArgs, "-var", fmt.Sprintf("'%s=%v'", key, val))
	}
	applyArgs = append(applyArgs, tmpDir)

	if applyOutput, err := runCmd(applyArgs); err != nil {
		return fmt.Errorf("terraform apply command failed.\nError: %s\nOutput: %s", err, applyOutput)
	}
	return nil
}

func (c Client) Output() (map[string]interface{}, error) {

	if c.StateFilePath == "" {
		return nil, errors.New("Client.StateFilePath can not be empty")
	}

	rawOutput, err := runCmd([]string{
		"terraform",
		"output",
		fmt.Sprintf("-state=%s", c.StateFilePath),
	})

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

func runCmd(args []string) ([]byte, error) {
	cmd := exec.Command("/bin/bash", "-c", strings.Join(args, " "))
	return cmd.CombinedOutput()
}
