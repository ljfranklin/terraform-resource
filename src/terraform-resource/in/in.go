package in

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"terraform-resource/encoder"
	"terraform-resource/models"
	"terraform-resource/storage"
	"terraform-resource/terraform"
)

type Runner struct {
	OutputDir string
}

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

	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-in")
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to create tmp dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	storageModel := req.Source.Storage
	if err = storageModel.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	stateFilename := fmt.Sprintf("%s.tfstate", req.Version.EnvName)
	storageVersion, err := storageDriver.Version(stateFilename)
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to check for existing state file: %s", err)
	}
	if storageVersion.IsZero() {
		if req.Version.IsPlan() {
			resp := models.InResponse{
				Version: req.Version,
			}
			return resp, nil
		}

		return models.InResponse{}, fmt.Errorf(
			"State file does not exist with key '%s'."+
				"\nIf you intended to run the `destroy` action, add `put.get_params.action: destroy`."+
				"\nThis is a temporary requirement until Concourse supports a `delete` step.",
			stateFilename,
		)
	}

	terraformModel := models.Terraform{
		StateFileLocalPath:  path.Join(tmpDir, "terraform.tfstate"),
		StateFileRemotePath: stateFilename,
	}
	if err = terraformModel.Validate(); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to validate terraform Model: %s", err)
	}

	client := terraform.Client{
		Model:         terraformModel,
		StorageDriver: storageDriver,
	}
	stateFile := terraform.StateFile{
		LocalPath:     terraformModel.StateFileLocalPath,
		RemotePath:    terraformModel.StateFileRemotePath,
		StorageDriver: storageDriver,
	}

	storageVersion, err = stateFile.Download()
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to download state file from storage backend: %s", err)
	}
	version := models.NewVersion(storageVersion)

	nameFilepath := path.Join(r.OutputDir, "name")
	nameFile, err := os.Create(nameFilepath)
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to create name file at path '%s': %s", nameFilepath, err)
	}
	defer nameFile.Close()
	nameFile.WriteString(version.EnvName)

	tfOutput, err := client.Output()
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to parse terraform output.\nError: %s", err)
	}
	result := terraform.Result{
		Output: tfOutput,
	}

	outputFilepath := path.Join(r.OutputDir, "metadata")
	outputFile, err := os.Create(outputFilepath)
	if err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to create output file at path '%s': %s", outputFilepath, err)
	}

	if err := encoder.NewJSONEncoder(outputFile).Encode(result.RawOutput()); err != nil {
		return models.InResponse{}, fmt.Errorf("Failed to write output file: %s", err)
	}

	metadata := []models.MetadataField{}
	for key, value := range result.SanitizedOutput() {
		metadata = append(metadata, models.MetadataField{
			Name:  key,
			Value: value,
		})
	}

	tfVersion, err := client.Version()
	if err != nil {
		return models.InResponse{}, err
	}
	metadata = append(metadata, models.MetadataField{
		Name:  "terraform_version",
		Value: tfVersion,
	})

	if req.Params.OutputStatefile {
		stateFilePath := path.Join(r.OutputDir, "terraform.tfstate")
		stateContents, err := ioutil.ReadFile(terraformModel.StateFileLocalPath)
		if err != nil {
			return models.InResponse{}, err
		}
		err = ioutil.WriteFile(stateFilePath, stateContents, 0777)
		if err != nil {
			return models.InResponse{}, err
		}
	}

	resp := models.InResponse{
		Version:  version,
		Metadata: metadata,
	}
	return resp, nil
}
