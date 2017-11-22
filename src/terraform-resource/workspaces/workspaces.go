package workspaces

import (
	"terraform-resource/models"
	"terraform-resource/terraform"
)

type Workspaces struct {
	client terraform.Client
}

func New(client terraform.Client) *Workspaces {
	return &Workspaces{
		client: client,
	}
}

func (w Workspaces) LatestVersionForEnv(envName string) (models.Version, error) {
	err := w.client.InitWithBackend()
	if err != nil {
		return models.Version{}, err
	}

	exists, err := w.spaceExists(envName)
	if err != nil {
		return models.Version{}, err
	}
	if !exists {
		return models.Version{}, nil
	}

	serial, err := w.client.CurrentSerial(envName)
	if err != nil {
		return models.Version{}, err
	}

	return models.Version{EnvName: envName, Serial: serial}, nil
}

func (w Workspaces) spaceExists(envName string) (bool, error) {
	spaces, err := w.client.WorkspaceList()
	if err != nil {
		return false, err
	}

	for _, space := range spaces {
		if space == envName {
			return true, nil
		}
	}

	return false, nil
}
