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

	"gopkg.in/yaml.v2"

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

	terraformVars, err := getTerraformVariables(req, sourceDir)
	if err != nil {
		log.Fatal(err.Error())
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
		Source:             req.Params.TerraformSource,
		StateFilePath:      stateFilePath,
		StateFileRemoteKey: storageKey,
		StorageDriver:      storageDriver,
	}

	_, err = client.DownloadStateFileIfExists()
	if err != nil {
		log.Fatal(err.Error())
	}

	resp := models.OutResponse{}
	if req.Params.Action == models.DestroyAction {
		resp, err = performDestroy(stateFilePath, terraformVars, client, storageDriver)
	} else {
		resp, err = performApply(stateFilePath, terraformVars, client, storageDriver)
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

func getTerraformVariables(req models.OutRequest, sourceDir string) (models.TerraformVars, error) {
	terraformVars := models.TerraformVars{}
	for key, value := range req.Params.TerraformVars {
		terraformVars[key] = value
	}
	if req.Params.TerraformVarFile != "" {
		varFilePath := path.Join(sourceDir, req.Params.TerraformVarFile)
		fileContents, readErr := ioutil.ReadFile(varFilePath)
		if readErr != nil {
			return nil, fmt.Errorf("Failed to read TerraformVarFile at '%s': %s", varFilePath, readErr)
		}

		fileVars := map[string]interface{}{}
		readErr = yaml.Unmarshal(fileContents, &fileVars)
		if readErr != nil {
			return nil, fmt.Errorf("Failed to parse TerraformVarFile at '%s': %s", varFilePath, readErr)
		}

		for key, value := range fileVars {
			terraformVars[key] = value
		}
	}

	return terraformVars, nil
}

func performApply(stateFilePath string, terraformVars models.TerraformVars, client terraform.Client, storageDriver storage.Storage) (models.OutResponse, error) {
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

func performDestroy(stateFilePath string, terraformVars models.TerraformVars, client terraform.Client, storageDriver storage.Storage) (models.OutResponse, error) {
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
