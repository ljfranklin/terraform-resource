package models_test

import (
	"time"

	"github.com/ljfranklin/terraform-resource/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Models", func() {

	Describe("InRequest.Validate", func() {

		It("returns nil if all fields are provided", func() {
			req := models.InRequest{
				Source: models.Source{
					Storage: models.Storage{
						Driver:          models.S3Driver,
						Bucket:          "fake-bucket",
						RegionName:      "fake-region",
						Key:             "fake-key",
						AccessKeyID:     "fake-access-key",
						SecretAccessKey: "fake-secret-key",
					},
				},
				Version: models.Version{
					Version: time.Now().UTC().Format(time.RFC3339),
				},
			}

			err := req.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error if storage fields are missing", func() {
			requiredFields := []string{
				"source.storage.bucket",
				"source.storage.key",
				"source.storage.access_key_id",
				"source.storage.secret_access_key",
			}

			req := models.InRequest{}
			err := req.Validate()
			Expect(err).To(HaveOccurred())
			for _, field := range requiredFields {
				Expect(err.Error()).To(ContainSubstring(field))
			}
		})

		It("returns error if storage driver is unknown", func() {
			req := models.InRequest{
				Source: models.Source{
					Storage: models.Storage{
						Driver: "bad-driver",
					},
				},
			}
			err := req.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bad-driver"))
		})
	})

	Describe("OutRequest.Validate", func() {

		It("returns nil if all fields are provided", func() {
			req := models.OutRequest{
				Source: models.Source{
					Storage: models.Storage{
						Driver:          models.S3Driver,
						Bucket:          "fake-bucket",
						RegionName:      "fake-region",
						Key:             "fake-key",
						AccessKeyID:     "fake-access-key",
						SecretAccessKey: "fake-secret-key",
					},
				},
			}

			err := req.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error if storage fields are missing", func() {
			requiredFields := []string{
				"source.storage.bucket",
				"source.storage.key",
				"source.storage.access_key_id",
				"source.storage.secret_access_key",
			}

			req := models.OutRequest{}
			err := req.Validate()
			Expect(err).To(HaveOccurred())
			for _, field := range requiredFields {
				Expect(err.Error()).To(ContainSubstring(field))
			}
		})

		It("returns error if storage driver is unknown", func() {
			req := models.OutRequest{
				Source: models.Source{
					Storage: models.Storage{
						Driver: "bad-driver",
					},
				},
			}
			err := req.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bad-driver"))
		})
	})
})
