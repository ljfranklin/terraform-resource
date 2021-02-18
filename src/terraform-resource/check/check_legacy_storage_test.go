package check_test

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/ljfranklin/terraform-resource/check"
	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Check with Legacy `storage`", func() {

	var (
		checkInput          models.InRequest
		bucket              string
		prevEnvName         string
		currEnvName         string
		pathToPrevS3Fixture string
		pathToCurrS3Fixture string
		awsVerifier         *helpers.AWSVerifier
	)

	BeforeEach(func() {
		accessKey := os.Getenv("AWS_ACCESS_KEY")
		Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")

		secretKey := os.Getenv("AWS_SECRET_KEY")
		Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")

		bucket = os.Getenv("AWS_BUCKET")
		Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")

		bucketPath := os.Getenv("AWS_BUCKET_SUBFOLDER")
		Expect(bucketPath).ToNot(BeEmpty(), "AWS_BUCKET_SUBFOLDER must be set")
		// create nested folder to all running in parallel
		bucketPath = path.Join(bucketPath, helpers.RandomString("check-storage-test"))

		region := os.Getenv("AWS_REGION") // optional
		if region == "" {
			region = "us-east-1"
		}

		awsVerifier = helpers.NewAWSVerifier(
			accessKey,
			secretKey,
			region,
			"",
		)
		prevEnvName = helpers.RandomString("s3-test-fixture-previous.tfstate")
		currEnvName = helpers.RandomString("s3-test-fixture-current.tfstate")
		pathToPrevS3Fixture = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", prevEnvName))
		pathToCurrS3Fixture = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", currEnvName))

		checkInput = models.InRequest{
			Source: models.Source{
				Storage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					RegionName:      region,
				},
			},
		}
	})

	AfterEach(func() {
		awsVerifier.DeleteObjectFromS3(bucket, pathToPrevS3Fixture)
		awsVerifier.DeleteObjectFromS3(bucket, pathToCurrS3Fixture)
	})

	Context("when bucket is empty", func() {
		It("returns an empty version list", func() {
			runner := check.Runner{}
			resp, err := runner.Run(checkInput)
			Expect(err).ToNot(HaveOccurred())

			expectedOutput := []models.Version{}
			Expect(resp).To(Equal(expectedOutput))
		})
	})

	Context("when bucket contains multiple state files", func() {
		BeforeEach(func() {
			prevFixture, err := os.Open(helpers.FileLocation("fixtures/s3-storage/terraform-previous.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer prevFixture.Close()

			awsVerifier.UploadObjectToS3(bucket, pathToPrevS3Fixture, prevFixture)
			time.Sleep(5 * time.Second) // ensure last modified is different

			currFixture, err := os.Open(helpers.FileLocation("fixtures/s3-storage/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToCurrS3Fixture, currFixture)
		})

		It("returns the latest version on S3", func() {
			runner := check.Runner{}
			resp, err := runner.Run(checkInput)
			Expect(err).ToNot(HaveOccurred())

			lastModified := awsVerifier.GetLastModifiedFromS3(bucket, pathToCurrS3Fixture)

			expectOutput := []models.Version{
				models.Version{
					LastModified: lastModified,
					EnvName:      currEnvName,
				},
			}
			Expect(resp).To(Equal(expectOutput))
		})

		It("returns the requested version when current version matches storage version", func() {
			currentLastModified := awsVerifier.GetLastModifiedFromS3(bucket, pathToCurrS3Fixture)
			checkInput.Version = models.Version{
				LastModified: currentLastModified,
				EnvName:      currEnvName,
			}

			runner := check.Runner{}
			resp, err := runner.Run(checkInput)
			Expect(err).ToNot(HaveOccurred())

			expectOutput := []models.Version{
				models.Version{
					LastModified: currentLastModified,
					EnvName:      currEnvName,
				},
			}
			Expect(resp).To(Equal(expectOutput))
		})
	})

	Context("when bucket contains a tainted state file", func() {
		var pathToTaintedFixture string

		BeforeEach(func() {
			prevFixture, err := os.Open(helpers.FileLocation("fixtures/s3-storage/terraform-previous.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer prevFixture.Close()

			pathToTaintedFixture = fmt.Sprintf("%s.tainted", pathToPrevS3Fixture)
			awsVerifier.UploadObjectToS3(bucket, pathToTaintedFixture, prevFixture)
		})

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, pathToTaintedFixture)
		})

		It("returns an empty version list", func() {
			runner := check.Runner{}
			resp, err := runner.Run(checkInput)
			Expect(err).ToNot(HaveOccurred())

			expectOutput := []models.Version{}
			Expect(resp).To(Equal(expectOutput))
		})
	})
})
