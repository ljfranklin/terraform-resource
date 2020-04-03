package terraform

import (
	"fmt"
	"strconv"
	"terraform-resource/logger"
	"terraform-resource/models"
	"terraform-resource/storage"
)

type MigratedFromStorageAction struct {
	Client    Client
	Model     models.Terraform
	Logger    logger.Logger
	EnvName   string
	StateFile storage.StateFile
}

func (a *MigratedFromStorageAction) Apply() (Result, error) {
	err := a.setup()
	if err != nil {
		return Result{}, err
	}

	result, err := a.attemptApply()
	if err != nil {
		a.Logger.Error("Failed To Run Terraform Apply!")
		err = fmt.Errorf("Apply Error: %s", err)
	}

	if err != nil && a.Model.DeleteOnFailure {
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

func (a *MigratedFromStorageAction) attemptApply() (Result, error) {
	a.Logger.InfoSection("Terraform Apply")
	defer a.Logger.EndSection()

	legacyStateFileExists, err := a.StateFile.Exists()
	if err != nil {
		return Result{}, err
	}

	if legacyStateFileExists == false {
		legacyStateFileExists, err = a.StateFile.ExistsAsTainted()
		if err != nil {
			return Result{}, err
		}
		if legacyStateFileExists {
			a.StateFile = a.StateFile.ConvertToTainted()
		}
	}

	if legacyStateFileExists {
		_, err = a.StateFile.Download()
		if err != nil {
			return Result{}, err
		}

		if err = a.importExistingStateFileIntoNewWorkspace(); err != nil {
			return Result{}, err
		}
	} else {
		if a.Model.PlanRun {
			if err := a.Client.GetPlanFromBackend(a.planNameForEnv()); err != nil {
				return Result{}, err
			}
		}

		if err = a.Client.WorkspaceNewIfNotExists(a.EnvName); err != nil {
			return Result{}, err
		}
	}

	// make sure that legacy state file is deleted immediately after new workspace is created
	if legacyStateFileExists {
		migratedStateFile := a.StateFile.ConvertToMigrated()
		if _, err = migratedStateFile.Upload(); err != nil {
			return Result{}, err
		}
		if _, err = a.StateFile.Delete(); err != nil {
			return Result{}, err
		}
	}

	if err = a.Client.Import(a.EnvName); err != nil {
		return Result{}, err
	}

	if err = a.Client.Apply(); err != nil {
		return Result{}, err
	}

	stateVersion, err := a.Client.CurrentStateVersion(a.EnvName)
	if err != nil {
		return Result{}, err
	}
	clientOutput, err := a.Client.Output(a.EnvName)
	if err != nil {
		return Result{}, err
	}

	if err := a.deletePlanWorkspaceIfExists(); err != nil {
		return Result{}, err
	}

	return Result{
		Output: clientOutput,
		Version: models.Version{
			EnvName: a.EnvName,
			Serial:  strconv.Itoa(stateVersion.Serial),
			Lineage: stateVersion.Lineage,
		},
	}, nil
}

func (a *MigratedFromStorageAction) Destroy() (Result, error) {
	err := a.setup()
	if err != nil {
		return Result{}, err
	}

	result, err := a.attemptDestroy()
	if err == nil {
		a.Logger.Success("Successfully Ran Terraform Destroy!")
	}

	return result, err
}

func (a *MigratedFromStorageAction) attemptDestroy() (Result, error) {
	a.Logger.WarnSection("Terraform Destroy")
	defer a.Logger.EndSection()

	legacyStateFileExists, err := a.StateFile.Exists()
	if err != nil {
		return Result{}, err
	}

	if legacyStateFileExists == false {
		legacyStateFileExists, err = a.StateFile.ExistsAsTainted()
		if err != nil {
			return Result{}, err
		}
		if legacyStateFileExists {
			a.StateFile = a.StateFile.ConvertToTainted()
		}
	}

	if legacyStateFileExists {
		_, err = a.StateFile.Download()
		if err != nil {
			return Result{}, err
		}

		if err = a.importExistingStateFileIntoNewWorkspace(); err != nil {
			return Result{}, err
		}
	}

	if legacyStateFileExists {
		_, err = a.StateFile.Delete()
		if err != nil {
			return Result{}, err
		}
	}

	if err := a.Client.WorkspaceSelect(a.EnvName); err != nil {
		return Result{}, err
	}

	if err := a.Client.Import(a.EnvName); err != nil {
		return Result{}, err
	}

	if err := a.Client.Destroy(); err != nil {
		return Result{}, err
	}

	if err := a.Client.WorkspaceDelete(a.EnvName); err != nil {
		return Result{}, err
	}

	if err := a.deletePlanWorkspaceIfExists(); err != nil {
		return Result{}, err
	}

	return Result{
		Output: map[string]map[string]interface{}{},
		Version: models.Version{
			EnvName: a.EnvName,
		},
	}, nil
}

func (a *MigratedFromStorageAction) Plan() (Result, error) {
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

func (a *MigratedFromStorageAction) attemptPlan() (Result, error) {
	a.Logger.InfoSection("Terraform Plan")
	defer a.Logger.EndSection()

	legacyStateFileExists, err := a.StateFile.Exists()
	if err != nil {
		return Result{}, err
	}

	if legacyStateFileExists == false {
		legacyStateFileExists, err = a.StateFile.ExistsAsTainted()
		if err != nil {
			return Result{}, err
		}
		if legacyStateFileExists {
			a.StateFile = a.StateFile.ConvertToTainted()
		}
	}

	if legacyStateFileExists {
		_, err = a.StateFile.Download()
		if err != nil {
			return Result{}, err
		}

		if err = a.importExistingStateFileIntoNewWorkspace(); err != nil {
			return Result{}, err
		}

		// make sure that legacy state file is deleted immediately after new workspace is created
		migratedStateFile := a.StateFile.ConvertToMigrated()
		if _, err = migratedStateFile.Upload(); err != nil {
			return Result{}, err
		}
		if _, err = a.StateFile.Delete(); err != nil {
			return Result{}, err
		}
	} else {
		if err = a.Client.WorkspaceNewIfNotExists(a.EnvName); err != nil {
			return Result{}, err
		}
	}

	planChecksum, err := a.Client.Plan()
	if err != nil {
		return Result{}, err
	}

	if err := a.Client.SavePlanToBackend(a.planNameForEnv()); err != nil {
		return Result{}, err
	}

	return Result{
		Output: map[string]map[string]interface{}{},
		Version: models.Version{
			EnvName: a.EnvName,
			PlanChecksum: planChecksum,
		},
	}, nil
}

func (a *MigratedFromStorageAction) setup() error {
	if err := LinkToThirdPartyPluginDir(a.Model.Source); err != nil {
		return err
	}

	if err := copyOverrideFilesIntoSource(a.Model.OverrideFiles, a.Model.Source); err != nil {
		return err
	}

	if err := copyOverrideFilesIntoSourceDir(a.Model.ModuleOverrideFiles); err != nil {
		return err
	}

	if err := a.Client.InitWithBackend(); err != nil {
		return err
	}

	return nil
}

func (a *MigratedFromStorageAction) importExistingStateFileIntoNewWorkspace() error {
	return a.Client.WorkspaceNewFromExistingStateFile(a.EnvName, a.StateFile.LocalPath)
}

func (a *MigratedFromStorageAction) deletePlanWorkspaceIfExists() error {
	workspaces, err := a.Client.WorkspaceList()

	if err != nil {
		return err
	}

	workspaceExists := false
	for _, space := range workspaces {
		if space == a.planNameForEnv() {
			workspaceExists = true
		}
	}

	if workspaceExists {
		return a.Client.WorkspaceDeleteWithForce(a.planNameForEnv())
	}
	return nil
}

func (a *MigratedFromStorageAction) planNameForEnv() string {
	return fmt.Sprintf("%s-plan", a.EnvName)
}
