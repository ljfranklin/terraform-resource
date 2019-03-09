package models_test

import (
	"terraform-resource/models"
	"terraform-resource/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Source Model", func() {

	DescribeTable("valid model configurations",
		func(model models.Source) {
			err := model.Validate()
			Expect(err).ToNot(HaveOccurred())
		},
		Entry("Backend", models.Source{
			EnvName: "some-env",
			Terraform: models.Terraform{
				Source:        "some-source",
				BackendType:   "some-backend",
				BackendConfig: map[string]interface{}{"some-key": "some-value"},
			},
		}),
		Entry("MigratedFromStorage", models.Source{
			EnvName: "some-env",
			MigratedFromStorage: storage.Model{
				Driver:          "s3",
				Bucket:          "some-bucket",
				BucketPath:      "some-path",
				AccessKeyID:     "some-key",
				SecretAccessKey: "some-secret",
			},
			Terraform: models.Terraform{
				Source:        "some-source",
				BackendType:   "some-backend",
				BackendConfig: map[string]interface{}{"some-key": "some-value"},
			},
		}),
		Entry("Legacy Storage", models.Source{
			EnvName: "some-env",
			Storage: storage.Model{
				Driver:          "s3",
				Bucket:          "some-bucket",
				BucketPath:      "some-path",
				AccessKeyID:     "some-key",
				SecretAccessKey: "some-secret",
			},
			Terraform: models.Terraform{
				Source: "some-source",
			},
		}),
	)

	DescribeTable("invalid model configurations",
		func(model models.Source, expectedMessage string) {
			err := model.Validate()
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(expectedMessage))
		},
		Entry("Backend and Legacy Storage", models.Source{
			EnvName: "some-env",
			Terraform: models.Terraform{
				Source:        "some-source",
				BackendType:   "some-backend",
				BackendConfig: map[string]interface{}{"some-key": "some-value"},
			},
			Storage: storage.Model{
				Driver:          "s3",
				Bucket:          "some-bucket",
				BucketPath:      "some-path",
				AccessKeyID:     "some-key",
				SecretAccessKey: "some-secret",
			},
		}, "Cannot specify both `backend_type` and `storage`"),
		Entry("MigratedFromStorage without Backend", models.Source{
			EnvName: "some-env",
			MigratedFromStorage: storage.Model{
				Driver:          "s3",
				Bucket:          "some-bucket",
				BucketPath:      "some-path",
				AccessKeyID:     "some-key",
				SecretAccessKey: "some-secret",
			},
			Terraform: models.Terraform{
				Source: "some-source",
			},
		}, "Must specify `backend_type` and `backend_config` when using `migrated_from_storage`"),
		Entry("MigratedFromStorage and Legacy Storage", models.Source{
			EnvName: "some-env",
			MigratedFromStorage: storage.Model{
				Driver:          "s3",
				Bucket:          "some-bucket",
				BucketPath:      "some-path",
				AccessKeyID:     "some-key",
				SecretAccessKey: "some-secret",
			},
			Storage: storage.Model{
				Driver:          "s3",
				Bucket:          "some-bucket",
				BucketPath:      "some-path",
				AccessKeyID:     "some-key",
				SecretAccessKey: "some-secret",
			},
			Terraform: models.Terraform{
				Source: "some-source",
			},
		}, "Cannot specify both `migrated_from_storage` and `storage`"),
		Entry("Unknown Legacy Storage driver", models.Source{
			EnvName: "some-env",
			Storage: storage.Model{
				Driver:          "bad-driver",
				Bucket:          "some-bucket",
				BucketPath:      "some-path",
				AccessKeyID:     "some-key",
				SecretAccessKey: "some-secret",
			},
			Terraform: models.Terraform{
				Source: "some-source",
			},
		}, "bad-driver"),
		Entry("Unknown MigratedFromStorage driver", models.Source{
			EnvName: "some-env",
			MigratedFromStorage: storage.Model{
				Driver:          "bad-driver",
				Bucket:          "some-bucket",
				BucketPath:      "some-path",
				AccessKeyID:     "some-key",
				SecretAccessKey: "some-secret",
			},
			Terraform: models.Terraform{
				Source:        "some-source",
				BackendType:   "some-backend",
				BackendConfig: map[string]interface{}{"some-key": "some-value"},
			},
		}, "bad-driver"),
	)
})
