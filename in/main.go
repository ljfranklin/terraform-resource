package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/ljfranklin/terraform-resource/in/models"
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
	if err := req.Version.Validate(); err != nil {
		log.Fatalf("Invalid Version request: %s", err)
	}

	if req.Params.Action == models.DestroyAction {
		resp := models.InResponse{
			Version: req.Version,
		}

		if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
			log.Fatalf("Failed to write InResponse: %s", err)
		}
		return
	}

	storageModel := req.Source.Storage
	if err = storageModel.Validate(); err != nil {
		log.Fatalf("Failed to validate storage Model: %s", err)
	}
	storageDriver := storage.BuildDriver(storageModel)

	terraformModel := terraform.Model{
		StateFileLocalPath:  path.Join(tmpDir, "terraform.tfstate"),
		StateFileRemotePath: req.Version.StateFileKey,
	}
	if err := terraformModel.Validate(); err != nil {
		log.Fatalf("Failed to validate terraform Model: %s", err)
	}

	client := terraform.Client{
		Model:         terraformModel,
		StorageDriver: storageDriver,
	}

	version, err := client.DownloadStateFileIfExists()
	if err != nil {
		log.Fatalf("Failed to download state file from storage backend: %s", err)
	}
	if version.IsZero() {
		log.Fatalf("StateFile does not exist with key '%s'", req.Version.StateFileKey)
	}

	output, err := client.Output()
	if err != nil {
		log.Fatalf("Failed to parse terraform output.\nError: %s", err)
	}

	outputFilepath := path.Join(outputDir, "metadata")
	outputFile, err := os.Create(outputFilepath)
	if err != nil {
		log.Fatalf("Failed to create output file at path '%s': %s", outputFilepath, err)
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
		Version:  version,
		Metadata: metadata,
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write InResponse: %s", err)
	}
}
