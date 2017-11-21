package in

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"

	"terraform-resource/encoder"
	"terraform-resource/logger"
	"terraform-resource/models"
	"terraform-resource/storage"
	"terraform-resource/terraform"
)

type Runner struct {
	OutputDir string
	LogWriter io.Writer
}

type EnvNotFoundError error

func (r Runner) Run(req models.InRequest) (models.InResponse, error) {
	if err := req.Version.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Invalid Version request: %s", err)
	}

	if req.Params.Action == models.DestroyAction {
		resp := models.InResponse{
			Version: req.Version,
		}
		return resp, nil
	}

	if err := req.Source.Validate(); err != nil {
		return models.InResponse{}, err
	}

	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-in")
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to create tmp dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	var resp models.InResponse
	if req.Source.BackendType != "" && req.Source.MigratedFromStorage != (storage.Model{}) {
		resp, err = r.inWithMigratedFromStorage(req, tmpDir)
	} else if req.Source.BackendType != "" {
		resp, err = r.inWithBackend(req, tmpDir)
	} else {
		resp, err = r.inWithLegacyStorage(req, tmpDir)
	}
	if err != nil {
		return models.InResponse{}, err
	}

	if err = r.writeNameToFile(req.Version.EnvName); err != nil {
		return models.InResponse{}, err
	}

	return resp, nil
}

func (r Runner) inWithMigratedFromStorage(req models.InRequest, tmpDir string) (models.InResponse, error) {
	resp, err := r.inWithBackend(req, tmpDir)
	if err == nil {
		return resp, nil
	}

	if _, ok := err.(EnvNotFoundError); ok {
		req.Source.Storage = req.Source.MigratedFromStorage
		return r.inWithLegacyStorage(req, tmpDir)
	}

	return models.InResponse{}, err
}

func (r Runner) inWithBackend(req models.InRequest, tmpDir string) (models.InResponse, error) {
	if req.Version.IsPlan() {
		resp := models.InResponse{
			Version: req.Version,
		}
		return resp, nil
	}

	terraformModel := req.Source.Terraform
	if err := terraformModel.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to validate terraform Model: %s", err)
	}
	terraformModel.Source = "."
	if req.Params.OutputModule != "" {
		terraformModel.OutputModule = req.Params.OutputModule
	}
	targetEnvName := req.Version.EnvName

	client := terraform.NewClient(
		terraformModel,
		r.LogWriter,
	)

	if err := client.InitWithBackend(); err != nil {
		return models.InResponse{}, err
	}

	if err := r.ensureEnvExistsInBackend(targetEnvName, client); err != nil {
		return models.InResponse{}, err
	}

	tfOutput, err := client.Output(targetEnvName)
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to parse terraform output.\nError: %s", err)
	}
	result := terraform.Result{
		Output: tfOutput,
	}

	if err = r.writeRawOutputToFile(result); err != nil {
		return models.InResponse{}, err
	}

	if req.Params.OutputStatefile {
		if err = r.writeBackendStateToFile(targetEnvName, client); err != nil {
			return models.InResponse{}, err
		}
	}

	serial, err := r.getCurrentSerial(client, targetEnvName)
	if err != nil {
		return models.InResponse{}, err
	}

	metadata, err := r.sanitizedOutput(result, client)
	if err != nil {
		return models.InResponse{}, err
	}

	resp := models.InResponse{
		Version:  models.Version{EnvName: targetEnvName, Serial: serial},
		Metadata: metadata,
	}
	return resp, nil
}

// TODO: extract this somewhere
func (r Runner) getCurrentSerial(client terraform.Client, envName string) (string, error) {
	rawState, err := client.StatePull(envName)
	if err != nil {
		return "", err
	}

	tfState := map[string]interface{}{}
	if err = json.Unmarshal(rawState, &tfState); err != nil {
		return "", fmt.Errorf("Failed to unmarshal JSON output.\nError: %s\nOutput: %s", err, rawState)
	}

	serial, ok := tfState["serial"].(float64)
	if !ok {
		return "", fmt.Errorf("Expected number value for 'serial' but got '%#v'", tfState["serial"])
	}

	return strconv.Itoa(int(serial)), nil
}

func (r Runner) ensureEnvExistsInBackend(envName string, client terraform.Client) error {
	spaces, err := client.WorkspaceList()
	if err != nil {
		return err
	}
	foundEnv := false
	for _, space := range spaces {
		if space == envName {
			foundEnv = true
		}
	}
	if !foundEnv {
		return EnvNotFoundError(fmt.Errorf(
			"Workspace '%s' does not exist in backend."+
				"\nIf you intended to run the `destroy` action, add `put.get_params.action: destroy`."+
				"\nThis is a temporary requirement until Concourse supports a `delete` step.",
			envName,
		))
	}

	return nil
}

func (r Runner) writeNameToFile(envName string) error {
	nameFilepath := path.Join(r.OutputDir, "name")
	return ioutil.WriteFile(nameFilepath, []byte(envName), 0644)
}

func (r Runner) writeRawOutputToFile(result terraform.Result) error {
	outputFilepath := path.Join(r.OutputDir, "metadata")
	outputFile, err := os.Create(outputFilepath)
	if err != nil {
		return fmt.Errorf("Failed to create output file at path '%s': %s", outputFilepath, err)
	}

	if err = encoder.NewJSONEncoder(outputFile).Encode(result.RawOutput()); err != nil {
		return fmt.Errorf("Failed to write output file: %s", err)
	}

	return nil
}

func (r Runner) writeBackendStateToFile(envName string, client terraform.Client) error {
	stateFilePath := path.Join(r.OutputDir, "terraform.tfstate")
	stateContents, err := client.StatePull(envName)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(stateFilePath, stateContents, 0777)
}

func (r Runner) writeLegacyStateToFile(localStatefilePath string) error {
	stateFilePath := path.Join(r.OutputDir, "terraform.tfstate")
	stateContents, err := ioutil.ReadFile(localStatefilePath)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(stateFilePath, stateContents, 0777)
}

// TODO: duplication with out.go
func (r Runner) sanitizedOutput(result terraform.Result, client terraform.Client) ([]models.MetadataField, error) {
	metadata := []models.MetadataField{}
	for key, value := range result.SanitizedOutput() {
		metadata = append(metadata, models.MetadataField{
			Name:  key,
			Value: value,
		})
	}

	tfVersion, err := client.Version()
	if err != nil {
		return nil, err
	}
	return append(metadata, models.MetadataField{
		Name:  "terraform_version",
		Value: tfVersion,
	}), nil
}

func (r Runner) inWithLegacyStorage(req models.InRequest, tmpDir string) (models.InResponse, error) {
	logger := logger.Logger{
		Sink: r.LogWriter,
	}
	logger.Warn(fmt.Sprintf("%s\n", storage.DeprecationWarning))

	if req.Version.IsPlan() {
		resp := models.InResponse{
			Version: req.Version,
		}
		return resp, nil
	}

	storageModel := req.Source.Storage
	if err := storageModel.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	// TODO: push down knowledge that statefile ends in `.tfstate`
	stateFilename := fmt.Sprintf("%s.tfstate", req.Version.EnvName)

	if err := r.ensureEnvExistsInLegacyStorage(stateFilename, storageDriver); err != nil {
		return models.InResponse{}, err
	}

	terraformModel := models.Terraform{
		StateFileLocalPath:  path.Join(tmpDir, "terraform.tfstate"),
		StateFileRemotePath: stateFilename,
	}

	if req.Params.OutputModule != "" {
		terraformModel.OutputModule = req.Params.OutputModule
	}

	if err := terraformModel.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to validate terraform Model: %s", err)
	}

	client := terraform.NewClient(
		terraformModel,
		r.LogWriter,
	)
	stateFile := storage.StateFile{
		LocalPath:     terraformModel.StateFileLocalPath,
		RemotePath:    terraformModel.StateFileRemotePath,
		StorageDriver: storageDriver,
	}

	storageVersion, err := stateFile.Download()
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to download state file from storage backend: %s", err)
	}
	version := models.NewVersionFromLegacyStorage(storageVersion)

	if err = client.InitWithoutBackend(); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to initialize terraform.\nError: %s", err)
	}

	tfOutput, err := client.OutputWithLegacyStorage()
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to parse terraform output.\nError: %s", err)
	}
	result := terraform.Result{
		Output: tfOutput,
	}

	if err = r.writeRawOutputToFile(result); err != nil {
		return models.InResponse{}, err
	}

	if req.Params.OutputStatefile {
		if err = r.writeLegacyStateToFile(terraformModel.StateFileLocalPath); err != nil {
			return models.InResponse{}, err
		}
	}

	metadata, err := r.sanitizedOutput(result, client)
	if err != nil {
		return models.InResponse{}, err
	}

	resp := models.InResponse{
		Version:  version,
		Metadata: metadata,
	}
	return resp, nil

}

func (r Runner) ensureEnvExistsInLegacyStorage(stateFilename string, storageDriver storage.Storage) error {
	storageVersion, err := storageDriver.Version(stateFilename)
	if err != nil {
		return fmt.Errorf("Failed to check for existing state file: %s", err)
	}
	if storageVersion.IsZero() {
		return EnvNotFoundError(fmt.Errorf(
			"State file does not exist with key '%s'."+
				"\nIf you intended to run the `destroy` action, add `put.get_params.action: destroy`."+
				"\nThis is a temporary requirement until Concourse supports a `delete` step.",
			stateFilename,
		))
	}
	return nil
}
