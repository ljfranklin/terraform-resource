package workspaces

import (
	"encoding/json"
	"fmt"
	"strconv"
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

	rawState, err := w.client.StatePull(envName)
	if err != nil {
		return models.Version{}, err
	}

	// TODO: read this into a struct
	tfState := map[string]interface{}{}
	if err = json.Unmarshal(rawState, &tfState); err != nil {
		return models.Version{}, fmt.Errorf("Failed to unmarshal JSON output.\nError: %s\nOutput: %s", err, rawState)
	}

	serial, ok := tfState["serial"].(float64)
	if !ok {
		return models.Version{}, fmt.Errorf("Expected number value for 'serial' but got '%#v'", tfState["serial"])
	}

	return models.Version{EnvName: envName, Serial: strconv.Itoa(int(serial))}, nil
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
