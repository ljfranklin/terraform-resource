package main

import (
	"encoding/json"
	"log"
	"os"

	"terraform-resource/check"
	"terraform-resource/encoder"
	"terraform-resource/models"
)

func main() {
	req := models.InRequest{}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("Failed to read InRequest: %s", err)
	}

	cmd := check.Runner{}
	resp, err := cmd.Run(req)
	if err != nil {
		log.Fatal(err)
	}

	if err := encoder.NewJSONEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write Versions to stdout: %s", err)
	}
}
