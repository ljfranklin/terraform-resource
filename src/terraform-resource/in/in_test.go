package in_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"terraform-resource/in"
	"terraform-resource/models"
	"terraform-resource/storage"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("In", func() {

	var (
		awsVerifier         *helpers.AWSVerifier
		inReq               models.InRequest
		bucket              string
		prevEnvName         string
		currEnvName         string
		pathToPrevS3Fixture string
		pathToCurrS3Fixture string
		tmpDir              string
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
		prevEnvName = helpers.RandomString("s3-test-fixture-previous")
		currEnvName = helpers.RandomString("s3-test-fixture-current")
		pathToPrevS3Fixture = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", prevEnvName))
		pathToCurrS3Fixture = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", currEnvName))

		inReq = models.InRequest{
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

		var err error
		tmpDir, err = ioutil.TempDir("", "terraform-resource-in-test")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Context("when multiple state files exist on S3", func() {
		BeforeEach(func() {
			prevFixture, err := os.Open(helpers.FileLocation("fixtures/s3/terraform-previous.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer prevFixture.Close()

			awsVerifier.UploadObjectToS3(bucket, pathToPrevS3Fixture, prevFixture)
			time.Sleep(5 * time.Second) // ensure last modified is different

			currFixture, err := os.Open(helpers.FileLocation("fixtures/s3/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToCurrS3Fixture, currFixture)
		})

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, pathToPrevS3Fixture)
			awsVerifier.DeleteObjectFromS3(bucket, pathToCurrS3Fixture)
		})

		It("fetches the state file matching the provided version", func() {

			inReq.Version = models.Version{
				LastModified: awsVerifier.GetLastModifiedFromS3(bucket, pathToPrevS3Fixture),
				EnvName:      prevEnvName,
			}

			runner := in.Runner{
				OutputDir: tmpDir,
			}
			resp, err := runner.Run(inReq)
			Expect(err).ToNot(HaveOccurred())

			_, err = time.Parse(storage.TimeFormat, resp.Version.LastModified)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.Version.EnvName).To(Equal(prevEnvName))

			metadata := map[string]string{}
			for _, field := range resp.Metadata {
				metadata[field.Name] = field.Value
			}
			Expect(metadata["terraform_version"]).To(MatchRegexp("Terraform v.*"))
			Expect(metadata["env_name"]).To(Equal("previous"))
			Expect(metadata["secret"]).To(Equal(`\u003csensitive\u003e`)) // JSON encoder escapes < and >

			expectedOutputPath := path.Join(tmpDir, "metadata")
			Expect(expectedOutputPath).To(BeAnExistingFile())
			outputFile, err := os.Open(expectedOutputPath)
			Expect(err).ToNot(HaveOccurred())
			defer outputFile.Close()

			outputContents := map[string]interface{}{}
			err = json.NewDecoder(outputFile).Decode(&outputContents)
			Expect(err).ToNot(HaveOccurred())

			Expect(outputContents["env_name"]).To(Equal("previous"))
			Expect(outputContents["map"]).To(Equal(map[string]interface{}{
				"key-1": "value-1",
				"key-2": "value-2",
			}))
			Expect(outputContents["list"]).To(Equal([]interface{}{
				"item-1",
				"item-2",
			}))
			Expect(outputContents["secret"]).To(Equal("super-secret"))

			expectedNamePath := path.Join(tmpDir, "name")
			Expect(expectedNamePath).To(BeAnExistingFile())
			nameContents, err := ioutil.ReadFile(expectedNamePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(nameContents)).To(Equal(prevEnvName))
		})

		It("outputs the statefile if `output_statefile` is given", func() {
			inReq.Params.OutputStatefile = true
			inReq.Version = models.Version{
				LastModified: awsVerifier.GetLastModifiedFromS3(bucket, pathToPrevS3Fixture),
				EnvName:      prevEnvName,
			}

			runner := in.Runner{
				OutputDir: tmpDir,
			}
			_, err := runner.Run(inReq)
			Expect(err).ToNot(HaveOccurred())

			expectedOutputPath := path.Join(tmpDir, "metadata")
			Expect(expectedOutputPath).To(BeAnExistingFile())

			expectedNamePath := path.Join(tmpDir, "name")
			Expect(expectedNamePath).To(BeAnExistingFile())

			expectedStatePath := path.Join(tmpDir, "terraform.tfstate")
			Expect(expectedStatePath).To(BeAnExistingFile())
		})
	})

	Context("when state file does not exist on S3", func() {

		Context("and it was called as part of the 'destroy' action", func() {

			BeforeEach(func() {
				inReq.Params.Action = models.DestroyAction
				inReq.Version = models.Version{
					LastModified: time.Now().UTC().Format(storage.TimeFormat),
					EnvName:      currEnvName,
				}
			})

			It("returns the deleted version, but does not create the metadata file", func() {

				runner := in.Runner{
					OutputDir: tmpDir,
				}
				resp, err := runner.Run(inReq)
				Expect(err).ToNot(HaveOccurred())

				_, err = time.Parse(storage.TimeFormat, resp.Version.LastModified)
				Expect(err).ToNot(HaveOccurred())

				expectedOutputPath := path.Join(tmpDir, "metadata")
				Expect(expectedOutputPath).ToNot(BeAnExistingFile())
			})
		})

		Context("and it was called with 'plan_only'", func() {
			BeforeEach(func() {
				inReq.Version = models.Version{
					LastModified: time.Now().UTC().Format(storage.TimeFormat),
					EnvName:      currEnvName,
					PlanOnly:     "true",
				}
			})

			It("returns the version, but does not create the metadata file", func() {
				runner := in.Runner{
					OutputDir: tmpDir,
				}
				resp, err := runner.Run(inReq)
				Expect(err).ToNot(HaveOccurred())

				_, err = time.Parse(storage.TimeFormat, resp.Version.LastModified)
				Expect(err).ToNot(HaveOccurred())

				expectedOutputPath := path.Join(tmpDir, "metadata")
				Expect(expectedOutputPath).ToNot(BeAnExistingFile())
			})
		})

		Context("and it was called as part of update or create", func() {
			BeforeEach(func() {
				inReq.Params.Action = ""
				inReq.Version = models.Version{
					LastModified: time.Now().UTC().Format(storage.TimeFormat),
					EnvName:      "missing-env-name",
				}
			})

			It("returns an error", func() {
				runner := in.Runner{
					OutputDir: tmpDir,
				}
				_, err := runner.Run(inReq)
				Expect(err).To(HaveOccurred())

				Expect(err.Error()).To(ContainSubstring("missing-env-name"))
				Expect(err.Error()).To(ContainSubstring("get_params"))
			})
		})
	})
})
