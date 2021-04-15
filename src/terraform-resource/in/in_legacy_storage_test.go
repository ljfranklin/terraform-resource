package in_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/ljfranklin/terraform-resource/in"
	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("In with legacy storage", func() {

	var (
		awsVerifier            *helpers.AWSVerifier
		inReq                  models.InRequest
		bucket                 string
		bucketPath             string
		prevEnvName            string
		currEnvName            string
		modulesEnvName         string
		pathToPrevS3Fixture    string
		pathToCurrS3Fixture    string
		pathToModulesS3Fixture string
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

		bucketPath = os.Getenv("AWS_BUCKET_SUBFOLDER")
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
		modulesEnvName = helpers.RandomString("s3-test-fixture-modules")
		pathToPrevS3Fixture = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", prevEnvName))
		pathToCurrS3Fixture = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", currEnvName))
		pathToModulesS3Fixture = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", modulesEnvName))

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
		tmpDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-in-test")
		Expect(err).ToNot(HaveOccurred())

		err = os.Chdir(tmpDir)
		Expect(err).ToNot(HaveOccurred())

		logWriter = bytes.Buffer{}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Context("when multiple state files exist on S3", func() {
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

			modulesFixture, err := os.Open(helpers.FileLocation("fixtures/s3-storage/terraform-modules.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToModulesS3Fixture, modulesFixture)
		})

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, pathToPrevS3Fixture)
			awsVerifier.DeleteObjectFromS3(bucket, pathToCurrS3Fixture)
			awsVerifier.DeleteObjectFromS3(bucket, pathToModulesS3Fixture)
		})

		It("fetches the state file matching the provided version", func() {
			inReq.Version = models.Version{
				LastModified: awsVerifier.GetLastModifiedFromS3(bucket, pathToPrevS3Fixture),
				EnvName:      prevEnvName,
			}

			runner := in.Runner{
				OutputDir: tmpDir,
				LogWriter: &logWriter,
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
			Expect(metadata["secret"]).To(Equal("<sensitive>"))

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
				LogWriter: &logWriter,
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

		It("returns an error when OutputModule is used", func() {
			inReq.Params.OutputModule = "module_1"
			inReq.Version = models.Version{
				LastModified: awsVerifier.GetLastModifiedFromS3(bucket, pathToModulesS3Fixture),
				EnvName:      modulesEnvName,
			}

			runner := in.Runner{
				OutputDir: tmpDir,
				LogWriter: &logWriter,
			}
			_, err := runner.Run(inReq)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(MatchRegexp("output_module"))
		})

		It("prints a deprecation warning for `storage`", func() {
			inReq.Version = models.Version{
				LastModified: awsVerifier.GetLastModifiedFromS3(bucket, pathToPrevS3Fixture),
				EnvName:      prevEnvName,
			}

			runner := in.Runner{
				OutputDir: tmpDir,
				LogWriter: &logWriter,
			}
			_, err := runner.Run(inReq)
			Expect(err).ToNot(HaveOccurred(), "Logs: %s", logWriter.String())

			Expect(logWriter.String()).To(MatchRegexp("storage.*deprecated"))
		})
	})

	Context("when state file exists as tainted on S3", func() {
		var (
			pathToTaintedS3Fixture string
		)

		BeforeEach(func() {
			pathToTaintedS3Fixture = path.Join(bucketPath, fmt.Sprintf("%s.tfstate.tainted", currEnvName))
			fixture, err := os.Open(helpers.FileLocation("fixtures/s3-storage/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer fixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToTaintedS3Fixture, fixture)
		})

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, pathToTaintedS3Fixture)
		})

		It("fetches the state file matching the provided version", func() {
			inReq.Version = models.Version{
				LastModified: awsVerifier.GetLastModifiedFromS3(bucket, pathToTaintedS3Fixture),
				EnvName:      currEnvName,
			}

			runner := in.Runner{
				OutputDir: tmpDir,
				LogWriter: &logWriter,
			}
			resp, err := runner.Run(inReq)
			Expect(err).ToNot(HaveOccurred())

			_, err = time.Parse(storage.TimeFormat, resp.Version.LastModified)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.Version.EnvName).To(Equal(currEnvName))

			metadata := map[string]string{}
			for _, field := range resp.Metadata {
				metadata[field.Name] = field.Value
			}
			Expect(metadata["env_name"]).To(Equal("current"))

			expectedOutputPath := path.Join(tmpDir, "metadata")
			Expect(expectedOutputPath).To(BeAnExistingFile())
			outputFile, err := os.Open(expectedOutputPath)
			Expect(err).ToNot(HaveOccurred())
			defer outputFile.Close()

			outputContents := map[string]interface{}{}
			err = json.NewDecoder(outputFile).Decode(&outputContents)
			Expect(err).ToNot(HaveOccurred())

			Expect(outputContents["env_name"]).To(Equal("current"))

			expectedNamePath := path.Join(tmpDir, "name")
			Expect(expectedNamePath).To(BeAnExistingFile())
			nameContents, err := ioutil.ReadFile(expectedNamePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(nameContents)).To(Equal(currEnvName))
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

			It("returns the deleted version, creates a name file, but does not create the metadata file", func() {

				runner := in.Runner{
					OutputDir: tmpDir,
					LogWriter: &logWriter,
				}
				resp, err := runner.Run(inReq)
				Expect(err).ToNot(HaveOccurred())

				_, err = time.Parse(storage.TimeFormat, resp.Version.LastModified)
				Expect(err).ToNot(HaveOccurred())

				expectedNamePath := path.Join(tmpDir, "name")
				Expect(expectedNamePath).To(BeAnExistingFile())
				nameContents, err := ioutil.ReadFile(expectedNamePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(nameContents)).To(Equal(currEnvName))

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
