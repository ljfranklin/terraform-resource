package out

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"terraform-resource/models"
	"terraform-resource/namer"
	"terraform-resource/storage"
	"terraform-resource/terraform"
)

const (
	NameClashRetries = 10
)

type Runner struct {
	SourceDir string
	Namer     namer.Namer
	LogWriter io.Writer
}

func (r Runner) Run(req models.OutRequest) (models.OutResponse, error) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-out")
	if err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to create tmp dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	storageModel := req.Source.Storage
	if err = storageModel.Validate(); err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	terraformModel := req.Source.Terraform.Merge(req.Params.Terraform)
	if terraformModel.VarFile != "" {
		terraformModel.VarFile = path.Join(r.SourceDir, terraformModel.VarFile)
	}
	if err = terraformModel.ParseVarsFromFile(); err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to parse `terraform.var_file`: %s", err)
	}
	if len(terraformModel.Source) == 0 {
		return models.OutResponse{}, errors.New("Missing required field `terraform.source`")
	}

	envName, err := r.buildEnvName(req, storageDriver)
	if err != nil {
		return models.OutResponse{}, err
	}
	terraformModel.Vars["env_name"] = envName

	terraformModel.StateFileLocalPath = path.Join(tmpDir, "terraform.tfstate")
	terraformModel.StateFileRemotePath = fmt.Sprintf("%s.tfstate", envName)

	if err = terraformModel.Validate(); err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to validate terraform Model: %s", err)
	}

	client := terraform.Client{
		Model:         terraformModel,
		StorageDriver: storageDriver,
		LogWriter:     r.LogWriter,
	}

	stateFileExists, err := client.DoesStateFileExist()
	if err != nil {
		return models.OutResponse{}, err
	}
	if stateFileExists {
		_, err = client.DownloadStateFile()
		if err != nil {
			return models.OutResponse{}, err
		}
		outputs, err := client.Output()
		if err != nil {
			return models.OutResponse{}, err
		}
		client.Model = models.Terraform{Vars: outputs}.Merge(client.Model)
	}

	resp := models.OutResponse{}
	if req.Params.Action == models.DestroyAction {
		resp, err = performDestroy(client, storageDriver)
	} else {
		resp, err = performApply(client, storageDriver)
		if err != nil && terraformModel.DeleteOnFailure {
			_, destroyErr := performDestroy(client, storageDriver)
			if destroyErr != nil {
				err = fmt.Errorf("Apply Error: %s\nDestroyError: %s", err, destroyErr)
			}
		}
	}
	if err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to run terraform with action '%s': %s", req.Params.Action, err)
	}

	return resp, nil
}

func (r Runner) buildEnvName(req models.OutRequest, storageDriver storage.Storage) (string, error) {
	envName := ""
	if len(req.Params.EnvNameFile) > 0 {
		contents, err := ioutil.ReadFile(req.Params.EnvNameFile)
		if err != nil {
			return "", fmt.Errorf("Failed to read `env_name_file`: %s", err)
		}
		envName = string(contents)
	} else if len(req.Params.EnvName) > 0 {
		envName = req.Params.EnvName
	} else if req.Params.GenerateRandomName {
		randomName := ""
		for i := 0; i < NameClashRetries; i++ {
			randomName = r.Namer.RandomName()
			clash, err := doesEnvNameClash(randomName, storageDriver)
			if err != nil {
				return "", err
			}
			if clash == false {
				envName = randomName
				break
			}
		}
		if len(envName) == 0 {
			return "", fmt.Errorf("Failed to generate a non-clashing random name after %d attempts", NameClashRetries)
		}
	}

	if len(envName) == 0 {
		return "", fmt.Errorf("Must specify `put.params.env_name`, `put.params.env_name_file`, or `put.params.generate_random_name`")
	}
	envName = strings.TrimSpace(envName)
	envName = strings.Replace(envName, " ", "-", -1)

	return envName, nil
}

func performApply(client terraform.Client, storageDriver storage.Storage) (models.OutResponse, error) {
	var nilResponse models.OutResponse

	if err := client.Apply(); err != nil {
		return nilResponse, fmt.Errorf("Failed to run terraform apply.\nError: %s", err)
	}

	storageVersion, err := client.UploadStateFile()
	if err != nil {
		return nilResponse, fmt.Errorf("Failed to upload state file: %s", err)
	}
	version := models.NewVersion(storageVersion)

	clientOutput, err := client.Output()
	if err != nil {
		return nilResponse, fmt.Errorf("Failed to terraform output.\nError: %s", err)
	}

	metadata := []models.MetadataField{}
	for key, value := range clientOutput {
		metadata = append(metadata, models.MetadataField{
			Name:  key,
			Value: value,
		})
	}

	resp := models.OutResponse{
		Version:  version,
		Metadata: metadata,
	}
	return resp, nil
}

func performDestroy(client terraform.Client, storageDriver storage.Storage) (models.OutResponse, error) {
	var nilResponse models.OutResponse

	if err := client.Destroy(); err != nil {
		return nilResponse, fmt.Errorf("Failed to run terraform destroy.\nError: %s", err)
	}

	storageVersion, err := client.DeleteStateFile()
	if err != nil {
		return nilResponse, fmt.Errorf("Failed to delete state file: %s", err)
	}
	version := models.NewVersion(storageVersion)

	resp := models.OutResponse{
		Version:  version,
		Metadata: []models.MetadataField{},
	}
	return resp, nil
}

func doesEnvNameClash(envName string, storageDriver storage.Storage) (bool, error) {
	filename := fmt.Sprintf("%s.tfstate", envName)
	version, err := storageDriver.Version(filename)
	if err != nil {
		return false, err
	}
	return version.IsZero() == false, nil
}
