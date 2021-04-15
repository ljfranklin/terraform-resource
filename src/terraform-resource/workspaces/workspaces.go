package workspaces

import (
	"github.com/ljfranklin/terraform-resource/terraform"
)

type Workspaces struct {
	client terraform.Client
}

func New(client terraform.Client) *Workspaces {
	return &Workspaces{
		client: client,
	}
}

func (w Workspaces) LatestVersionForEnv(envName string) (terraform.StateVersion, error) {
	err := w.client.InitWithBackend()
	if err != nil {
		return terraform.StateVersion{}, err
	}

	exists, err := w.spaceExists(envName)
	if err != nil {
		return terraform.StateVersion{}, err
	}
	if !exists {
		return terraform.StateVersion{}, nil
	}

	return w.client.CurrentStateVersion(envName)
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
