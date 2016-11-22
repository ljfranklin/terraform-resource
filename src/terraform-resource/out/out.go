package out

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"terraform-resource/logger"
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

	if len(terraformModel.Source) == 0 && terraformModel.PlanRun == false {
		return models.OutResponse{}, errors.New("Missing required field `terraform.source`")
	}

	envName, err := r.buildEnvName(req, storageDriver)
	if err != nil {
		return models.OutResponse{}, err
	}
	terraformModel.Vars["env_name"] = envName

	terraformModel.PlanFileLocalPath = path.Join(tmpDir, "plan")
	terraformModel.PlanFileRemotePath = fmt.Sprintf("%s.plan", envName)
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
	stateFile := terraform.StateFile{
		LocalPath:     terraformModel.StateFileLocalPath,
		RemotePath:    terraformModel.StateFileRemotePath,
		StorageDriver: storageDriver,
	}
	planFile := terraform.PlanFile{
		LocalPath:     terraformModel.PlanFileLocalPath,
		RemotePath:    terraformModel.PlanFileRemotePath,
		StorageDriver: storageDriver,
	}
	action := terraform.Action{
		Client:          client,
		StateFile:       stateFile,
		PlanFile:        planFile,
		DeleteOnFailure: terraformModel.DeleteOnFailure,
		Logger: logger.Logger{
			Sink: r.LogWriter,
		},
	}

	var result terraform.Result
	var actionErr error

	if req.Params.PlanOnly {
		result, actionErr = action.Plan()
	} else if req.Params.Action == models.DestroyAction {
		result, actionErr = action.Destroy()
	} else {
		result, actionErr = action.Apply()
	}
	if actionErr != nil {
		return models.OutResponse{}, actionErr
	}

	version := models.NewVersion(result.Version)

	metadata := []models.MetadataField{}
	for key, value := range result.Output {
		jsonValue, err := json.Marshal(value)
		if err != nil {
			jsonValue = []byte(fmt.Sprintf("Unable to parse output value for key '%s': %s", key, err))
		}
		metadata = append(metadata, models.MetadataField{
			Name:  key,
			Value: strings.Trim(string(jsonValue), "\""),
		})
	}
	resp := models.OutResponse{
		Version:  version,
		Metadata: metadata,
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

func doesEnvNameClash(envName string, storageDriver storage.Storage) (bool, error) {
	filename := fmt.Sprintf("%s.tfstate", envName)
	version, err := storageDriver.Version(filename)
	if err != nil {
		return false, err
	}
	return version.IsZero() == false, nil
}
