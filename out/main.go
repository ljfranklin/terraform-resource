package main

import (
	"encoding/json"
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
		log.Fatalf("Must specify 'key' under resource.source")
	}

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
		log.Fatalf("Unknown storage_driver '%s'. Supported drivers are: %v", driverType, strings.Join(supportedDrivers, ", "))
	}

	terraformSource, ok := req.Params["terraform_source"]
	if !ok {
		log.Fatalf("Must specify 'terraform_source' under put params")
	}
	delete(req.Params, "terraform_source")

	stateFilePath := path.Join(tmpDir, "terraform.tfstate")
	client := terraform.Client{
		Source:        terraformSource.(string),
		StateFilePath: stateFilePath,
	}

	version := ""
	metadata := []models.MetadataField{}

	if req.Params["action"] == models.DestroyAction {
		stateFile, createErr := os.Create(stateFilePath)
		if createErr != nil {
			log.Fatalf("Failed to create state file at '%s': %s", stateFilePath, createErr)
		}
		defer stateFile.Close()

		err = storageDriver.Download(storageKey, stateFile)
		if err != nil {
			log.Fatalf("Failed to download state file: %s", err)
		}
		stateFile.Close()

		if err = client.Destroy(req.Params); err != nil {
			log.Fatalf("Failed to run terraform destroy.\nError: %s", err)
		}

		err = storageDriver.Delete(storageKey)
		if err != nil {
			log.Fatalf("Failed to delete state file: %s", err)
		}

		version = time.Now().UTC().Format(time.RFC3339)
	} else {
		if err = client.Apply(req.Params); err != nil {
			log.Fatalf("Failed to run terraform apply.\nError: %s", err)
		}
		stateFile, err := os.Open(stateFilePath)
		if err != nil {
			log.Fatalf("Failed to open state file at '%s'", stateFilePath)
		}
		defer stateFile.Close()

		err = storageDriver.Upload(storageKey, stateFile)
		if err != nil {
			log.Fatalf("Failed to upload state file: %s", err)
		}

		version, err = storageDriver.Version(storageKey)
		if err != nil {
			log.Fatalf("Failed to retrieve version from '%s': %s", storageKey, err)
		}
		if version == "" {
			log.Fatalf("Couldn't find state file at: %s", storageKey)
		}

		output, err := client.Output()
		if err != nil {
			log.Fatalf("Failed to terraform output.\nError: %s", err)
		}
		for key, value := range output {
			metadata = append(metadata, models.MetadataField{
				Name:  key,
				Value: value,
			})
		}
	}

	resp := models.OutResponse{
		Version: models.Version{
			Version: version,
		},
		Metadata: metadata,
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write OutResponse: %s", err)
	}
}
