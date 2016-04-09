package terraform

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/ljfranklin/terraform-resource/storage"
)

type Client struct {
	// Source can be a local directory or a valid Terraform module source:
	// https://www.terraform.io/docs/modules/
	Source             string
	StateFilePath      string
	StateFileRemoteKey string
	StorageDriver      storage.Storage
	OutputWriter       io.Writer
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

	initCmd := terraformCmd([]string{
		"init",
		c.Source,
		tmpDir,
	})
	if initOutput, initErr := initCmd.CombinedOutput(); initErr != nil {
		return fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", initErr, initOutput)
	}

	applyArgs := []string{
		"apply",
		"-backup='-'",  // no need to backup state file
		"-input=false", // do not prompt for inputs
		fmt.Sprintf("-state=%s", c.StateFilePath),
	}
	for key, val := range inputs {
		applyArgs = append(applyArgs, "-var", fmt.Sprintf("'%s=%v'", key, val))
	}
	applyArgs = append(applyArgs, tmpDir)

	applyCmd := terraformCmd(applyArgs)
	applyCmd.Stdout = c.OutputWriter
	applyCmd.Stderr = c.OutputWriter
	err = applyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c Client) Destroy(inputs map[string]interface{}) error {

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

	initCmd := terraformCmd([]string{
		"init",
		c.Source,
		tmpDir,
	})
	if initOutput, initErr := initCmd.CombinedOutput(); initErr != nil {
		return fmt.Errorf("terraform init command failed.\nError: %s\nOutput: %s", initErr, initOutput)
	}

	destroyArgs := []string{
		"destroy",
		"-backup='-'", // no need to backup state file
		"-force",      // do not prompt for confirmation
		fmt.Sprintf("-state=%s", c.StateFilePath),
	}
	for key, val := range inputs {
		destroyArgs = append(destroyArgs, "-var", fmt.Sprintf("'%s=%v'", key, val))
	}
	destroyArgs = append(destroyArgs, tmpDir)

	destroyCmd := terraformCmd(destroyArgs)
	destroyCmd.Stdout = c.OutputWriter
	destroyCmd.Stderr = c.OutputWriter
	err = destroyCmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run Terraform command: %s", err)
	}

	return nil
}

func (c Client) Output() (map[string]interface{}, error) {

	if c.StateFilePath == "" {
		return nil, errors.New("Client.StateFilePath can not be empty")
	}

	outputCmd := terraformCmd([]string{
		"output",
		fmt.Sprintf("-state=%s", c.StateFilePath),
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

func (c Client) DownloadStateFileIfExists() (string, error) {
	if c.StateFilePath == "" {
		return "", errors.New("Client.StateFilePath can not be empty")
	}
	if c.StateFileRemoteKey == "" {
		return "", errors.New("Client.StateFileRemoteKey can not be empty")
	}

	version, err := c.StorageDriver.Version(c.StateFileRemoteKey)
	if err != nil {
		return "", fmt.Errorf("Failed to check for existing state file from '%s': %s", c.StateFileRemoteKey, err)
	}
	if version != "" {
		stateFile, createErr := os.Create(c.StateFilePath)
		if createErr != nil {
			return "", fmt.Errorf("Failed to create state file at '%s': %s", c.StateFilePath, createErr)
		}
		defer stateFile.Close()

		err = c.StorageDriver.Download(c.StateFileRemoteKey, stateFile)
		if err != nil {
			return "", fmt.Errorf("Failed to download state file: %s", err)
		}
		stateFile.Close()
	}

	return version, nil
}

func (c Client) UploadStateFile() (string, error) {
	stateFile, err := os.Open(c.StateFilePath)
	if err != nil {
		return "", fmt.Errorf("Failed to open state file at '%s'", c.StateFilePath)
	}
	defer stateFile.Close()

	err = c.StorageDriver.Upload(c.StateFileRemoteKey, stateFile)
	if err != nil {
		return "", fmt.Errorf("Failed to upload state file: %s", err)
	}

	version, err := c.StorageDriver.Version(c.StateFileRemoteKey)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve version from '%s': %s", c.StateFileRemoteKey, err)
	}
	if version == "" {
		return "", fmt.Errorf("Couldn't find state file at: %s", c.StateFileRemoteKey)
	}

	return version, nil
}

func (c Client) DeleteStateFile() error {
	if err := c.StorageDriver.Delete(c.StateFileRemoteKey); err != nil {
		return fmt.Errorf("Failed to delete state file: %s", err)
	}
	return nil
}

func terraformCmd(args []string) *exec.Cmd {
	return exec.Command("/bin/bash", "-c", fmt.Sprintf("terraform %s", strings.Join(args, " ")))
}
