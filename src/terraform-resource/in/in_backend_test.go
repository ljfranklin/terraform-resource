package in_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"

	"terraform-resource/in"
	"terraform-resource/models"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("In with Backend", func() {

	var (
		awsVerifier            *helpers.AWSVerifier
		inReq                  models.InRequest
		bucket                 string
		prevEnvName            string
		currEnvName            string
		modulesEnvName         string
		pathToPrevS3Fixture    string
		pathToCurrS3Fixture    string
		pathToModulesS3Fixture string
		tmpDir                 string
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
		modulesEnvName = helpers.RandomString("s3-test-fixture-modules")

		workspacePath := helpers.RandomString("in-backend-test")

		pathToPrevS3Fixture = path.Join(workspacePath, prevEnvName, "terraform.tfstate")
		pathToCurrS3Fixture = path.Join(workspacePath, currEnvName, "terraform.tfstate")
		pathToModulesS3Fixture = path.Join(workspacePath, modulesEnvName, "terraform.tfstate")

		inReq = models.InRequest{
			Source: models.Source{
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

		err = os.Chdir(tmpDir)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Context("when multiple state files exist on S3", func() {
		BeforeEach(func() {
			prevFixture, err := os.Open(helpers.FileLocation("fixtures/s3-backend/terraform-previous.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer prevFixture.Close()

			awsVerifier.UploadObjectToS3(bucket, pathToPrevS3Fixture, prevFixture)
			time.Sleep(5 * time.Second) // ensure last modified is different

			currFixture, err := os.Open(helpers.FileLocation("fixtures/s3-backend/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToCurrS3Fixture, currFixture)

			modulesFixture, err := os.Open(helpers.FileLocation("fixtures/s3-backend/terraform-modules.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer modulesFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToModulesS3Fixture, modulesFixture)
		})

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, pathToPrevS3Fixture)
			awsVerifier.DeleteObjectFromS3(bucket, pathToCurrS3Fixture)
			awsVerifier.DeleteObjectFromS3(bucket, pathToModulesS3Fixture)
		})

		It("fetches the state file matching the provided version", func() {
			inReq.Version = models.Version{
				EnvName: prevEnvName,
				Serial:  "0",
			}

			runner := in.Runner{
				OutputDir: tmpDir,
			}
			resp, err := runner.Run(inReq)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.Version.EnvName).To(Equal(prevEnvName))
			serial, err := strconv.Atoi(resp.Version.Serial)
			Expect(err).ToNot(HaveOccurred())
			Expect(serial).To(BeNumerically(">=", 0))
			Expect(resp.Version.Lineage).To(Equal("f62eee11-6a4e-4d39-b5c7-15d3dad8e5f7"))

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
				EnvName: prevEnvName,
				Serial:  "0",
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

			stateContents, err := ioutil.ReadFile(expectedStatePath)
			Expect(err).To(BeNil())
			Expect(string(stateContents)).To(ContainSubstring("previous"))
		})

		It("retrieve module specific output when `output_module` is specified", func() {
			inReq.Params.OutputModule = "module_1"
			inReq.Version = models.Version{
				EnvName: modulesEnvName,
				Serial:  "1",
			}

			runner := in.Runner{
				OutputDir: tmpDir,
			}
			resp, err := runner.Run(inReq)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.Version.EnvName).To(Equal(modulesEnvName))
			serial, err := strconv.Atoi(resp.Version.Serial)
			Expect(err).ToNot(HaveOccurred())
			Expect(serial).To(BeNumerically(">=", 1))

			metadata := map[string]string{}
			for _, field := range resp.Metadata {
				metadata[field.Name] = field.Value
			}
			Expect(metadata["terraform_version"]).To(MatchRegexp("Terraform v.*"))
			Expect(metadata["env_name"]).To(Equal("module_1"))
			Expect(metadata["secret"]).To(Equal("<sensitive>"))

			expectedOutputPath := path.Join(tmpDir, "metadata")
			Expect(expectedOutputPath).To(BeAnExistingFile())
			outputFile, err := os.Open(expectedOutputPath)
			Expect(err).ToNot(HaveOccurred())
			defer outputFile.Close()

			outputContents := map[string]interface{}{}
			err = json.NewDecoder(outputFile).Decode(&outputContents)
			Expect(err).ToNot(HaveOccurred())

			Expect(outputContents["env_name"]).To(Equal("module_1"))
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
			Expect(string(nameContents)).To(Equal(modulesEnvName))
		})
	})

	Context("when state file does not exist on S3", func() {

		Context("and it was called as part of the 'destroy' action", func() {

			BeforeEach(func() {
				inReq.Params.Action = models.DestroyAction
				inReq.Version = models.Version{
					EnvName: currEnvName,
					Serial:  "1",
				}
			})

			It("returns the deleted version, but does not create the metadata file", func() {

				runner := in.Runner{
					OutputDir: tmpDir,
				}
				resp, err := runner.Run(inReq)
				Expect(err).ToNot(HaveOccurred())

				Expect(resp.Version.EnvName).To(Equal(currEnvName))
				serial, err := strconv.Atoi(resp.Version.Serial)
				Expect(err).ToNot(HaveOccurred())
				Expect(serial).To(BeNumerically(">=", 1))

				expectedOutputPath := path.Join(tmpDir, "metadata")
				Expect(expectedOutputPath).ToNot(BeAnExistingFile())
			})
		})

		Context("and it was called with 'plan_only'", func() {
			BeforeEach(func() {
				inReq.Version = models.Version{
					EnvName:  currEnvName,
					Serial:   "1",
					PlanOnly: "true",
				}
			})

			It("returns the version, but does not create the metadata file", func() {
				runner := in.Runner{
					OutputDir: tmpDir,
				}
				resp, err := runner.Run(inReq)
				Expect(err).ToNot(HaveOccurred())

				Expect(resp.Version.EnvName).To(Equal(currEnvName))
				serial, err := strconv.Atoi(resp.Version.Serial)
				Expect(err).ToNot(HaveOccurred())
				Expect(serial).To(BeNumerically(">=", 1))

				expectedOutputPath := path.Join(tmpDir, "metadata")
				Expect(expectedOutputPath).ToNot(BeAnExistingFile())
			})
		})

		Context("and it was called as part of update or create", func() {
			BeforeEach(func() {
				inReq.Params.Action = ""
				inReq.Version = models.Version{
					EnvName: "missing-env-name",
					Serial:  "0",
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
