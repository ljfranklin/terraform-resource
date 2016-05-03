package out

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	baseModels "github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/namer"
	"github.com/ljfranklin/terraform-resource/out/models"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/terraform"
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

	terraformModel := req.Source.Terraform.Merge(req.Params.Terraform)
	if terraformModel.VarFile != "" {
		terraformModel.VarFile = path.Join(r.SourceDir, terraformModel.VarFile)
	}
	if err = terraformModel.ParseVarsFromFile(); err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to parse `terraform.var_file`: %s", err)
	}

	envName, err := r.buildEnvName(req)
	if err != nil {
		return models.OutResponse{}, err
	}
	terraformModel.Vars["env_name"] = envName

	terraformModel.StateFileLocalPath = path.Join(tmpDir, "terraform.tfstate")
	terraformModel.StateFileRemotePath = fmt.Sprintf("%s.tfstate", envName)

	if err = terraformModel.Validate(); err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to validate terraform Model: %s", err)
	}

	storageModel := req.Source.Storage
	if err = storageModel.Validate(); err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	client := terraform.Client{
		Model:         terraformModel,
		StorageDriver: storageDriver,
		LogWriter:     r.LogWriter,
	}

	_, err = client.DownloadStateFileIfExists()
	if err != nil {
		return models.OutResponse{}, err
	}

	resp := models.OutResponse{}
	if req.Params.Action == models.DestroyAction {
		resp, err = performDestroy(client, storageDriver)
	} else {
		resp, err = performApply(client, storageDriver)
	}
	if err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to run terraform with action '%s': %s", req.Params.Action, err)
	}

	return resp, nil
}

func (r Runner) buildEnvName(req models.OutRequest) (string, error) {
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
		envName = r.Namer.RandomName()
	}

	if len(envName) == 0 {
		return "", fmt.Errorf("Must specify `put.params.env_name`, `put.params.env_name_file`, or `put.params.generate_random_name`")
	}
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
	version := baseModels.NewVersion(storageVersion)

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
	version := baseModels.NewVersion(storageVersion)

	resp := models.OutResponse{
		Version:  version,
		Metadata: []models.MetadataField{},
	}
	return resp, nil
}
