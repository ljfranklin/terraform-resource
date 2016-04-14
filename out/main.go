package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ljfranklin/terraform-resource/out/models"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/terraform"
)

func main() {

	if len(os.Args) < 2 {
		log.Fatalf("Expected path to sources as first arg")
	}
	sourceDir := os.Args[1]
	if err := os.Chdir(sourceDir); err != nil {
		log.Fatalf("Failed to access source dir '%s': %s", sourceDir, err)
	}
	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-out")
	if err != nil {
		log.Fatalf("Failed to create tmp dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	req := models.OutRequest{}
	if err = json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("Failed to read OutRequest: %s", err)
	}

	if err = req.Validate(); err != nil {
		log.Fatalf("Failed to validate Check Request: %s", err)
	}

	terraformModel := req.Source.Terraform.Merge(req.Params.Terraform)
	if terraformModel.VarFile != "" {
		terraformModel.VarFile = path.Join(sourceDir, terraformModel.VarFile)
	}
	if err = terraformModel.ParseVarsFromFile(); err != nil {
		log.Fatalf("Failed to parse `terraform.var_file`: %s", err)
	}

	storageDriver, err := buildStorageDriver(req)
	if err != nil {
		log.Fatal(err.Error())
	}

	stateFilePath := path.Join(tmpDir, "terraform.tfstate")
	client := terraform.Client{
		Source:             req.TerraformSource(),
		StateFilePath:      stateFilePath,
		StateFileRemoteKey: req.Source.Storage.Key,
		StorageDriver:      storageDriver,
		OutputWriter:       os.Stderr,
	}

	_, err = client.DownloadStateFileIfExists()
	if err != nil {
		log.Fatal(err.Error())
	}

	resp := models.OutResponse{}
	if req.Params.Action == models.DestroyAction {
		resp, err = performDestroy(stateFilePath, terraformModel.Vars, client, storageDriver)
	} else {
		resp, err = performApply(stateFilePath, terraformModel.Vars, client, storageDriver)
	}
	if err != nil {
		log.Fatalf("Failed to run terraform with action '%s': %s", req.Params.Action, err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write OutResponse: %s", err)
	}
}

func buildStorageDriver(req models.OutRequest) (storage.Storage, error) {
	driverType := req.Source.Storage.Driver
	if driverType == "" {
		driverType = storage.S3Driver
	}

	var storageDriver storage.Storage
	switch driverType {
	case storage.S3Driver:
		storageDriver = storage.NewS3(
			req.Source.Storage.AccessKeyID,
			req.Source.Storage.SecretAccessKey,
			req.Source.Storage.RegionName,
			req.Source.Storage.Bucket,
		)
	default:
		supportedDrivers := []string{storage.S3Driver}
		return nil, fmt.Errorf("Unknown storage_driver '%s'. Supported drivers are: %v", driverType, strings.Join(supportedDrivers, ", "))
	}

	return storageDriver, nil
}

func performApply(stateFilePath string, terraformVars map[string]interface{}, client terraform.Client, storageDriver storage.Storage) (models.OutResponse, error) {
	var nilResponse models.OutResponse

	if err := client.Apply(terraformVars); err != nil {
		return nilResponse, fmt.Errorf("Failed to run terraform apply.\nError: %s", err)
	}

	version, err := client.UploadStateFile()
	if err != nil {
		return nilResponse, fmt.Errorf("Failed to upload state file: %s", err)
	}

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
		Version: models.Version{
			Version: version,
		},
		Metadata: metadata,
	}
	return resp, nil
}

func performDestroy(stateFilePath string, terraformVars map[string]interface{}, client terraform.Client, storageDriver storage.Storage) (models.OutResponse, error) {
	var nilResponse models.OutResponse

	if err := client.Destroy(terraformVars); err != nil {
		return nilResponse, fmt.Errorf("Failed to run terraform destroy.\nError: %s", err)
	}

	if err := client.DeleteStateFile(); err != nil {
		return nilResponse, err
	}

	// use current time rather than state file LastModified time
	version := time.Now().UTC().Format(time.RFC3339)

	resp := models.OutResponse{
		Version: models.Version{
			Version: version,
		},
		Metadata: []models.MetadataField{},
	}
	return resp, nil
}
