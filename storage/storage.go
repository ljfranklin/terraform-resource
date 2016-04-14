package storage

import (
	"io"
)

type Storage interface {
	Download(string, io.Writer) error
	Upload(string, io.Reader) error
	Delete(string) error
	Version(string) (string, error)
}

func BuildDriver(m Model) Storage {
	driverType := m.Driver
	if driverType == "" {
		driverType = S3Driver
	}

	var storageDriver Storage
	switch driverType {
	case S3Driver:
		storageDriver = NewS3(m)
	default:
		// calling model.Validate will throw error for this case
		return null{}
	}

	return storageDriver
}
