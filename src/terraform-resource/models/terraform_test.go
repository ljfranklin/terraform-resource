package models_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"github.com/ljfranklin/terraform-resource/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Terraform Models", func() {

	var (
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "terraform-resource-test")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Describe("#Validate", func() {

		It("returns nil if all fields are provided", func() {
			model := models.Terraform{
				Source:              "fake-source",
				StateFileLocalPath:  "fake-local-path",
				StateFileRemotePath: "fake-remote-path",
				Vars: map[string]interface{}{
					"fake-key": "fake-value",
				},
				BackendType: "fake-type",
				BackendConfig: map[string]interface{}{
					"fake-backend-key": "fake-backend-value",
				},
			}

			err := model.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("merges non-var fields", func() {
			baseModel := models.Terraform{
				Source: "base-source",
			}
			mergeModel := models.Terraform{
				StateFileLocalPath:  "fake-local-path",
				StateFileRemotePath: "fake-remote-path",
				DeleteOnFailure:     true,
				ImportFiles:         []string{"fake-imports-path"},
				OverrideFiles:       []string{"fake-override-path"},
				ModuleOverrideFiles: []map[string]string{map[string]string{"src": "fake-override-src-path", "dst": "fake-override-dst-path"}},
				Imports:             map[string]string{"fake-key": "fake-value"},
				PluginDir:           "fake-plugin-path",
				BackendType:         "fake-type",
				BackendConfig:       map[string]interface{}{"fake-backend-key": "fake-backend-value"},
			}

			finalModel := baseModel.Merge(mergeModel)
			Expect(finalModel.Source).To(Equal("base-source"))
			Expect(finalModel.StateFileLocalPath).To(Equal("fake-local-path"))
			Expect(finalModel.StateFileRemotePath).To(Equal("fake-remote-path"))
			Expect(finalModel.DeleteOnFailure).To(BeTrue())
			Expect(finalModel.ImportFiles).To(Equal([]string{"fake-imports-path"}))
			Expect(finalModel.OverrideFiles).To(Equal([]string{"fake-override-path"}))
			Expect(finalModel.ModuleOverrideFiles).To(Equal([]map[string]string{map[string]string{"src": "fake-override-src-path", "dst": "fake-override-dst-path"}}))
			Expect(finalModel.Imports).To(Equal(map[string]string{"fake-key": "fake-value"}))
			Expect(finalModel.PluginDir).To(Equal("fake-plugin-path"))
			Expect(finalModel.BackendType).To(Equal("fake-type"))
			Expect(finalModel.BackendConfig).To(Equal(map[string]interface{}{"fake-backend-key": "fake-backend-value"}))
		})
	})

	Describe("Vars", func() {

		It("returns original vars and vars from Merged model", func() {
			baseModel := models.Terraform{
				Source:   "base-source",
				VarFiles: []string{"base-file"},
				Vars: map[string]interface{}{
					"base-key":     "base-value",
					"override-key": "base-override",
				},
			}
			mergeModel := models.Terraform{
				Vars: map[string]interface{}{
					"merge-key":    "merge-value",
					"override-key": "merge-override",
				},
			}

			finalModel := baseModel.Merge(mergeModel)
			Expect(finalModel.Source).To(Equal("base-source"))
			Expect(finalModel.VarFiles).To(Equal([]string{"base-file"}))

			Expect(finalModel.Vars).To(Equal(map[string]interface{}{
				"base-key":     "base-value",
				"merge-key":    "merge-value",
				"override-key": "merge-override",
			}))
		})

		It("writes Vars/VarFiles to formatted files", func() {
			varFiles := []string{}
			jsonFileContents := `{
  "some_json_key": "some_json_value"
}`
			varFiles = append(varFiles, writeToTempFile(tmpDir, jsonFileContents, ".json"))
			yamlFileContents := `
some_yaml_key: some_yaml_value
`
			varFiles = append(varFiles, writeToTempFile(tmpDir, yamlFileContents, ".yaml"))
			hclFileContents := `
some_hcl_key = "some_hcl_value"
`
			varFiles = append(varFiles, writeToTempFile(tmpDir, hclFileContents, ".tfvars"))

			model := models.Terraform{
				Vars: map[string]interface{}{
					"some_var_key": "some_var_value",
				},
				VarFiles: varFiles,
			}

			err := model.ConvertVarFiles(tmpDir)
			Expect(err).ToNot(HaveOccurred())

			Expect(model.ConvertedVarFiles).To(HaveLen(4))

			varFile0 := readJsonFile(model.ConvertedVarFiles[0])
			Expect(varFile0).To(Equal(map[string]string{
				"some_var_key": "some_var_value",
			}))
			varFile1 := readJsonFile(model.ConvertedVarFiles[1])
			Expect(varFile1).To(Equal(map[string]string{
				"some_json_key": "some_json_value",
			}))
			varFile2 := readJsonFile(model.ConvertedVarFiles[2])
			Expect(varFile2).To(Equal(map[string]string{
				"some_yaml_key": "some_yaml_value",
			}))
			varFile3, err := ioutil.ReadFile(model.ConvertedVarFiles[3])
			Expect(err).ToNot(HaveOccurred())
			Expect(string(varFile3)).To(Equal(hclFileContents))
		})
	})

	Describe("Env", func() {
		It("returns original env and env from Merged model", func() {
			baseModel := models.Terraform{
				Env: map[string]string{
					"base-key":     "base-value",
					"override-key": "base-override",
				},
			}
			mergeModel := models.Terraform{
				Env: map[string]string{
					"merge-key":    "merge-value",
					"override-key": "merge-override",
				},
			}

			finalModel := baseModel.Merge(mergeModel)
			Expect(finalModel.Env).To(Equal(map[string]string{
				"base-key":     "base-value",
				"merge-key":    "merge-value",
				"override-key": "merge-override",
			}))
		})
	})

	Describe("ParseImportsFromFile", func() {
		It("populates Imports from contents of ImportsFile", func() {
			importsFilePath := path.Join(tmpDir, "imports")
			importsFileContents := "key: value"
			err := ioutil.WriteFile(importsFilePath, []byte(importsFileContents), 0700)
			Expect(err).ToNot(HaveOccurred())

			model := models.Terraform{
				ImportFiles: []string{importsFilePath},
			}
			err = model.ParseImportsFromFile()
			Expect(err).ToNot(HaveOccurred())

			Expect(model.Imports).To(Equal(map[string]string{
				"key": "value",
			}))
		})
	})

	Describe("PrivateKey", func() {
		It("returns the key from original", func() {
			baseModel := models.Terraform{
				PrivateKey: "fake-key",
			}
			mergeModel := models.Terraform{}

			finalModel := baseModel.Merge(mergeModel)
			Expect(finalModel.PrivateKey).To(Equal("fake-key"))
		})

		It("returns the key from merged", func() {
			baseModel := models.Terraform{
				PrivateKey: "fake-key",
			}
			mergeModel := models.Terraform{
				PrivateKey: "fake-merged-key",
			}

			finalModel := baseModel.Merge(mergeModel)
			Expect(finalModel.PrivateKey).To(Equal("fake-merged-key"))
		})
	})
})

func writeToTempFile(tmpDir string, contents string, ext string) string {
	f, err := ioutil.TempFile(tmpDir, "*"+ext)
	Expect(err).ToNot(HaveOccurred())
	defer f.Close()
	err = ioutil.WriteFile(f.Name(), []byte(contents), 0600)
	Expect(err).ToNot(HaveOccurred())
	return f.Name()
}

func readJsonFile(varFilePath string) map[string]string {
	varFileContents, err := ioutil.ReadFile(varFilePath)
	Expect(err).ToNot(HaveOccurred())
	var varFile map[string]string
	err = json.Unmarshal(varFileContents, &varFile)
	Expect(err).ToNot(HaveOccurred())
	return varFile
}
