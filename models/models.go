package models

type Version struct {
	Version string `json:"version"`
}

type InRequest struct {
	Source  Source  `json:"source"`
	Version Version `json:"version,omitempty"` // absent on initial request
}

type InResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata"`
}

type OutRequest struct {
	Source Source `json:"source"`
	Params Params `json:"params"`
}

type OutResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata"`
}

type Metadata []MetadataField

type MetadataField struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

type Source struct {
	StorageDriver string `json:"storage_driver"`

	// S3 driver
	Bucket          string `json:"bucket"`
	Key             string `json:"key"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	RegionName      string `json:"region_name,omitempty"` // optional
}

type Params map[string]interface{}

const (
	S3Driver = "s3"
)
