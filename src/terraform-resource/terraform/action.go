package terraform

import (
	"errors"
	"fmt"
	"terraform-resource/logger"
	"terraform-resource/models"
	"terraform-resource/storage"
)

type Action struct {
	Client          Client
	PlanFile        PlanFile
	StateFile       StateFile
	Logger          logger.Logger
	DeleteOnFailure bool
}

type Result struct {
	Version storage.Version
	Output  map[string]interface{}
}

func (a Action) Apply() (Result, error) {
	err := a.setup()
	if err != nil {
		return Result{}, err
	}

	result, err := a.attemptApply()
	if err != nil {
		a.Logger.Error("Failed To Run Terraform Apply!")
		err = fmt.Errorf("Apply Error: %s", err)
	}

	alreadyDeleted := false
	if err != nil && a.DeleteOnFailure {
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

func (a Action) attemptApply() (Result, error) {
	a.Logger.InfoSection("Terraform Apply")
	defer a.Logger.EndSection()

	if err := a.Client.Apply(); err != nil {
		return Result{}, err
	}

	if a.StateFile.IsTainted() {
		_, err := a.StateFile.Delete()
		if err != nil {
			return Result{}, err
		}
		a.StateFile = a.StateFile.ConvertFromTainted()
	}

	storageVersion, err := a.StateFile.Upload()
	if err != nil {
		return Result{}, err
	}

	// Does a plan exist on the bucket ?
	planExist, err := a.PlanFile.Exists()
	if err != nil {
		return Result{}, err
	}

	// if yes, then, delete it
	if planExist {
		if _, err := a.PlanFile.Delete(); err != nil {
			return Result{}, err
		}
	}

	clientOutput, err := a.Client.Output()
	if err != nil {
		return Result{}, err
	}
	return Result{
		Output:  clientOutput,
		Version: storageVersion,
	}, nil
}

func (a Action) Destroy() (Result, error) {
	err := a.setup()
	if err != nil {
		return Result{}, err
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

func (a Action) attemptDestroy() (Result, error) {
	a.Logger.WarnSection("Terraform Destroy")
	defer a.Logger.EndSection()

	if err := a.Client.Destroy(); err != nil {
		return Result{}, err
	}

	_, err := a.PlanFile.Delete()
	if err != nil {
		return Result{}, err
	}
	storageVersion, err := a.StateFile.Delete()
	if err != nil {
		return Result{}, err
	}
	return Result{
		Output:  map[string]interface{}{},
		Version: storageVersion,
	}, nil
}

func (a Action) Plan() (Result, error) {
	err := a.setup()
	if err != nil {
		return Result{}, err
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

func (a Action) attemptPlan() (Result, error) {
	a.Logger.InfoSection("Terraform Plan")
	defer a.Logger.EndSection()

	if err := a.Client.Plan(); err != nil {
		return Result{}, err
	}

	storageVersion, err := a.PlanFile.Upload()
	if err != nil {
		return Result{}, err
	}

	return Result{
		Output:  map[string]interface{}{},
		Version: storageVersion,
	}, nil
}

func (a *Action) setup() error {
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
		outputs, err := a.Client.Output()
		if err != nil {
			return err
		}
		a.Client.Model = models.Terraform{Vars: outputs}.Merge(a.Client.Model)
	}
	return nil
}

func (a *Action) uploadTaintedStatefile() error {
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
