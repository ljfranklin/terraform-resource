package in_test

import (
	"bytes"
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

var _ = Describe("In with migrated from storage", func() {

	var (
		awsVerifier            *helpers.AWSVerifier
		inReq                  models.InRequest
		bucket                 string
		envName                string
		pathToStorageS3Fixture string
		pathToBackendS3Fixture string
		tmpDir                 string
		logWriter              bytes.Buffer
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
		envName = helpers.RandomString("s3-test-fixture-migrated")

		// TODO: workspace_prefix can't include nested dir
		workspacePath := helpers.RandomString("in-backend-test")

		pathToStorageS3Fixture = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", envName))
		pathToBackendS3Fixture = path.Join(workspacePath, envName, "terraform.tfstate")

		inReq = models.InRequest{
			Source: models.Source{
				MigratedFromStorage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					RegionName:      region,
				},
				Terraform: models.Terraform{
					BackendType: "s3",
					BackendConfig: map[string]interface{}{
						"bucket":               bucket,
						"key":                  "terraform.tfstate",
						"access_key":           accessKey,
						"secret_key":           secretKey,
						"region":               region,
						"workspace_key_prefix": workspacePath,
					},
				},
			},
		}

		var err error
		tmpDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-in-test")
		Expect(err).ToNot(HaveOccurred())

		// TODO: should production code be changing dir instead?
		err = os.Chdir(tmpDir)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Context("when state file exists in Backend storage but not Legacy Storage", func() {
		BeforeEach(func() {
			currFixture, err := os.Open(helpers.FileLocation("fixtures/s3-backend/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToBackendS3Fixture, currFixture)
			time.Sleep(1 * time.Second)
		})

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, pathToStorageS3Fixture)
		})

		It("fetches the state file from Backend", func() {
			inReq.Version = models.Version{
				LastModified: awsVerifier.GetLastModifiedFromS3(bucket, pathToBackendS3Fixture),
				EnvName:      envName,
			}

			runner := in.Runner{
				OutputDir: tmpDir,
				LogWriter: &logWriter,
			}
			resp, err := runner.Run(inReq)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.Version.EnvName).To(Equal(envName))
			Expect(resp.Version.Serial).To(Equal(1))

			metadata := map[string]string{}
			for _, field := range resp.Metadata {
				metadata[field.Name] = field.Value
			}
			Expect(metadata["terraform_version"]).To(MatchRegexp("Terraform v.*"))
			Expect(metadata["env_name"]).To(Equal("current"))
			Expect(metadata["secret"]).To(Equal("<sensitive>"))

			expectedOutputPath := path.Join(tmpDir, "metadata")
			Expect(expectedOutputPath).To(BeAnExistingFile())
			outputFile, err := os.Open(expectedOutputPath)
			Expect(err).ToNot(HaveOccurred())
			defer outputFile.Close()

			outputContents := map[string]interface{}{}
			err = json.NewDecoder(outputFile).Decode(&outputContents)
			Expect(err).ToNot(HaveOccurred())

			Expect(outputContents["env_name"]).To(Equal("current"))
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
			Expect(string(nameContents)).To(Equal(envName))
		})
	})

	Context("when state file exists in Legacy Storage but not Backend", func() {
		BeforeEach(func() {
			currFixture, err := os.Open(helpers.FileLocation("fixtures/s3-storage/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToStorageS3Fixture, currFixture)
			time.Sleep(1 * time.Second)
		})

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, pathToStorageS3Fixture)
		})

		It("fetches the state file from Legacy Storage", func() {
			inReq.Version = models.Version{
				LastModified: awsVerifier.GetLastModifiedFromS3(bucket, pathToStorageS3Fixture),
				EnvName:      envName,
			}

			runner := in.Runner{
				OutputDir: tmpDir,
				LogWriter: &logWriter,
			}
			resp, err := runner.Run(inReq)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.Version.EnvName).To(Equal(envName))
			_, err = time.Parse(storage.TimeFormat, resp.Version.LastModified)
			Expect(err).ToNot(HaveOccurred())

			metadata := map[string]string{}
			for _, field := range resp.Metadata {
				metadata[field.Name] = field.Value
			}
			Expect(metadata["terraform_version"]).To(MatchRegexp("Terraform v.*"))
			Expect(metadata["env_name"]).To(Equal("current"))
			Expect(metadata["secret"]).To(Equal("<sensitive>"))

			expectedOutputPath := path.Join(tmpDir, "metadata")
			Expect(expectedOutputPath).To(BeAnExistingFile())
			outputFile, err := os.Open(expectedOutputPath)
			Expect(err).ToNot(HaveOccurred())
			defer outputFile.Close()

			outputContents := map[string]interface{}{}
			err = json.NewDecoder(outputFile).Decode(&outputContents)
			Expect(err).ToNot(HaveOccurred())

			Expect(outputContents["env_name"]).To(Equal("current"))
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
			Expect(string(nameContents)).To(Equal(envName))
		})
	})

	Context("when state file does not exist on S3", func() {

		Context("and it was called as part of the 'destroy' action", func() {

			BeforeEach(func() {
				inReq.Params.Action = models.DestroyAction
				inReq.Version = models.Version{
					LastModified: time.Now().UTC().Format(storage.TimeFormat),
					EnvName:      envName,
				}
			})

			It("returns the deleted version, but does not create the metadata file", func() {
				runner := in.Runner{
					OutputDir: tmpDir,
					LogWriter: &logWriter,
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
					LogWriter: &logWriter,
				}
				_, err := runner.Run(inReq)
				Expect(err).To(HaveOccurred())

				Expect(err.Error()).To(ContainSubstring("missing-env-name"))
				Expect(err.Error()).To(ContainSubstring("get_params"))
			})
		})
	})
})
