package models

import (
	"errors"
	"fmt"

	"github.com/ljfranklin/terraform-resource/storage"
)

type InRequest struct {
	Source  Source  `json:"source"`
	Version Version `json:"version,omitempty"` // absent on initial request
	Params  Params  `json:"params,omitempty"`  // used to specify 'destroy' action
}

type InResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata"`
}

type Version struct {
	Version string `json:"version"`
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

func (r InRequest) Validate() error {
	errMsg := ""
	if err := r.Source.Storage.Validate(); err != nil {
		errMsg += fmt.Sprintf("%s\n", err)
	}

	if len(errMsg) > 0 {
		return errors.New(errMsg)
	}
	return nil
}
