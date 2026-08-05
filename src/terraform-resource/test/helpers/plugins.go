package helpers

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

func DownloadPlugins(pluginPath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	zipFile, err := ioutil.TempFile("", "terraform-resource-out-test")
	if err != nil {
		return err
	}
	defer zipFile.Close()

	if _, err := io.Copy(zipFile, resp.Body); err != nil {
		return err
	}

	zipReader, err := zip.OpenReader(zipFile.Name())
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, sourceFile := range zipReader.File {
		path := filepath.Join(pluginPath, sourceFile.Name)

		reader, err := sourceFile.Open()
		if err != nil {
			return err
		}
		defer reader.Close()

		writer, err := os.Create(path)
		if err != nil {
			return err
		}
		defer writer.Close()

		if _, err := io.Copy(writer, reader); err != nil {
			return err
		}

		if err := os.Chmod(path, 0700); err != nil {
			return err
		}
	}

	return nil
}
