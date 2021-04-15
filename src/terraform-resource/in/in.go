package in

import (
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/ljfranklin/terraform-resource/encoder"
	"github.com/ljfranklin/terraform-resource/logger"
	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/terraform"
)

type Runner struct {
	OutputDir string
	LogWriter io.Writer
}

type EnvNotFoundError error

var ErrOutputModule error = errors.New("the `output_module` feature was removed in Terraform 0.12.0, you must now explicitly declare all outputs in the root module")

func (r Runner) Run(req models.InRequest) (models.InResponse, error) {
	if err := req.Version.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Invalid Version request: %s", err)
	}

	envName := req.Version.EnvName
	nameFilepath := path.Join(r.OutputDir, "name")
	if err := ioutil.WriteFile(nameFilepath, []byte(envName), 0644); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to create name file at path '%s': %s", nameFilepath, err)
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
	terraformModel := req.Source.Terraform.Merge(req.Params.Terraform)
	if err := terraformModel.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to validate terraform Model: %s", err)
	}
	terraformModel.Source = "."
	if req.Params.OutputModule != "" {
		return models.InResponse{}, ErrOutputModule
	}

	targetEnvName := req.Version.EnvName

	client := terraform.NewClient(
		terraformModel,
		r.LogWriter,
	)

	if err := client.InitWithBackend(); err != nil {
		return models.InResponse{}, err
	}

	if req.Version.IsPlan() {
		if req.Params.OutputJSONPlanfile {
			if err := r.writeJSONPlanToFile(targetEnvName+"-plan", client); err != nil {
				return models.InResponse{}, err
			}
		}

		// HACK: Attempt to download a statefile if one exists, but silently ignore
		// any errors on failure. This is a workaround for an intermittent issue
		// where generating and applying a plan within the same job will incorrectly
		// mark the plan run as the latest version. This results in missing `metadata`
		// file on subsequent `get` calls. Issue:
		// https://github.com/ljfranklin/terraform-resource/issues/136. A better long-term
		// fix would be to make `check` more robust by updating Terraform to record
		// timestamps in the statefile: https://github.com/hashicorp/terraform/issues/15950.
		_, _ = r.writeBackendOutputs(req, targetEnvName, client)

		resp := models.InResponse{
			Version: req.Version,
		}

		return resp, nil
	}

	return r.writeBackendOutputs(req, targetEnvName, client)
}

func (r Runner) writeBackendOutputs(req models.InRequest, targetEnvName string, client terraform.Client) (models.InResponse, error) {
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
	stateVersion, err := client.CurrentStateVersion(targetEnvName)
	if err != nil {
		return models.InResponse{}, err
	}

	metadata, err := r.sanitizedOutput(result, client)
	if err != nil {
		return models.InResponse{}, err
	}

	resp := models.InResponse{
		Version: models.Version{
			EnvName: targetEnvName,
			Serial:  strconv.Itoa(stateVersion.Serial),
			Lineage: stateVersion.Lineage,
		},
		Metadata: metadata,
	}
	return resp, nil
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

func (r Runner) writeJSONPlanToFile(envName string, client terraform.Client) error {
	tfOutput, err := client.Output(envName)
	if err != nil {
		return err
	}

	planFilePath := path.Join(r.OutputDir, "plan.json")

	var encodedPlan string
	if val, ok := tfOutput[models.PlanContentJSON]; ok {
		encodedPlan = val["value"].(string)
	} else {
		return fmt.Errorf("state has no output for key %s", models.PlanContentJSON)
	}

	// Base64 decode then gunzip the JSON plan
	rawPlanReader := strings.NewReader(encodedPlan)
	decodedReader := base64.NewDecoder(base64.StdEncoding, rawPlanReader)
	zr, err := gzip.NewReader(decodedReader)
	if err != nil {
		return err
	}
	outputFile, err := os.OpenFile(planFilePath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	if _, err := io.Copy(outputFile, zr); err != nil {
		return err
	}

	if err := zr.Close(); err != nil {
		return err
	}

	if err := outputFile.Close(); err != nil {
		return err
	}

	return nil
}

func (r Runner) writeLegacyStateToFile(localStatefilePath string) error {
	stateFilePath := path.Join(r.OutputDir, "terraform.tfstate")
	stateContents, err := ioutil.ReadFile(localStatefilePath)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(stateFilePath, stateContents, 0777)
}

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

	stateFile, err := r.stateFileFromLegacyStorage(req, tmpDir)
	if err != nil {
		return models.InResponse{}, err
	}

	terraformModel := models.Terraform{
		StateFileLocalPath:  stateFile.LocalPath,
		StateFileRemotePath: stateFile.RemotePath,
	}

	if req.Params.OutputModule != "" {
		return models.InResponse{}, ErrOutputModule
	}

	if err := terraformModel.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to validate terraform Model: %s", err)
	}

	client := terraform.NewClient(
		terraformModel,
		r.LogWriter,
	)

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

func (r Runner) stateFileFromLegacyStorage(req models.InRequest, tmpDir string) (storage.StateFile, error) {
	storageModel := req.Source.Storage
	if err := storageModel.Validate(); err != nil {
		return storage.StateFile{}, fmt.Errorf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	stateFile := storage.StateFile{
		LocalPath:     path.Join(tmpDir, "terraform.tfstate"),
		RemotePath:    fmt.Sprintf("%s.tfstate", req.Version.EnvName),
		StorageDriver: storageDriver,
	}

	existsAsTainted, err := stateFile.ExistsAsTainted()
	if err != nil {
		return storage.StateFile{}, fmt.Errorf("Failed to check for tainted state file: %s", err)
	}
	if existsAsTainted {
		stateFile = stateFile.ConvertToTainted()
	}

	exists, err := stateFile.Exists()
	if err != nil {
		return storage.StateFile{}, fmt.Errorf("Failed to check for existing state file: %s", err)
	}
	if !exists {
		return storage.StateFile{}, EnvNotFoundError(fmt.Errorf(
			"State file does not exist with key '%s'."+
				"\nIf you intended to run the `destroy` action, add `put.get_params.action: destroy`."+
				"\nThis is a temporary requirement until Concourse supports a `delete` step.",
			stateFile.RemotePath,
		))
	}
	return stateFile, nil
}
