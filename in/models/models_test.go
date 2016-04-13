package models_test

import (
	"time"

	"github.com/ljfranklin/terraform-resource/in/models"
	"github.com/ljfranklin/terraform-resource/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InRequest", func() {

	var (
		validStorage storage.Model
		validVersion models.Version
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
		validVersion = models.Version{
			Version: time.Now().UTC().Format(time.RFC3339),
		}
	})

	Describe("#Validate", func() {

		It("returns nil if all fields are provided", func() {
			req := models.InRequest{
				Source: models.Source{
					Storage: validStorage,
				},
				Version: validVersion,
			}

			err := req.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error if storage is invalid", func() {
			req := models.InRequest{
				Source: models.Source{
					Storage: storage.Model{},
				},
				Version: validVersion,
			}

			err := req.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("storage"))
		})
	})
})
