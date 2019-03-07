package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"terraform-resource/logger"
	"terraform-resource/models"
)

type Action struct {
	Client    Client
	Model     models.Terraform
	Logger    logger.Logger
	EnvName   string
	SourceDir string
}

type Result struct {
	Version models.Version
	Output  map[string]map[string]interface{}
}

func (r Result) RawOutput() map[string]interface{} {
	outputs := map[string]interface{}{}
	for key, value := range r.Output {
		outputs[key] = value["value"]
	}

	return outputs
}

func (r Result) SanitizedOutput() map[string]string {
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

func LinkToThirdPartyPluginDir(sourceDir string) error {
	possiblePluginDir := filepath.Join(sourceDir, "terraform.d")
	if _, err := os.Stat(possiblePluginDir); err == nil {
		err = os.Symlink(possiblePluginDir, "terraform.d")
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Action) Apply() (Result, error) {
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

func (a *Action) attemptApply() (Result, error) {
	a.Logger.InfoSection("Terraform Apply")
	defer a.Logger.EndSection()

	if a.Model.PlanRun {
		if err := a.Client.GetPlanFromBackend(a.planNameForEnv()); err != nil {
			return Result{}, err
		}
	}

	if err := a.createWorkspaceIfNotExists(); err != nil {
		return Result{}, err
	}

	if err := a.Client.Import(a.EnvName); err != nil {
		return Result{}, err
	}

	if err := a.Client.Apply(); err != nil {
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

	if err := a.Client.WorkspaceDeleteWithForce(a.planNameForEnv()); err != nil {
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

func (a *Action) Destroy() (Result, error) {
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

func (a *Action) attemptDestroy() (Result, error) {
	a.Logger.WarnSection("Terraform Destroy")
	defer a.Logger.EndSection()

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

	return Result{
		Output: map[string]map[string]interface{}{},
		Version: models.Version{
			EnvName: a.EnvName,
		},
	}, nil
}

func (a *Action) Plan() (Result, error) {
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

func (a *Action) attemptPlan() (Result, error) {
	a.Logger.InfoSection("Terraform Plan")
	defer a.Logger.EndSection()

	if err := a.createWorkspaceIfNotExists(); err != nil {
		return Result{}, err
	}

	if err := a.Client.Plan(); err != nil {
		return Result{}, err
	}

	if err := a.Client.SavePlanToBackend(a.planNameForEnv()); err != nil {
		return Result{}, err
	}

	return Result{
		Output: map[string]map[string]interface{}{},
		Version: models.Version{
			EnvName: a.EnvName,
		},
	}, nil
}

func (a *Action) setup() error {
	if err := LinkToThirdPartyPluginDir(a.SourceDir); err != nil {
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

func (a *Action) createWorkspaceIfNotExists() error {
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

	if workspaceExists {
		return a.Client.WorkspaceSelect(a.EnvName)
	}
	return a.Client.WorkspaceNew(a.EnvName)
}

func copyOverrideFilesIntoSource(overrideFiles []string, sourceDir string) error {
	for _, overridePath := range overrideFiles {
		if fileInfo, err := os.Stat(overridePath); os.IsNotExist(err) {
			return fmt.Errorf("override file '%s' does not exist", overridePath)
		} else if err != nil {
			return err
		} else if fileInfo.IsDir() {
			return fmt.Errorf("override '%s' is as directory, must pass files instead", overridePath)
		}
		absOverridePath, err := filepath.Abs(overridePath)
		if err != nil {
			return err
		}
		err = os.Symlink(absOverridePath, path.Join(sourceDir, path.Base(absOverridePath)))
		if err != nil {
			return err
		}
	}

	return nil
}

func copyOverrideFilesIntoSourceDir(ModuleOverrideFiles []map[string]string) error {
	for i, overrideMap := range ModuleOverrideFiles {

		overrideSrcPath, ok := overrideMap["src"]
		if !ok {
			return fmt.Errorf("override map '%d' does not include src key", i)
		} else {
			if fileInfo, err := os.Stat(overrideSrcPath); os.IsNotExist(err) {
				return fmt.Errorf("override source file '%s' does not exist", overrideSrcPath)
			} else if err != nil {
				return err
			} else if fileInfo.IsDir() {
				return fmt.Errorf("override source '%s' is as directory, must pass files instead", overrideSrcPath)
			}
		}

		overrideDstPath, ok := overrideMap["dst"]
		if !ok {
			return fmt.Errorf("override map '%d' does not include dst key", i)
		} else {
			if fileInfo, err := os.Stat(overrideDstPath); os.IsNotExist(err) {
				return fmt.Errorf("override destination directory '%s' does not exist", overrideDstPath)
			} else if err != nil {
				return err
			} else if !fileInfo.IsDir() {
				return fmt.Errorf("override destination '%s' is a file, must pass directory instead", overrideDstPath)
			}
		}

		absOverrideSrcPath, err := filepath.Abs(overrideSrcPath)
		if err != nil {
			return err
		}
		absOverrideDstPath, err := filepath.Abs(overrideDstPath)
		if err != nil {
			return err
		}

		err = os.Symlink(absOverrideSrcPath, path.Join(absOverrideDstPath, path.Base(absOverrideSrcPath)))
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Action) planNameForEnv() string {
	return fmt.Sprintf("%s-plan", a.EnvName)
}
