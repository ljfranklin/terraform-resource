package check_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"terraform-resource/check"
	"terraform-resource/models"
	"terraform-resource/storage"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Check with Migrated From Storage", func() {

	var (
		checkInput             models.InRequest
		bucket                 string
		backendEnvName         string
		storageEnvName         string
		pathToBackendStatefile string
		pathToStorageStatefile string
		awsVerifier            *helpers.AWSVerifier
		workingDir             string
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

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-check-backend-test")
		Expect(err).ToNot(HaveOccurred())

		// ensure relative paths resolve correctly
		err = os.Chdir(workingDir)
		Expect(err).ToNot(HaveOccurred())

		fixturesDir := path.Join(helpers.ProjectRoot(), "fixtures")
		err = exec.Command("cp", "-r", fixturesDir, workingDir).Run()
		Expect(err).ToNot(HaveOccurred())

		// TODO: workspace_prefix can't include nested dir
		workspacePath := helpers.RandomString("check-backend-test")

		backendEnvName = "s3-test-fixture-backend"
		pathToBackendStatefile = path.Join(workspacePath, backendEnvName, "terraform.tfstate")
		storageEnvName = "s3-test-fixture-storage"
		pathToStorageStatefile = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", storageEnvName))

		checkInput = models.InRequest{
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
				MigratedFromStorage: storage.Model{
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
		_ = os.RemoveAll(workingDir)
		// TODO: do we need to delete parent folder?
		awsVerifier.DeleteObjectFromS3(bucket, pathToBackendStatefile)
		awsVerifier.DeleteObjectFromS3(bucket, pathToStorageStatefile)
	})

	Context("when both backend and legacy storage are empty", func() {
		It("returns an empty version list", func() {
			runner := check.Runner{}
			resp, err := runner.Run(checkInput)
			Expect(err).ToNot(HaveOccurred())

			expectedOutput := []models.Version{}
			Expect(resp).To(Equal(expectedOutput))
		})
	})

	Context("when both Backend and Legacy Storage contains state files", func() {
		BeforeEach(func() {
			// TODO: can we need current and previous fixtures?
			backendFixture, err := os.Open(helpers.FileLocation("fixtures/s3-backend/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer backendFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToBackendStatefile, backendFixture)

			storageFixture, err := os.Open(helpers.FileLocation("fixtures/s3-storage/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer storageFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToStorageStatefile, storageFixture)
			time.Sleep(1 * time.Second)
		})

		Context("when watching a single env with `source.env_name`", func() {
			BeforeEach(func() {
				checkInput.Source.EnvName = backendEnvName
			})

			It("returns the latest version from the backend when no version is given", func() {
				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: backendEnvName,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})

			It("returns the latest version when the given version matches latest version", func() {
				checkInput.Version = models.Version{
					Serial:  "1",
					EnvName: backendEnvName,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: backendEnvName,
					},
				}
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

			It("returns the latest version when the given backend version matches latest version", func() {
				checkInput.Version = models.Version{
					Serial:  "1",
					EnvName: backendEnvName,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: backendEnvName,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})

			It("returns the latest version when the given storage version matches latest version", func() {
				lastModified := awsVerifier.GetLastModifiedFromS3(bucket, pathToStorageStatefile)

				checkInput.Version = models.Version{
					LastModified: lastModified,
					EnvName:      storageEnvName,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						LastModified: lastModified,
						EnvName:      storageEnvName,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})
		})
	})

	Context("when only Backend contains state files", func() {
		BeforeEach(func() {
			// TODO: can we need current and previous fixtures?
			currFixture, err := os.Open(helpers.FileLocation("fixtures/s3-backend/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToBackendStatefile, currFixture)
			time.Sleep(1 * time.Second)
		})

		Context("when watching a single env with `source.env_name`", func() {
			BeforeEach(func() {
				checkInput.Source.EnvName = backendEnvName
			})

			It("returns the latest version from the backend when no version is given", func() {
				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: backendEnvName,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})

			It("returns the latest version when the given version matches latest version", func() {
				checkInput.Version = models.Version{
					Serial:  "1",
					EnvName: backendEnvName,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: backendEnvName,
					},
				}
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
					EnvName: backendEnvName,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						Serial:  "1",
						EnvName: backendEnvName,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})
		})
	})

	Context("when only Legacy Storage contains state files", func() {
		BeforeEach(func() {
			// TODO: can we need current and previous fixtures?
			currFixture, err := os.Open(helpers.FileLocation("fixtures/s3-storage/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToStorageStatefile, currFixture)
			time.Sleep(1 * time.Second)
		})

		Context("when watching a single env with `source.env_name`", func() {
			BeforeEach(func() {
				checkInput.Source.EnvName = storageEnvName
			})

			It("returns the latest version from the backend when no version is given", func() {
				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				lastModified := awsVerifier.GetLastModifiedFromS3(bucket, pathToStorageStatefile)

				expectOutput := []models.Version{
					models.Version{
						LastModified: lastModified,
						EnvName:      storageEnvName,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})

			It("returns the latest version when the given version matches latest version", func() {
				lastModified := awsVerifier.GetLastModifiedFromS3(bucket, pathToStorageStatefile)

				checkInput.Version = models.Version{
					LastModified: lastModified,
					EnvName:      storageEnvName,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						LastModified: lastModified,
						EnvName:      storageEnvName,
					},
				}
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
				lastModified := awsVerifier.GetLastModifiedFromS3(bucket, pathToStorageStatefile)

				checkInput.Version = models.Version{
					LastModified: lastModified,
					EnvName:      storageEnvName,
				}

				runner := check.Runner{}
				resp, err := runner.Run(checkInput)
				Expect(err).ToNot(HaveOccurred())

				expectOutput := []models.Version{
					models.Version{
						LastModified: lastModified,
						EnvName:      storageEnvName,
					},
				}
				Expect(resp).To(Equal(expectOutput))
			})
		})
	})
})
