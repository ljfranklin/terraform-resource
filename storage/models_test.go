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
					Key:             "fake-key",
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
					"storage.key",
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

		Describe("#Validate", func() {
			It("returns nil if all fields are provided", func() {
				model := storage.Version{
					LastModified: "2006-01-02T15:04:05Z",
					MD5:          "7a63ed6fbcede31d157c9c52a8cef535",
				}

				err := model.Validate()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns error if LastModified is in invalid format", func() {
				model := storage.Version{
					LastModified: "Mon Jan _2 15:04:05 2006",
				}
				err := model.Validate()
				expectedErr := "LastModified field is in invalid format"
				Expect(err).To(MatchError(ContainSubstring(expectedErr)))
			})
		})

		Describe("#IsZero", func() {
			It("returns false if a field is provided", func() {
				model := storage.Version{
					LastModified: "2006-01-02T15:04:05Z",
				}

				Expect(model.IsZero()).To(BeFalse(), "Expected IsZero() to be false")
			})

			It("returns true if no fields are provided", func() {
				model := storage.Version{}

				Expect(model.IsZero()).To(BeTrue(), "Expected IsZero() to be true")
			})
		})

		Describe("#LastModifiedTime", func() {
			It("returns the LastModified value as a Time struct", func() {
				now := time.Now()
				model := storage.Version{
					LastModified: now.Format(storage.TimeFormat),
				}

				Expect(model.LastModifiedTime().Unix()).To(Equal(now.Unix()))
			})
		})
	})
})
