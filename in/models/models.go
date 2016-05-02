package models

import (
	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/storage"
)

type InRequest struct {
	Source  Source         `json:"source"`
	Version models.Version `json:"version,omitempty"` // absent on initial request
	Params  Params         `json:"params,omitempty"`  // used to specify 'destroy' action
}

type InResponse struct {
	Version  models.Version `json:"version"`
	Metadata Metadata       `json:"metadata"`
}

type Metadata []MetadataField

type MetadataField struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

type Source struct {
	Storage storage.Model `json:"storage"`
}

type Params struct {
	Action string `json:"action,omitempty"` // optional
}

const (
	DestroyAction = "destroy"
)
