package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/terraform"
)

const (
	stateFileName = "terraform.tfstate"
)

func main() {

	if len(os.Args) < 2 {
		log.Fatalf("Expected path to sources as first arg")
	}
	sourceDir := os.Args[1]
	if err := os.Chdir(sourceDir); err != nil {
		log.Fatalf("Failed to access source dir '%s': %s", sourceDir, err)
	}

	req := models.OutRequest{}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("Failed to read OutRequest: %s", err)
	}

	terraformSource, ok := req.Params["terraform_source"]
	if !ok {
		log.Fatalf("Must specify 'terraform_source' under put params")
	}
	delete(req.Params, "terraform_source")

	client := terraform.Client{
		Source:    terraformSource.(string),
		StateFile: stateFileName,
	}

	if err := client.Apply(req.Params); err != nil {
		log.Fatalf("Failed to run terraform apply.\nError: %s", err)
	}

	output, err := client.Output()
	if err != nil {
		log.Fatalf("Failed to terraform output.\nError: %s", err)
	}
	metadata := []models.MetadataField{}
	for key, value := range output {
		metadata = append(metadata, models.MetadataField{
			Name:  key,
			Value: value,
		})
	}

	resp := models.OutResponse{
		Version: models.Version{
			Version: "0.0.0",
		},
		Metadata: metadata,
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write OutResponse: %s", err)
	}
}
