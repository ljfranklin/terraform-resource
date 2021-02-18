package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/ljfranklin/terraform-resource/check"
	"github.com/ljfranklin/terraform-resource/encoder"
	"github.com/ljfranklin/terraform-resource/models"
)

func main() {
	req := models.InRequest{}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("Failed to read InRequest: %s", err)
	}

	cmd := check.Runner{
		LogWriter: os.Stderr,
	}
	resp, err := cmd.Run(req)
	if err != nil {
		log.Fatal(err)
	}

	if err := encoder.NewJSONEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("Failed to write Versions to stdout: %s", err)
	}
}
