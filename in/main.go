package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/ljfranklin/terraform-resource/models"
)

func main() {

	req := models.InRequest{}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("Failed to read InRequest: %s", err)
	}

	resp := models.InResponse{
		Version:  req.Version,
		Metadata: []models.MetadataField{},
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write InResponse: %s", err)
	}
}
