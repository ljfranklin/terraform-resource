package terraform

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"terraform-resource/logger"
	"terraform-resource/models"
	"terraform-resource/storage"
)

type LegacyStorageAction struct {
	Client    Client
	Model     models.Terraform
	PlanFile  storage.PlanFile
	StateFile storage.StateFile
	Logger    logger.Logger
}

type LegacyStorageResult struct {
	Version storage.Version
	Output  map[string]map[string]interface{}
}

func (r LegacyStorageResult) RawOutput() map[string]interface{} {
	outputs := map[string]interface{}{}
	for key, value := range r.Output {
		outputs[key] = value["value"]
	}

	return outputs
}

func (r LegacyStorageResult) SanitizedOutput() map[string]string {
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

func (a *LegacyStorageAction) Apply() (LegacyStorageResult, error) {
	err := a.setup()
	if err != nil {
		return LegacyStorageResult{}, err
	}

	result, err := a.attemptApply()
	if err != nil {
		a.Logger.Error("Failed To Run Terraform Apply!")
		err = fmt.Errorf("Apply Error: %s", err)
	}

	alreadyDeleted := false
	if err != nil && a.Model.DeleteOnFailure {
		a.Logger.Warn("Cleaning Up Partially Created Resources...")

		_, destroyErr := a.attemptDestroy()
		if destroyErr != nil {
			a.Logger.Error("Failed To Run Terraform Destroy!")
			err = fmt.Errorf("%s\nDestroy Error: %s", err, destroyErr)
		} else {
			alreadyDeleted = true
		}
	}

	if err != nil && alreadyDeleted == false {
		uploadErr := a.uploadTaintedStatefile()
		if uploadErr != nil {
			err = fmt.Errorf("Destroy Error: %s\nUpload Error: %s", err, uploadErr)
		}
	}

	if err == nil {
		a.Logger.Success("Successfully Ran Terraform Apply!")
	}

	return result, err
}

func (a *LegacyStorageAction) attemptApply() (LegacyStorageResult, error) {
	a.Logger.InfoSection("Terraform Apply")
	defer a.Logger.EndSection()

	if err := a.Client.Apply(); err != nil {
		return LegacyStorageResult{}, err
	}

	if a.StateFile.IsTainted() {
		_, err := a.StateFile.Delete()
		if err != nil {
			return LegacyStorageResult{}, err
		}
		a.StateFile = a.StateFile.ConvertFromTainted()
	}

	storageVersion, err := a.StateFile.Upload()
	if err != nil {
		return LegacyStorageResult{}, err
	}

	// Does a plan exist on the bucket ?
	planExist, err := a.PlanFile.Exists()
	if err != nil {
		return LegacyStorageResult{}, err
	}

	// if yes, then, delete it
	if planExist {
		if _, err = a.PlanFile.Delete(); err != nil {
			return LegacyStorageResult{}, err
		}
	}

	clientOutput, err := a.Client.OutputWithLegacyStorage()
	if err != nil {
		return LegacyStorageResult{}, err
	}
	return LegacyStorageResult{
		Output:  clientOutput,
		Version: storageVersion,
	}, nil
}

func (a *LegacyStorageAction) Destroy() (LegacyStorageResult, error) {
	err := a.setup()
	if err != nil {
		return LegacyStorageResult{}, err
	}

	result, err := a.attemptDestroy()

	if err != nil {
		a.Logger.Error("Failed To Run Terraform Destroy!")
		uploadErr := a.uploadTaintedStatefile()
		if uploadErr != nil {
			err = fmt.Errorf("Destroy Error: %s\nUpload Error: %s", err, uploadErr)
		}
	}

	if err == nil {
		a.Logger.Success("Successfully Ran Terraform Destroy!")
	}

	return result, err
}

func (a *LegacyStorageAction) attemptDestroy() (LegacyStorageResult, error) {
	a.Logger.WarnSection("Terraform Destroy")
	defer a.Logger.EndSection()

	if err := a.Client.Destroy(); err != nil {
		return LegacyStorageResult{}, err
	}

	_, err := a.PlanFile.Delete()
	if err != nil {
		return LegacyStorageResult{}, err
	}
	storageVersion, err := a.StateFile.Delete()
	if err != nil {
		return LegacyStorageResult{}, err
	}
	return LegacyStorageResult{
		Output:  map[string]map[string]interface{}{},
		Version: storageVersion,
	}, nil
}

func (a *LegacyStorageAction) Plan() (LegacyStorageResult, error) {
	err := a.setup()
	if err != nil {
		return LegacyStorageResult{}, err
	}

	result, err := a.attemptPlan()
	if err != nil {
		a.Logger.Error("Failed To Run Terraform Plan!")
		err = fmt.Errorf("Plan Error: %s", err)
	}

	if err == nil {
		a.Logger.Success("Successfully Ran Terraform Plan!")
	}

	return result, err
}

func (a *LegacyStorageAction) attemptPlan() (LegacyStorageResult, error) {
	a.Logger.InfoSection("Terraform Plan")
	defer a.Logger.EndSection()

	if err := a.Client.Plan(); err != nil {
		return LegacyStorageResult{}, err
	}

	storageVersion, err := a.PlanFile.Upload()
	if err != nil {
		return LegacyStorageResult{}, err
	}

	return LegacyStorageResult{
		Output:  map[string]map[string]interface{}{},
		Version: storageVersion,
	}, nil
}

func (a *LegacyStorageAction) setup() error {
	stateFileExists, err := a.StateFile.Exists()
	if err != nil {
		return err
	}

	planFileExists, err := a.PlanFile.Exists()
	if err != nil {
		return err
	}

	if stateFileExists == false {
		stateFileExists, err = a.StateFile.ExistsAsTainted()
		if err != nil {
			return err
		}
		if stateFileExists {
			a.StateFile = a.StateFile.ConvertToTainted()
		}
	}

	if planFileExists {
		_, err = a.PlanFile.Download()
		if err != nil {
			return err
		}
	}

	if stateFileExists {
		_, err = a.StateFile.Download()
		if err != nil {
			return err
		}
	}

	if err := LinkToThirdPartyPluginDir(a.Model.Source); err != nil {
		return err
	}

	if err := copyOverrideFilesIntoSource(a.Model.OverrideFiles, a.Model.Source); err != nil {
		return err
	}

	if err := a.Client.InitWithoutBackend(); err != nil {
		return err
	}

	if err := a.Client.ImportWithLegacyStorage(); err != nil {
		return err
	}

	return nil
}

func (a *LegacyStorageAction) uploadTaintedStatefile() error {
	errMsg := ""
	_, deleteErr := a.StateFile.Delete()
	if deleteErr != nil {
		errMsg = fmt.Sprintf("Delete original state file error: %s", deleteErr)
	}
	a.StateFile = a.StateFile.ConvertToTainted()

	_, uploadErr := a.StateFile.Upload()
	if uploadErr != nil {
		errMsg = fmt.Sprintf("%s\nUpload Error: %s", errMsg, uploadErr)
	}

	if len(errMsg) > 0 {
		return errors.New(errMsg)
	}

	a.Logger.Success(fmt.Sprintf("IMPORTANT - Uploaded State File for manual cleanup to '%s'", a.StateFile.RemotePath))

	return nil
}
