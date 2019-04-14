package out

import (
	"fmt"
	"io/ioutil"
	"strings"
	"terraform-resource/models"
	"terraform-resource/namer"
	"terraform-resource/storage"
	"terraform-resource/terraform"
)

const (
	NameClashRetries = 10
)

type BackendEnvNamer struct {
	Req             models.OutRequest
	TerraformClient terraform.Client
	Namer           namer.Namer
}

func (b BackendEnvNamer) EnvName() (string, error) {
	params := b.Req.Params

	envName := ""
	if len(params.EnvNameFile) > 0 {
		contents, err := ioutil.ReadFile(params.EnvNameFile)
		if err != nil {
			return "", fmt.Errorf("Failed to read `env_name_file`: %s", err)
		}
		envName = string(contents)
	} else if params.GenerateRandomName {
		var err error
		envName, err = b.generateRandomName()
		if err != nil {
			return "", err
		}
	} else if len(params.EnvName) > 0 {
		envName = params.EnvName
	} else if len(b.Req.Source.EnvName) > 0 {
		envName = b.Req.Source.EnvName
	}

	if len(envName) == 0 {
		return "", fmt.Errorf("Must specify `put.params.env_name`, `put.params.env_name_file`, `put.params.generate_random_name`, or `source.env_name`")
	}
	envName = strings.TrimSpace(envName)
	envName = strings.Replace(envName, " ", "-", -1)

	return envName, nil
}

func (b BackendEnvNamer) generateRandomName() (string, error) {
	if err := b.TerraformClient.InitWithBackend(); err != nil {
		return "", err
	}

	existingEnvs, err := b.TerraformClient.WorkspaceList()
	if err != nil {
		return "", err
	}

	var envName string
	for i := 0; i < NameClashRetries; i++ {
		randomName := b.Namer.RandomName()
		clash := false
		for _, e := range existingEnvs {
			if e == randomName {
				clash = true
				break
			}
		}
		if clash == false {
			envName = randomName
			break
		}
	}
	if len(envName) == 0 {
		return "", fmt.Errorf("Failed to generate a non-clashing random name after %d attempts", NameClashRetries)
	}

	return envName, nil
}

type MigratedFromStorageEnvNamer struct {
	Req             models.OutRequest
	TerraformClient terraform.Client
	Namer           namer.Namer
	StorageDriver   storage.Storage
}

func (m MigratedFromStorageEnvNamer) EnvName() (string, error) {
	params := m.Req.Params

	if params.GenerateRandomName {
		return m.generateRandomName()
	}

	backendNamer := BackendEnvNamer{
		Req:             m.Req,
		TerraformClient: m.TerraformClient,
	}
	return backendNamer.EnvName()
}

func (m MigratedFromStorageEnvNamer) generateRandomName() (string, error) {
	if err := m.TerraformClient.InitWithBackend(); err != nil {
		return "", err
	}

	existingEnvs, err := m.TerraformClient.WorkspaceList()
	if err != nil {
		return "", err
	}

	var envName string
	for i := 0; i < NameClashRetries; i++ {
		randomName := m.Namer.RandomName()
		clash := false
		for _, e := range existingEnvs {
			if e == randomName {
				clash = true
				break
			}
		}
		if clash == false {
			clash, err = doesEnvNameClashWithLegacyEnv(randomName, m.StorageDriver)
			if err != nil {
				return "", err
			}
		}
		if clash == false {
			envName = randomName
			break
		}
	}
	if len(envName) == 0 {
		return "", fmt.Errorf("Failed to generate a non-clashing random name after %d attempts", NameClashRetries)
	}

	return envName, nil
}

type LegacyStorageEnvNamer struct {
	Req           models.OutRequest
	StorageDriver storage.Storage
	Namer         namer.Namer
}

func (l LegacyStorageEnvNamer) EnvName() (string, error) {
	envName := ""
	params := l.Req.Params
	if len(params.EnvNameFile) > 0 {
		contents, err := ioutil.ReadFile(params.EnvNameFile)
		if err != nil {
			return "", fmt.Errorf("Failed to read `env_name_file`: %s", err)
		}
		envName = string(contents)
	} else if len(params.EnvName) > 0 {
		envName = params.EnvName
	} else if params.GenerateRandomName {
		var randomName string
		for i := 0; i < NameClashRetries; i++ {
			randomName = l.Namer.RandomName()
			clash, err := doesEnvNameClashWithLegacyEnv(randomName, l.StorageDriver)
			if err != nil {
				return "", err
			}
			if clash == false {
				envName = randomName
				break
			}
		}
		if len(envName) == 0 {
			return "", fmt.Errorf("Failed to generate a non-clashing random name after %d attempts", NameClashRetries)
		}
	} else if len(l.Req.Source.EnvName) > 0 {
		envName = params.EnvName
	}

	if len(envName) == 0 {
		return "", fmt.Errorf("Must specify `put.params.env_name`, `put.params.env_name_file`, `put.params.generate_random_name`, or `source.env_name`")
	}
	envName = strings.TrimSpace(envName)
	envName = strings.Replace(envName, " ", "-", -1)

	return envName, nil
}

func doesEnvNameClashWithLegacyEnv(envName string, storageDriver storage.Storage) (bool, error) {
	filename := fmt.Sprintf("%s.tfstate", envName)
	version, err := storageDriver.Version(filename)
	if err != nil {
		return false, err
	}
	return version.IsZero() == false, nil
}
