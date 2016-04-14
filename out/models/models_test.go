package models_test

import (
	"github.com/ljfranklin/terraform-resource/out/models"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/terraform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out Models", func() {

	var (
		validStorage   storage.Model
		validTerraform terraform.Model
	)

	BeforeEach(func() {
		validStorage = storage.Model{
			Driver:          storage.S3Driver,
			Bucket:          "fake-bucket",
			RegionName:      "fake-region",
			Key:             "fake-key",
			AccessKeyID:     "fake-access-key",
			SecretAccessKey: "fake-secret-key",
		}
		validTerraform = terraform.Model{
			Source: "fake-source",
		}
	})

	Describe("#Validate", func() {

		It("returns nil if all fields are provided", func() {
			req := models.OutRequest{
				Source: models.Source{
					Storage:   validStorage,
					Terraform: validTerraform,
				},
			}

			err := req.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error if storage is invalid", func() {
			req := models.OutRequest{
				Source: models.Source{
					Storage:   storage.Model{},
					Terraform: validTerraform,
				},
			}

			err := req.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("storage"))
		})
	})
})
