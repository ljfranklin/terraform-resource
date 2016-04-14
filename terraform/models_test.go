package terraform_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"github.com/ljfranklin/terraform-resource/terraform"
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
			model := terraform.Model{
				Source: "fake-source",
				Vars: map[string]interface{}{
					"fake-key": "fake-value",
				},
			}

			err := model.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error if terraform.source is missing", func() {
			model := terraform.Model{}

			err := model.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("source"))
		})
	})

	Describe("#BuildVars", func() {

		It("returns fields from VarFile", func() {
			varFile := path.Join(tmpDir, "var_file")

			fileVars := map[string]interface{}{
				"fake-key": "fake-value",
			}
			fileContents, err := json.Marshal(fileVars)
			Expect(err).ToNot(HaveOccurred())

			err = ioutil.WriteFile(varFile, fileContents, 0600)
			Expect(err).ToNot(HaveOccurred())

			model := terraform.Model{
				VarFile: varFile,
			}

			err = model.ParseVarsFromFile()
			Expect(err).ToNot(HaveOccurred())

			Expect(model.Vars).To(Equal(fileVars))
		})

		It("returns original vars and vars from Merged model", func() {
			baseModel := terraform.Model{
				Source:  "base-source",
				VarFile: "base-file",
				Vars: map[string]interface{}{
					"base-key":     "base-value",
					"override-key": "base-override",
				},
			}
			mergeModel := terraform.Model{
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

			model := terraform.Model{
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
