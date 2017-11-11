package terraform

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"terraform-resource/logger"
	"terraform-resource/models"
	"terraform-resource/storage"
)

type MigratedFromStorageAction struct {
	Client          Client
	Logger          logger.Logger
	EnvName         string
	DeleteOnFailure bool
	StateFile       storage.StateFile
}

type MigratedFromStorageResult struct {
	Version models.Version
	Output  map[string]map[string]interface{}
}

func (r MigratedFromStorageResult) RawOutput() map[string]interface{} {
	outputs := map[string]interface{}{}
	for key, value := range r.Output {
		outputs[key] = value["value"]
	}

	return outputs
}

func (r MigratedFromStorageResult) SanitizedOutput() map[string]string {
	output := map[string]string{}
	for key, value := range r.Output {
		if value["sensitive"] == true {
			output[key] = "<sensitive>"
		} else {
			jsonValue, err := json.Marshal(value["value"])
			if err != nil {
				jsonValue = []byte(fmt.Sprintf("Unable to parse output value for key '%s': %s", key, err))
			}

			output[key] = strings.Trim(string(jsonValue), "\"")
		}
	}
	return output
}

func (a *MigratedFromStorageAction) Apply() (MigratedFromStorageResult, error) {
	err := a.setup()
	if err != nil {
		return MigratedFromStorageResult{}, err
	}

	result, err := a.attemptApply()
	if err != nil {
		a.Logger.Error("Failed To Run Terraform Apply!")
		err = fmt.Errorf("Apply Error: %s", err)
	}

	if err != nil && a.DeleteOnFailure {
		a.Logger.Warn("Cleaning Up Partially Created Resources...")

		_, destroyErr := a.attemptDestroy()
		if destroyErr != nil {
			a.Logger.Error("Failed To Run Terraform Destroy!")
			err = fmt.Errorf("%s\nDestroy Error: %s", err, destroyErr)
		}
	}

	if err == nil {
		a.Logger.Success("Successfully Ran Terraform Apply!")
	}

	return result, err
}

func (a *MigratedFromStorageAction) attemptApply() (MigratedFromStorageResult, error) {
	a.Logger.InfoSection("Terraform Apply")
	defer a.Logger.EndSection()

	legacyStateFileExists, err := a.StateFile.Exists()
	if err != nil {
		return MigratedFromStorageResult{}, err
	}

	if legacyStateFileExists == false {
		legacyStateFileExists, err = a.StateFile.ExistsAsTainted()
		if err != nil {
			return MigratedFromStorageResult{}, err
		}
		if legacyStateFileExists {
			a.StateFile = a.StateFile.ConvertToTainted()
		}
	}

	if legacyStateFileExists {
		_, err = a.StateFile.Download()
		if err != nil {
			return MigratedFromStorageResult{}, err
		}

		if err = a.importExistingStateFileIntoNewWorkspace(); err != nil {
			return MigratedFromStorageResult{}, err
		}
	} else {
		if err := a.createWorkspaceIfNotExists(); err != nil {
			return MigratedFromStorageResult{}, err
		}
	}

	if legacyStateFileExists {
		migratedStateFile := a.StateFile.ConvertToMigrated()
		if _, err := migratedStateFile.Upload(); err != nil {
			return MigratedFromStorageResult{}, err
		}
		if _, err := a.StateFile.Delete(); err != nil {
			return MigratedFromStorageResult{}, err
		}
	}

	if err := a.Client.Import(a.EnvName); err != nil {
		return MigratedFromStorageResult{}, err
	}

	if err := a.Client.Apply(); err != nil {
		return MigratedFromStorageResult{}, err
	}

	serial, err := a.currentSerial()
	if err != nil {
		return MigratedFromStorageResult{}, err
	}
	clientOutput, err := a.Client.Output(a.EnvName)
	if err != nil {
		return MigratedFromStorageResult{}, err
	}

	return MigratedFromStorageResult{
		Output: clientOutput,
		Version: models.Version{
			EnvName: a.EnvName,
			Serial:  serial,
		},
	}, nil
}

func (a *MigratedFromStorageAction) Destroy() (MigratedFromStorageResult, error) {
	err := a.setup()
	if err != nil {
		return MigratedFromStorageResult{}, err
	}

	result, err := a.attemptDestroy()
	if err == nil {
		a.Logger.Success("Successfully Ran Terraform Destroy!")
	}

	return result, err
}

func (a *MigratedFromStorageAction) attemptDestroy() (MigratedFromStorageResult, error) {
	a.Logger.WarnSection("Terraform Destroy")
	defer a.Logger.EndSection()

	legacyStateFileExists, err := a.StateFile.Exists()
	if err != nil {
		return MigratedFromStorageResult{}, err
	}

	if legacyStateFileExists == false {
		legacyStateFileExists, err = a.StateFile.ExistsAsTainted()
		if err != nil {
			return MigratedFromStorageResult{}, err
		}
		if legacyStateFileExists {
			a.StateFile = a.StateFile.ConvertToTainted()
		}
	}

	if legacyStateFileExists {
		_, err = a.StateFile.Download()
		if err != nil {
			return MigratedFromStorageResult{}, err
		}

		if err = a.importExistingStateFileIntoNewWorkspace(); err != nil {
			return MigratedFromStorageResult{}, err
		}
	}

	if legacyStateFileExists {
		_, err = a.StateFile.Delete()
		if err != nil {
			return MigratedFromStorageResult{}, err
		}
	}

	if err := a.Client.Import(a.EnvName); err != nil {
		return MigratedFromStorageResult{}, err
	}

	if err := a.Client.Destroy(); err != nil {
		return MigratedFromStorageResult{}, err
	}

	if err := a.Client.WorkspaceDelete(a.EnvName); err != nil {
		return MigratedFromStorageResult{}, err
	}

	return MigratedFromStorageResult{
		Output: map[string]map[string]interface{}{},
		Version: models.Version{
			EnvName: a.EnvName,
		},
	}, nil
}

func (a *MigratedFromStorageAction) setup() error {
	if err := a.Client.InitWithBackend(); err != nil {
		return err
	}

	return nil
}

func (a *MigratedFromStorageAction) currentSerial() (string, error) {
	rawState, err := a.Client.StatePull(a.EnvName)
	if err != nil {
		return "", err
	}

	// TODO: read this into a struct
	tfState := map[string]interface{}{}
	if err = json.Unmarshal(rawState, &tfState); err != nil {
		return "", fmt.Errorf("Failed to unmarshal JSON output.\nError: %s\nOutput: %s", err, rawState)
	}

	serial, ok := tfState["serial"].(float64)
	if !ok {
		return "", fmt.Errorf("Expected number value for 'serial' but got '%#v'", tfState["serial"])
	}

	return strconv.Itoa(int(serial)), nil
}

func (a *MigratedFromStorageAction) importExistingStateFileIntoNewWorkspace() error {
	return a.Client.WorkspaceNewFromExistingStateFile(a.EnvName, a.StateFile.LocalPath)
}

func (a *MigratedFromStorageAction) createWorkspaceIfNotExists() error {
	workspaces, err := a.Client.WorkspaceList()
	if err != nil {
		return err
	}

	workspaceExists := false
	for _, space := range workspaces {
		if space == a.EnvName {
			workspaceExists = true
		}
	}

	if !workspaceExists {
		return a.Client.WorkspaceNew(a.EnvName)
	}

	return nil
}
