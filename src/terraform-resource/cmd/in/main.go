package main

import (
	"encoding/json"
	"log"
	"os"

	"terraform-resource/encoder"
	"terraform-resource/in"
	"terraform-resource/models"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Expected output path as first arg")
	}
	outputDir := os.Args[1]

	req := models.InRequest{}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("Failed to read InRequest: %s", err)
	}

	runner := in.Runner{
		OutputDir: outputDir,
	}
	resp, err := runner.Run(req)
	if err != nil {
		log.Fatal(err)
	}

	if err := encoder.NewJSONEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write InResponse: %s", err)
	}
}
