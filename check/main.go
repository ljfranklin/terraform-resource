package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/ljfranklin/terraform-resource/in/models"
	"github.com/ljfranklin/terraform-resource/storage"
)

func main() {
	req := models.InRequest{}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("Failed to read InRequest: %s", err)
	}

	currentVersionTime := time.Time{}
	if req.Version.IsZero() == false {
		if err := req.Version.Validate(); err != nil {
			log.Fatalf("Failed to validate provided version: %s", err)
		}
		currentVersionTime = req.Version.LastModifiedTime()
	}

	storageModel := req.Source.Storage
	if err := storageModel.Validate(); err != nil {
		log.Fatalf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	version, err := storageDriver.LatestVersion()
	if err != nil {
		log.Fatalf("Failed to check storage backend for latest version: %s", err)
	}

	resp := []storage.Version{}
	if version.IsZero() == false && version.LastModifiedTime().After(currentVersionTime) {
		resp = append(resp, version)
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write Versions to stdout: %s", err)
	}
}
