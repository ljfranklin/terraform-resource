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
	"time"

	"github.com/ljfranklin/terraform-resource/storage"
)

type Client struct {
	Model         Model
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

func (c Client) DownloadStateFileIfExists() (storage.Version, error) {
	version, err := c.StorageDriver.Version(c.Model.StateFileRemotePath)
	if err != nil {
		return storage.Version{}, fmt.Errorf("Failed to check for existing state file from '%s': %s", c.Model.StateFileRemotePath, err)
	}
	if version.IsZero() == false {
		stateFile, createErr := os.Create(c.Model.StateFileLocalPath)
		if createErr != nil {
			return storage.Version{}, fmt.Errorf("Failed to create state file at '%s': %s", c.Model.StateFileLocalPath, createErr)
		}
		defer stateFile.Close()

		err = c.StorageDriver.Download(c.Model.StateFileRemotePath, stateFile)
		if err != nil {
			return storage.Version{}, fmt.Errorf("Failed to download state file: %s", err)
		}
		stateFile.Close()
	}

	return version, nil
}

func (c Client) UploadStateFile() (storage.Version, error) {
	stateFile, err := os.Open(c.Model.StateFileLocalPath)
	if err != nil {
		return storage.Version{}, fmt.Errorf("Failed to open state file at '%s'", c.Model.StateFileLocalPath)
	}
	defer stateFile.Close()

	err = c.StorageDriver.Upload(c.Model.StateFileRemotePath, stateFile)
	if err != nil {
		return storage.Version{}, fmt.Errorf("Failed to upload state file: %s", err)
	}

	version, err := c.StorageDriver.Version(c.Model.StateFileRemotePath)
	if err != nil {
		return storage.Version{}, fmt.Errorf("Failed to retrieve version from '%s': %s", c.Model.StateFileRemotePath, err)
	}
	if version.IsZero() {
		return storage.Version{}, fmt.Errorf("Couldn't find state file at: %s", c.Model.StateFileRemotePath)
	}

	return version, nil
}

func (c Client) DeleteStateFile() (storage.Version, error) {
	if err := c.StorageDriver.Delete(c.Model.StateFileRemotePath); err != nil {
		return storage.Version{}, fmt.Errorf("Failed to delete state file: %s", err)
	}

	// use current time rather than state file LastModified time
	version := storage.Version{
		LastModified: time.Now().UTC().Format(storage.TimeFormat),
		StateFileKey: c.Model.StateFileRemotePath,
	}
	return version, nil
}

func terraformCmd(args []string) *exec.Cmd {
	return exec.Command("/bin/bash", "-c", fmt.Sprintf("terraform %s", strings.Join(args, " ")))
}
