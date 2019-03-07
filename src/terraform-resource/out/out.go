package out

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"terraform-resource/logger"
	"terraform-resource/models"
	"terraform-resource/namer"
	"terraform-resource/ssh"
	"terraform-resource/storage"
	"terraform-resource/terraform"
)

type Runner struct {
	SourceDir string
	Namer     namer.Namer
	LogWriter io.Writer
}

func (r Runner) Run(req models.OutRequest) (models.OutResponse, error) {
	if err := req.Source.Validate(); err != nil {
		return models.OutResponse{}, err
	}

	req.Source.Terraform = req.Source.Terraform.Merge(req.Params.Terraform)
	terraformModel, err := r.buildTerraformModel(req)
	if err != nil {
		return models.OutResponse{}, err
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

	if req.Source.BackendType != "" && req.Source.MigratedFromStorage != (storage.Model{}) {
		return r.runWithMigratedFromStorage(req, terraformModel)
	} else if req.Source.BackendType == "" {
		return r.runWithLegacyStorage(req, terraformModel)
	}
	return r.runWithBackend(req, terraformModel)
}

func (r Runner) runWithBackend(req models.OutRequest, terraformModel models.Terraform) (models.OutResponse, error) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-out")
	if err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to create tmp dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	envName, err := r.buildEnvName(req, terraformModel)
	if err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to create env name: %s", err)
	}

	terraformModel.Vars["env_name"] = envName
	terraformModel.PlanFileLocalPath = path.Join(tmpDir, "plan")

	client := terraform.NewClient(
		terraformModel,
		r.LogWriter,
	)

	action := terraform.Action{
		Client:  client,
		EnvName: envName,
		Model:   terraformModel,
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

	version := result.Version
	if req.Params.PlanOnly {
		version.PlanOnly = "true" // Concourse demands version fields are strings
	}

	metadata, err := r.buildMetadata(result.SanitizedOutput(), client)
	if err != nil {
		return models.OutResponse{}, actionErr
	}

	resp := models.OutResponse{
		Version:  version,
		Metadata: metadata,
	}

	return resp, nil
}

func (r Runner) runWithLegacyStorage(req models.OutRequest, terraformModel models.Terraform) (models.OutResponse, error) {
	logger := logger.Logger{
		Sink: r.LogWriter,
	}
	logger.Warn(fmt.Sprintf("%s\n", storage.DeprecationWarning))

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

	envName, err := r.buildEnvNameFromLegacyStorage(req, storageDriver)
	if err != nil {
		return models.OutResponse{}, err
	}
	terraformModel.Vars["env_name"] = envName

	terraformModel.PlanFileLocalPath = path.Join(tmpDir, "plan")
	terraformModel.PlanFileRemotePath = fmt.Sprintf("%s.plan", envName)
	terraformModel.StateFileLocalPath = path.Join(tmpDir, "terraform.tfstate")
	terraformModel.StateFileRemotePath = fmt.Sprintf("%s.tfstate", envName)

	client := terraform.NewClient(
		terraformModel,
		r.LogWriter,
	)
	stateFile := storage.StateFile{
		LocalPath:     terraformModel.StateFileLocalPath,
		RemotePath:    terraformModel.StateFileRemotePath,
		StorageDriver: storageDriver,
	}
	planFile := storage.PlanFile{
		LocalPath:     terraformModel.PlanFileLocalPath,
		RemotePath:    terraformModel.PlanFileRemotePath,
		StorageDriver: storageDriver,
	}
	action := terraform.LegacyStorageAction{
		Client:    client,
		StateFile: stateFile,
		PlanFile:  planFile,
		Model:     terraformModel,
		Logger:    logger,
	}

	var result terraform.LegacyStorageResult
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

	version := models.NewVersionFromLegacyStorage(result.Version)
	if req.Params.PlanOnly {
		version.PlanOnly = "true" // Concourse demands version fields are strings
	}

	metadata, err := r.buildMetadata(result.SanitizedOutput(), client)
	if err != nil {
		return models.OutResponse{}, actionErr
	}

	resp := models.OutResponse{
		Version:  version,
		Metadata: metadata,
	}

	return resp, nil
}

func (r Runner) runWithMigratedFromStorage(req models.OutRequest, terraformModel models.Terraform) (models.OutResponse, error) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-out")
	if err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to create tmp dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	storageModel := req.Source.MigratedFromStorage
	if err = storageModel.Validate(); err != nil {
		return models.OutResponse{}, fmt.Errorf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	envName, err := r.buildEnvNameFromMigrated(req, terraformModel, storageDriver)
	if err != nil {
		return models.OutResponse{}, err
	}

	terraformModel.Vars["env_name"] = envName
	terraformModel.PlanFileLocalPath = path.Join(tmpDir, "plan")

	client := terraform.NewClient(
		terraformModel,
		r.LogWriter,
	)

	terraformModel.StateFileLocalPath = path.Join(tmpDir, "terraform.tfstate")
	terraformModel.StateFileRemotePath = fmt.Sprintf("%s.tfstate", envName)

	stateFile := storage.StateFile{
		LocalPath:     terraformModel.StateFileLocalPath,
		RemotePath:    terraformModel.StateFileRemotePath,
		StorageDriver: storageDriver,
	}
	action := terraform.MigratedFromStorageAction{
		StateFile: stateFile,
		Client:    client,
		EnvName:   envName,
		Model:     terraformModel,
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

	version := result.Version
	if req.Params.PlanOnly {
		version.PlanOnly = "true" // Concourse demands version fields are strings
	}

	metadata, err := r.buildMetadata(result.SanitizedOutput(), client)
	if err != nil {
		return models.OutResponse{}, actionErr
	}

	resp := models.OutResponse{
		Version:  version,
		Metadata: metadata,
	}

	return resp, nil
}

func (r Runner) buildEnvName(req models.OutRequest, terraformModel models.Terraform) (string, error) {
	tfClientWithoutWorkspace := terraform.NewClient(
		terraformModel,
		r.LogWriter,
	)
	namer := BackendEnvNamer{
		Req:             req,
		TerraformClient: tfClientWithoutWorkspace,
		Namer:           r.Namer,
	}
	return namer.EnvName()
}

func (r Runner) buildEnvNameFromLegacyStorage(req models.OutRequest, storageDriver storage.Storage) (string, error) {
	namer := LegacyStorageEnvNamer{
		Req:           req,
		StorageDriver: storageDriver,
		Namer:         r.Namer,
	}
	return namer.EnvName()
}

func (r Runner) buildEnvNameFromMigrated(req models.OutRequest, terraformModel models.Terraform, storageDriver storage.Storage) (string, error) {
	tfClientWithoutWorkspace := terraform.NewClient(
		terraformModel,
		r.LogWriter,
	)
	namer := MigratedFromStorageEnvNamer{
		Req:             req,
		StorageDriver:   storageDriver,
		Namer:           r.Namer,
		TerraformClient: tfClientWithoutWorkspace,
	}
	return namer.EnvName()
}

func (r Runner) buildTerraformModel(req models.OutRequest) (models.Terraform, error) {
	terraformModel := req.Source.Terraform
	if terraformModel.VarFiles != nil {
		for i := range terraformModel.VarFiles {
			terraformModel.VarFiles[i] = path.Join(r.SourceDir, terraformModel.VarFiles[i])
		}
	}
	if err := terraformModel.ParseVarsFromFiles(); err != nil {
		return models.Terraform{}, fmt.Errorf("Failed to parse `terraform.var_files`: %s", err)
	}
	if err := terraformModel.ParseImportsFromFile(); err != nil {
		return models.Terraform{}, fmt.Errorf("Failed to parse `terraform.imports_file`: %s", err)
	}

	if len(terraformModel.Source) == 0 {
		return models.Terraform{}, errors.New("Missing required field `terraform.source`")
	}

	terraformModel.Vars["build_id"] = os.Getenv("BUILD_ID")
	terraformModel.Vars["build_name"] = os.Getenv("BUILD_NAME")
	terraformModel.Vars["build_job_name"] = os.Getenv("BUILD_JOB_NAME")
	terraformModel.Vars["build_pipeline_name"] = os.Getenv("BUILD_PIPELINE_NAME")
	terraformModel.Vars["build_team_name"] = os.Getenv("BUILD_TEAM_NAME")
	terraformModel.Vars["atc_external_url"] = os.Getenv("ATC_EXTERNAL_URL")

	return terraformModel, nil
}

func (r Runner) buildMetadata(outputs map[string]string, client terraform.Client) ([]models.MetadataField, error) {
	metadata := []models.MetadataField{}
	for key, value := range outputs {
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
