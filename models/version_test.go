package models_test

import (
	"time"

	"github.com/ljfranklin/terraform-resource/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version", func() {

	Describe("#Validate", func() {
		It("returns nil if all fields are provided", func() {
			model := models.Version{
				LastModified: "2006-01-02T15:04:05Z",
				EnvName:      "fake-env",
			}

			err := model.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error if storage fields are missing", func() {
			requiredFields := []string{
				"version.last_modified",
				"version.env_name",
			}

			version := models.Version{}
			err := version.Validate()
			Expect(err).To(HaveOccurred())
			for _, field := range requiredFields {
				Expect(err.Error()).To(ContainSubstring(field))
			}
		})

		It("returns error if LastModified is in invalid format", func() {
			model := models.Version{
				LastModified: "Mon Jan _2 15:04:05 2006",
				EnvName:      "fake-env",
			}
			err := model.Validate()
			expectedErr := "LastModified field is in invalid format"
			Expect(err).To(MatchError(ContainSubstring(expectedErr)))
		})
	})

	Describe("#IsZero", func() {
		It("returns false if a field is provided", func() {
			model := models.Version{
				LastModified: "2006-01-02T15:04:05Z",
			}

			Expect(model.IsZero()).To(BeFalse(), "Expected IsZero() to be false")
		})

		It("returns true if no fields are provided", func() {
			model := models.Version{}

			Expect(model.IsZero()).To(BeTrue(), "Expected IsZero() to be true")
		})
	})

	Describe("#LastModifiedTime", func() {
		It("returns the LastModified value as a Time struct", func() {
			now := time.Now()
			model := models.Version{
				LastModified: now.Format(models.TimeFormat),
			}

			Expect(model.LastModifiedTime().Unix()).To(Equal(now.Unix()))
		})
	})
})
