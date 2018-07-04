package models_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"terraform-resource/models"

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
			}

			err := model.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error if terraform fields are missing", func() {
			requiredFields := []string{
				"state_file_local_path",
				"state_file_remote_path",
			}

			model := models.Terraform{}

			err := model.Validate()
			Expect(err).To(HaveOccurred())

			for _, field := range requiredFields {
				Expect(err.Error()).To(ContainSubstring(field))
			}
		})
	})

	Describe("Vars", func() {

		It("returns fields from VarFiles", func() {
			varFile := path.Join(tmpDir, "var_file")

			fileVars := map[string]interface{}{
				"fake-key": "fake-value",
			}
			fileContents, err := json.Marshal(fileVars)
			Expect(err).ToNot(HaveOccurred())

			err = ioutil.WriteFile(varFile, fileContents, 0600)
			Expect(err).ToNot(HaveOccurred())

			model := models.Terraform{
				VarFiles: []string{varFile},
			}

			err = model.ParseVarsFromFiles()
			Expect(err).ToNot(HaveOccurred())

			Expect(model.Vars).To(Equal(fileVars))
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
				Imports:             map[string]string{"fake-key": "fake-value"},
				PluginDir:           "fake-plugin-path",
			}

			finalModel := baseModel.Merge(mergeModel)
			Expect(finalModel.Source).To(Equal("base-source"))
			Expect(finalModel.StateFileLocalPath).To(Equal("fake-local-path"))
			Expect(finalModel.StateFileRemotePath).To(Equal("fake-remote-path"))
			Expect(finalModel.DeleteOnFailure).To(BeTrue())
			Expect(finalModel.ImportFiles).To(Equal([]string{"fake-imports-path"}))
			Expect(finalModel.OverrideFiles).To(Equal([]string{"fake-override-path"}))
			Expect(finalModel.Imports).To(Equal(map[string]string{"fake-key": "fake-value"}))
			Expect(finalModel.PluginDir).To(Equal("fake-plugin-path"))
		})

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

		It("returns original vars and vars from VarFiles", func() {
			varFile := path.Join(tmpDir, "var_file")

			fileVars := map[string]interface{}{
				"merge-key":    "merge-value",
				"override-key": "merge-override",
			}
			fileContents, err := json.Marshal(fileVars)
			Expect(err).ToNot(HaveOccurred())

			err = ioutil.WriteFile(varFile, fileContents, 0600)
			Expect(err).ToNot(HaveOccurred())

			model := models.Terraform{
				Source:   "base-source",
				VarFiles: []string{varFile},
				Vars: map[string]interface{}{
					"base-key":     "base-value",
					"override-key": "base-override",
				},
			}

			err = model.ParseVarsFromFiles()
			Expect(err).ToNot(HaveOccurred())

			Expect(model.Vars).To(Equal(map[string]interface{}{
				"base-key":     "base-value",
				"merge-key":    "merge-value",
				"override-key": "merge-override",
			}))
		})

		It("reads vars from tfvars file in HCL format", func() {
			varFile := path.Join(tmpDir, "vars.tfvars")

			fileContents := []byte(`
some_map = {
	some_key = "some_value"
}`)

			err := ioutil.WriteFile(varFile, fileContents, 0600)
			Expect(err).ToNot(HaveOccurred())

			model := models.Terraform{
				VarFiles: []string{varFile},
			}

			err = model.ParseVarsFromFiles()
			Expect(err).ToNot(HaveOccurred())

			Expect(model.Vars).To(Equal(map[string]interface{}{
				"some_map": map[string]interface{}{
					"some_key": "some_value",
				},
			}))
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
