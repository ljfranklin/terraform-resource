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

	"github.com/ljfranklin/terraform-resource/models"
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

	storageKey := req.Source.Key
	if storageKey == "" {
		log.Fatal("Must specify 'key' under resource.source")
	}
	if req.Params.TerraformSource == "" {
		log.Fatal("Must specify 'terraform_source' under put params")
	}

	storageDriver, err := buildStorageDriver(req)
	if err != nil {
		log.Fatal(err.Error())
	}

	stateFilePath := path.Join(tmpDir, "terraform.tfstate")
	client := terraform.Client{
		Source:        req.Params.TerraformSource,
		StateFilePath: stateFilePath,
	}

	resp := models.OutResponse{}
	if req.Params.Action == models.DestroyAction {
		resp, err = performDestroy(stateFilePath, req, client, storageDriver)
	} else {
		resp, err = performApply(stateFilePath, req, client, storageDriver)
	}
	if err != nil {
		log.Fatalf("Failed to run terraform with action '%s': %s", req.Params.Action, err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write OutResponse: %s", err)
	}
}

func buildStorageDriver(req models.OutRequest) (storage.Storage, error) {
	driverType := req.Source.StorageDriver
	if driverType == "" {
		driverType = models.S3Driver
	}

	var storageDriver storage.Storage
	switch driverType {
	case models.S3Driver:
		storageDriver = storage.NewS3(
			req.Source.AccessKeyID,
			req.Source.SecretAccessKey,
			req.Source.RegionName,
			req.Source.Bucket,
		)
	default:
		supportedDrivers := []string{models.S3Driver}
		return nil, fmt.Errorf("Unknown storage_driver '%s'. Supported drivers are: %v", driverType, strings.Join(supportedDrivers, ", "))
	}

	return storageDriver, nil
}

func performApply(stateFilePath string, req models.OutRequest, client terraform.Client, storageDriver storage.Storage) (models.OutResponse, error) {
	var nilResponse models.OutResponse

	storageKey := req.Source.Key
	version, err := storageDriver.Version(storageKey)
	if err != nil {
		return nilResponse, fmt.Errorf("Failed to check for existing state file from '%s': %s", storageKey, err)
	}
	if version != "" {
		stateFile, createErr := os.Create(stateFilePath)
		if createErr != nil {
			return nilResponse, fmt.Errorf("Failed to create state file at '%s': %s", stateFilePath, createErr)
		}
		defer stateFile.Close()

		err = storageDriver.Download(storageKey, stateFile)
		if err != nil {
			return nilResponse, fmt.Errorf("Failed to download state file: %s", err)
		}
		stateFile.Close()
	}

	if err = client.Apply(req.Params.TerraformVars); err != nil {
		return nilResponse, fmt.Errorf("Failed to run terraform apply.\nError: %s", err)
	}
	stateFile, err := os.Open(stateFilePath)
	if err != nil {
		return nilResponse, fmt.Errorf("Failed to open state file at '%s'", stateFilePath)
	}
	defer stateFile.Close()

	err = storageDriver.Upload(storageKey, stateFile)
	if err != nil {
		return nilResponse, fmt.Errorf("Failed to upload state file: %s", err)
	}

	version, err = storageDriver.Version(storageKey)
	if err != nil {
		return nilResponse, fmt.Errorf("Failed to retrieve version from '%s': %s", storageKey, err)
	}
	if version == "" {
		return nilResponse, fmt.Errorf("Couldn't find state file at: %s", storageKey)
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

func performDestroy(stateFilePath string, req models.OutRequest, client terraform.Client, storageDriver storage.Storage) (models.OutResponse, error) {
	var nilResponse models.OutResponse
	stateFile, createErr := os.Create(stateFilePath)
	if createErr != nil {
		return nilResponse, fmt.Errorf("Failed to create state file at '%s': %s", stateFilePath, createErr)
	}
	defer stateFile.Close()

	err := storageDriver.Download(req.Source.Key, stateFile)
	if err != nil {
		return nilResponse, fmt.Errorf("Failed to download state file: %s", err)
	}
	stateFile.Close()

	if err = client.Destroy(req.Params.TerraformVars); err != nil {
		return nilResponse, fmt.Errorf("Failed to run terraform destroy.\nError: %s", err)
	}

	err = storageDriver.Delete(req.Source.Key)
	if err != nil {
		return nilResponse, fmt.Errorf("Failed to delete state file: %s", err)
	}

	version := time.Now().UTC().Format(time.RFC3339)

	resp := models.OutResponse{
		Version: models.Version{
			Version: version,
		},
		Metadata: []models.MetadataField{},
	}
	return resp, nil
}
