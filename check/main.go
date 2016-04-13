package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/ljfranklin/terraform-resource/in/models"
	"github.com/ljfranklin/terraform-resource/storage"
)

func main() {
	req := models.InRequest{}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("Failed to read InRequest: %s", err)
	}

	if req.Source.Storage.Key == "" {
		// checking for new versions only works if `Source.Storage.Key` is specified
		// return empty version list if `key` is specified as a put param instead
		resp := []models.Version{}
		if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
			log.Fatalf("Failed to write Versions to stdout: %s", err)
		}
		return
	}

	if err := req.Validate(); err != nil {
		log.Fatalf("Failed to validate Check Request: %s", err)
	}

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
		log.Fatalf("Unknown storage_driver '%s'. Supported drivers are: %v", driverType, strings.Join(supportedDrivers, ", "))
	}

	version, err := storageDriver.Version(req.Source.Storage.Key)
	if err != nil {
		log.Fatalf("Failed to check storage backend for version: %s", err)
	}

	resp := []models.Version{}
	if version != "" {
		resp = append(resp, models.Version{
			Version: version,
		})
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write Versions to stdout: %s", err)
	}
}
