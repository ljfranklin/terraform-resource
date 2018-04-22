package out

import (
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
	"terraform-resource/ssh"
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
	logger := logger.Logger{
		Sink: r.LogWriter,
	}

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
	if terraformModel.VarFiles != nil {
		for i := range terraformModel.VarFiles {
			terraformModel.VarFiles[i] = path.Join(r.SourceDir, terraformModel.VarFiles[i])
		}
	}
	if err = terraformModel.ParseVarsFromFiles(); err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to parse `terraform.var_files`: %s", err)
	}
	if err = terraformModel.ParseImportsFromFile(); err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to parse `terraform.imports_file`: %s", err)
	}

	if len(terraformModel.Source) == 0 && terraformModel.PlanRun == false {
		return models.OutResponse{}, errors.New("Missing required field `terraform.source`")
	}

	envName, err := r.buildEnvName(req, storageDriver)
	if err != nil {
		return models.OutResponse{}, err
	}
	terraformModel.Vars["env_name"] = envName
	terraformModel.Vars["build_id"] = os.Getenv("BUILD_ID")
	terraformModel.Vars["build_name"] = os.Getenv("BUILD_NAME")
	terraformModel.Vars["build_job_name"] = os.Getenv("BUILD_JOB_NAME")
	terraformModel.Vars["build_pipeline_name"] = os.Getenv("BUILD_PIPELINE_NAME")
	terraformModel.Vars["build_team_name"] = os.Getenv("BUILD_TEAM_NAME")
	terraformModel.Vars["atc_external_url"] = os.Getenv("ATC_EXTERNAL_URL")

	terraformModel.PlanFileLocalPath = path.Join(tmpDir, "plan")
	terraformModel.PlanFileRemotePath = fmt.Sprintf("%s.plan", envName)
	terraformModel.StateFileLocalPath = path.Join(tmpDir, "terraform.tfstate")
	terraformModel.StateFileRemotePath = fmt.Sprintf("%s.tfstate", envName)

	if err = terraformModel.Validate(); err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to validate terraform Model: %s", err)
	}

	if terraformModel.PrivateKey != "" {
		agent, err := ssh.SpawnAgent()
		if err != nil {
			return models.OutResponse{}, err
		}
		defer agent.Shutdown()

		if err = agent.AddKey([]byte(terraformModel.PrivateKey)); err != nil {
			return models.OutResponse{}, err
		}

		if err = os.Setenv("SSH_AUTH_SOCK", agent.SSHAuthSock()); err != nil {
			return models.OutResponse{}, err
		}
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
		Logger:          logger,
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
	if req.Params.PlanOnly {
		version.PlanOnly = "true" // Concourse demands version fields are strings
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
		return models.OutResponse{}, err
	}
	metadata = append(metadata, models.MetadataField{
		Name:  "terraform_version",
		Value: tfVersion,
	})

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
