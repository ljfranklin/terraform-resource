package terraform_test

import (
	"github.com/ljfranklin/terraform-resource/terraform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Terraform Models", func() {

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
})
