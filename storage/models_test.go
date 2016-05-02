package storage_test

import (
	"time"

	"github.com/ljfranklin/terraform-resource/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Storage Models", func() {

	Describe("Model", func() {

		Describe("#Validate", func() {

			It("returns nil if all fields are provided", func() {
				model := storage.Model{
					Driver:          storage.S3Driver,
					Bucket:          "fake-bucket",
					BucketPath:      "fake-bucket-path",
					AccessKeyID:     "fake-access-key",
					SecretAccessKey: "fake-secret-key",
					RegionName:      "fake-region",
				}

				err := model.Validate()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns error if storage fields are missing", func() {
				requiredFields := []string{
					"storage.bucket",
					"storage.bucket_path",
					"storage.access_key_id",
					"storage.secret_access_key",
				}

				model := storage.Model{}
				err := model.Validate()
				Expect(err).To(HaveOccurred())
				for _, field := range requiredFields {
					Expect(err.Error()).To(ContainSubstring(field))
				}
			})

			It("returns error if storage driver is unknown", func() {
				model := storage.Model{
					Driver: "bad-driver",
				}
				err := model.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("bad-driver"))
			})
		})
	})

	Describe("Version", func() {
		Describe("#IsZero", func() {
			It("returns false if a field is provided", func() {
				model := storage.Version{
					LastModified: time.Now(),
				}

				Expect(model.IsZero()).To(BeFalse(), "Expected IsZero() to be false")
			})

			It("returns true if no fields are provided", func() {
				model := storage.Version{}

				Expect(model.IsZero()).To(BeTrue(), "Expected IsZero() to be true")
			})
		})
	})
})
