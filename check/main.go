package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/storage"
)

func main() {
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

	resp := []models.Version{
		models.Version{
			Version: version,
		},
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write Versions to stdout: %s", err)
	}
}
