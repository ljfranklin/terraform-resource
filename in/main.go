package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/terraform"
)

func main() {

	if len(os.Args) < 2 {
		log.Fatalf("Expected output path as first arg")
	}
	outputDir := os.Args[1]
	tmpDir, err := ioutil.TempDir(os.TempDir(), "terraform-resource-in")
	if err != nil {
		log.Fatalf("Failed to create tmp dir at '%s'", os.TempDir())
	}
	defer os.RemoveAll(tmpDir)

	req := models.InRequest{}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("Failed to read InRequest: %s", err)
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

	version, err := storageDriver.Version(req.Source.Key)
	if err != nil {
		log.Fatalf("Failed to check storage backend for version: %s", err)
	}
	if version == "" {
		log.Fatalf("State file does not exist: %s", req.Source.Key)
	}

	stateFilepath := path.Join(tmpDir, path.Base(req.Source.Key))
	stateFile, err := os.Create(stateFilepath)
	if err != nil {
		log.Fatalf("Failed to create temporary state file at path '%s': %s", stateFilepath, err)
	}

	err = storageDriver.Download(req.Source.Key, stateFile)
	stateFile.Close()
	if err != nil {
		log.Fatalf("Failed to download state file from storage backend: %s", err)
	}

	outputFilepath := path.Join(outputDir, path.Base(req.Source.Key))
	outputFile, err := os.Create(outputFilepath)
	if err != nil {
		log.Fatalf("Failed to create output file at path '%s': %s", outputFilepath, err)
	}

	client := terraform.Client{
		StateFile: stateFilepath,
	}
	output, err := client.Output()
	if err != nil {
		log.Fatalf("Failed to parse terraform output.\nError: %s", err)
	}
	if err := json.NewEncoder(outputFile).Encode(output); err != nil {
		log.Fatalf("Failed to write output file: %s", err)
	}

	metadata := []models.MetadataField{}
	for key, value := range output {
		metadata = append(metadata, models.MetadataField{
			Name:  key,
			Value: value,
		})
	}

	resp := models.InResponse{
		Version: models.Version{
			Version: version,
		},
		Metadata: metadata,
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write InResponse: %s", err)
	}
}
