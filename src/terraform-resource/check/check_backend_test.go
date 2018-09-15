package check_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"terraform-resource/check"
	"terraform-resource/models"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Check with Terraform Backend", func() {

	var (
		checkInput          models.InRequest
		bucket              string
		prevEnvName         string
		currEnvName         string
		pathToPrevS3Fixture string
		pathToCurrS3Fixture string
		awsVerifier         *helpers.AWSVerifier
		workingDir          string
		expectedLineage     = "f62eee11-6a4e-4d39-b5c7-15d3dad8e5f7"
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

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-check-backend-test")
		Expect(err).ToNot(HaveOccurred())

		// ensure relative paths resolve correctly
		err = os.Chdir(workingDir)
		Expect(err).ToNot(HaveOccurred())

		fixturesDir := path.Join(helpers.ProjectRoot(), "fixtures")
		err = exec.Command("cp", "-r", fixturesDir, workingDir).Run()
		Expect(err).ToNot(HaveOccurred())

		workspacePath := helpers.RandomString("check-backend-test")

		prevEnvName = "s3-test-fixture-previous"
		currEnvName = "s3-test-fixture-current"
		pathToPrevS3Fixture = path.Join(workspacePath, prevEnvName, "terraform.tfstate")
		pathToCurrS3Fixture = path.Join(workspacePath, currEnvName, "terraform.tfstate")

		checkInput = models.InRequest{
			Source: models.Source{
				Terraform: models.Terraform{
					Source:      "unused",
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
	})

	AfterEach(func() {
		_ = os.RemoveAll(workingDir)
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
			prevFixture, err := os.Open(helpers.FileLocation("fixtures/s3-backend/terraform-previous.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer prevFixture.Close()

			awsVerifier.UploadObjectToS3(bucket, pathToPrevS3Fixture, prevFixture)

			currFixture, err := os.Open(helpers.FileLocation("fixtures/s3-backend/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToCurrS3Fixture, currFixture)
		})

		Context("when watching a single env with `source.env_name`", func() {
			BeforeEach(func() {
				checkInput.Source.EnvName = currEnvName
			})

			It("returns the latest version from the backend when no version is given", func() {
				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: currEnvName,
						Lineage: expectedLineage,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})

			It("returns the latest version when the given version matches latest version", func() {
				checkInput.Version = models.Version{
					Serial:  "1",
					EnvName: currEnvName,
					Lineage: expectedLineage,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: currEnvName,
						Lineage: expectedLineage,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})

			It("returns the latest version when the given version has a lower serial number", func() {
				checkInput.Version = models.Version{
					Serial:  "0",
					EnvName: currEnvName,
					Lineage: expectedLineage,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: currEnvName,
						Lineage: expectedLineage,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})

			It("returns an empty version list when the given version has a higher serial number", func() {
				checkInput.Version = models.Version{
					Serial:  "2",
					EnvName: currEnvName,
					Lineage: expectedLineage,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{}
				Expect(resp).To(Equal(expectOutput))
			})

			It("sorts the serial numerically", func() {
				checkInput.Version = models.Version{
					Serial:  "10",
					EnvName: currEnvName,
					Lineage: expectedLineage,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{}
				Expect(resp).To(Equal(expectOutput))
			})

			It("returns the latest version when the given lineage has changed", func() {
				checkInput.Version = models.Version{
					Serial:  "2",
					EnvName: currEnvName,
					Lineage: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: currEnvName,
						Lineage: expectedLineage,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})

			It("returns the latest version when the lineage is omitted", func() {
				checkInput.Version = models.Version{
					Serial:  "2",
					EnvName: currEnvName,
					Lineage: "",
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: currEnvName,
						Lineage: expectedLineage,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})

			It("can run twice in a row", func() {
				runner := check.Runner{}

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: currEnvName,
						Lineage: expectedLineage,
					},
				}

				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).To(Equal(expectOutput))

				resp, err = runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).To(Equal(expectOutput))
			})
		})

		Context("when watching a multiple envs with `source.env_name` unset", func() {
			BeforeEach(func() {
				checkInput.Source.EnvName = ""
			})

			It("returns an empty version list when no version is given", func() {
				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{}
				Expect(resp).To(Equal(expectOutput))
			})

			It("returns the latest version when the given version matches latest version", func() {
				checkInput.Version = models.Version{
					Serial:  "1",
					EnvName: currEnvName,
					Lineage: expectedLineage,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: currEnvName,
						Lineage: expectedLineage,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})
		})
	})
})
