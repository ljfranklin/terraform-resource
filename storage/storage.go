package storage

import "io"

type Storage interface {
	Download(string) (io.ReadCloser, error)
	Upload(string, io.Reader) error
	Delete(string) error
	Version(string) (string, error)
}
