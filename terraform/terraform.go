package terraform

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Client struct {
	// Source can be a local directory or a valid Terraform module source:
	// https://www.terraform.io/docs/modules/
	Source    string
	StateFile string
}

func (c Client) Apply(inputs map[string]interface{}) error {

	if c.Source == "" {
		return errors.New("Client.source can not be empty")
	}
	if c.StateFile == "" {
		return errors.New("Client.StateFile can not be empty")
	}

	args := []string{
		"terraform",
		"apply",
		"-backup='-'",  // no need to backup state file
		"-input=false", // do not prompt for inputs
		fmt.Sprintf("-state=%s", c.StateFile),
	}
	for key, val := range inputs {
		args = append(args, "-var", fmt.Sprintf("'%s=%v'", key, val))
	}
	args = append(args, c.Source)

	output, err := runCmd(args)
	if err != nil {
		return fmt.Errorf("terraform command failed.\nError: %s\nOutput: %s", err, output)
	}
	return nil
}

func (c Client) Output() (map[string]interface{}, error) {

	if c.StateFile == "" {
		return nil, errors.New("Client.StateFile can not be empty")
	}

	rawOutput, err := runCmd([]string{
		"terraform",
		"output",
		fmt.Sprintf("-state=%s", c.StateFile),
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
