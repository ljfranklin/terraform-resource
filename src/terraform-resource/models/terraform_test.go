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

		It("returns fields from VarFile", func() {
			varFile := path.Join(tmpDir, "var_file")

			fileVars := map[string]interface{}{
				"fake-key": "fake-value",
			}
			fileContents, err := json.Marshal(fileVars)
			Expect(err).ToNot(HaveOccurred())

			err = ioutil.WriteFile(varFile, fileContents, 0600)
			Expect(err).ToNot(HaveOccurred())

			model := models.Terraform{
				VarFile: varFile,
			}

			err = model.ParseVarsFromFile()
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
			}

			finalModel := baseModel.Merge(mergeModel)
			Expect(finalModel.Source).To(Equal("base-source"))
			Expect(finalModel.StateFileLocalPath).To(Equal("fake-local-path"))
			Expect(finalModel.StateFileRemotePath).To(Equal("fake-remote-path"))
			Expect(finalModel.DeleteOnFailure).To(BeTrue())
		})

		It("returns original vars and vars from Merged model", func() {
			baseModel := models.Terraform{
				Source:  "base-source",
				VarFile: "base-file",
				Vars: map[string]interface{}{
					"base-key":     "base-value",
					"override-key": "base-override",
				},
			}
			mergeModel := models.Terraform{
				VarFile: "merge-file",
				Vars: map[string]interface{}{
					"merge-key":    "merge-value",
					"override-key": "merge-override",
				},
			}

			finalModel := baseModel.Merge(mergeModel)
			Expect(finalModel.Source).To(Equal("base-source"))
			Expect(finalModel.VarFile).To(Equal("merge-file"))

			Expect(finalModel.Vars).To(Equal(map[string]interface{}{
				"base-key":     "base-value",
				"merge-key":    "merge-value",
				"override-key": "merge-override",
			}))
		})

		It("returns original vars and vars from VarFile", func() {
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
				Source:  "base-source",
				VarFile: varFile,
				Vars: map[string]interface{}{
					"base-key":     "base-value",
					"override-key": "base-override",
				},
			}

			err = model.ParseVarsFromFile()
			Expect(err).ToNot(HaveOccurred())

			Expect(model.Vars).To(Equal(map[string]interface{}{
				"base-key":     "base-value",
				"merge-key":    "merge-value",
				"override-key": "merge-override",
			}))
		})
	})
})
