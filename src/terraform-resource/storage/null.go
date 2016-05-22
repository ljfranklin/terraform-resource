package storage

import (
	"errors"
	"io"
)

type null struct{}

func (n null) Download(key string, destination io.Writer) (Version, error) {
	return Version{}, errors.New("Not Implemented")
}

func (n null) Upload(key string, content io.Reader) (Version, error) {
	return Version{}, errors.New("Not Implemented")
}

func (n null) Delete(key string) error {
	return errors.New("Not Implemented")
}

func (n null) Version(key string) (Version, error) {
	return Version{}, errors.New("Not Implemented")
}

func (n null) LatestVersion(filterRegex string) (Version, error) {
	return Version{}, errors.New("Not Implemented")
}
