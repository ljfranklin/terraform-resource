package helpers

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

func DownloadStatefulPlugin(workingDir string) error {
	var hostOS string
	if runtime.GOOS == "darwin" {
		hostOS = "darwin"
	} else {
		hostOS = "linux"
	}
	url := fmt.Sprintf("https://github.com/ashald/terraform-provider-stateful/releases/download/v1.2.0/terraform-provider-stateful_v1.2.0-%s-amd64", hostOS)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	pluginDir := filepath.Join(workingDir, ".terraform.d", "plugins", "github.com",
		"ashald", "stateful", "1.2.0", fmt.Sprintf("%s_amd64", hostOS))
	err = os.MkdirAll(pluginDir, os.ModePerm)
	if err != nil {
		return err
	}

	pluginPath := filepath.Join(pluginDir, "terraform-provider-stateful_v1.2.0")
	out, err := os.Create(pluginPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if err = out.Chmod(0755); err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)
	return err
}
