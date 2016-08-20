package models

type Metadata []MetadataField

type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
