package out_test

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"terraform-resource/models"
	"terraform-resource/namer/namerfakes"
	"terraform-resource/out"
	"terraform-resource/storage"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out - Migrated From Storage", func() {

	var (
		backendType          string
		backendConfig        map[string]interface{}
		storageModel         storage.Model
		envName              string
		backendStateFilePath string
		storageStateFilePath string
		s3ObjectPath         string
		workingDir           string
		namer                namerfakes.FakeNamer
		assertOutBehavior    func(models.OutRequest, map[string]string)
		// createYAMLTmpFile func(string, interface{}) string
		calculateMD5  func(string) string
		logWriter     bytes.Buffer
		workspacePath string
	)

	BeforeEach(func() {
		region := os.Getenv("AWS_REGION") // optional
		if region == "" {
			region = "us-east-1"
		}

		// TODO: workspace_prefix can't include nested dir
		workspacePath = helpers.RandomString("out-backend-test")

		envName = helpers.RandomString("out-test")
		backendStateFilePath = path.Join(workspacePath, envName, "terraform.tfstate")
		storageStateFilePath = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", envName))
		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-test"))

		os.Setenv("BUILD_ID", "sample-build-id")
		os.Setenv("BUILD_NAME", "sample-build-name")
		os.Setenv("BUILD_JOB_NAME", "sample-build-job-name")
		os.Setenv("BUILD_PIPELINE_NAME", "sample-build-pipeline-name")
		os.Setenv("BUILD_TEAM_NAME", "sample-build-team-name")
		os.Setenv("ATC_EXTERNAL_URL", "sample-atc-external-url")

		backendType = "s3"
		backendConfig = map[string]interface{}{
			"bucket":               bucket,
			"key":                  "terraform.tfstate",
			"access_key":           accessKey,
			"secret_key":           secretKey,
			"region":               region,
			"workspace_key_prefix": workspacePath,
		}

		storageModel = storage.Model{
			Bucket:          bucket,
			BucketPath:      bucketPath,
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
			RegionName:      region,
		}

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-test")
		Expect(err).ToNot(HaveOccurred())

		// ensure relative paths resolve correctly
		err = os.Chdir(workingDir)
		Expect(err).ToNot(HaveOccurred())

		fixturesDir := path.Join(helpers.ProjectRoot(), "fixtures")
		err = exec.Command("cp", "-r", fixturesDir, workingDir).Run()
		Expect(err).ToNot(HaveOccurred())

		namer = namerfakes.FakeNamer{}
	})

	AfterEach(func() {
		_ = os.RemoveAll(workingDir)
		awsVerifier.DeleteObjectFromS3(bucket, s3ObjectPath)
		awsVerifier.DeleteObjectFromS3(bucket, backendStateFilePath)
		awsVerifier.DeleteObjectFromS3(bucket, storageStateFilePath)
	})

	Context("when env does not already exist", func() {
		It("creates the env using the backend config", func() {
			req := models.OutRequest{
				Source: models.Source{
					Terraform: models.Terraform{
						BackendType:   backendType,
						BackendConfig: backendConfig,
					},
					MigratedFromStorage: storageModel,
				},
				Params: models.OutParams{
					EnvName: envName,
					Terraform: models.Terraform{
						Source: "fixtures/aws/",
						Vars: map[string]interface{}{
							"access_key":     accessKey,
							"secret_key":     secretKey,
							"bucket":         bucket,
							"object_key":     s3ObjectPath,
							"object_content": "terraform-is-neat",
							"region":         region,
						},
					},
				},
			}
			expectedMetadata := map[string]string{
				"env_name":    envName,
				"content_md5": calculateMD5("terraform-is-neat"),
			}

			assertOutBehavior(req, expectedMetadata)

			awsVerifier.ExpectS3FileToExist(bucket, s3ObjectPath)
			awsVerifier.ExpectS3FileToExist(bucket, backendStateFilePath)
		})
	})

	Context("when the env exists in Legacy Storage", func() {
		var (
			storageS3ObjectPath string
		)

		BeforeEach(func() {
			storageS3ObjectPath = helpers.RandomString("out-migrated-s3-object")

			req := models.OutRequest{
				Source: models.Source{
					Storage: storageModel,
				},
				Params: models.OutParams{
					EnvName: envName,
					Terraform: models.Terraform{
						Source: "fixtures/aws/",
						Vars: map[string]interface{}{
							"access_key":     accessKey,
							"secret_key":     secretKey,
							"bucket":         bucket,
							"object_key":     storageS3ObjectPath,
							"object_content": "terraform-is-neat",
							"region":         region,
						},
					},
				},
			}
			expectedMetadata := map[string]string{
				"env_name":    envName,
				"content_md5": calculateMD5("terraform-is-neat"),
			}
			assertOutBehavior(req, expectedMetadata)

			awsVerifier.ExpectS3FileToExist(bucket, storageStateFilePath)
			awsVerifier.ExpectS3FileToExist(bucket, storageS3ObjectPath)
		})

		It("migrates the env to the Backend", func() {
			req := models.OutRequest{
				Source: models.Source{
					Terraform: models.Terraform{
						BackendType:   backendType,
						BackendConfig: backendConfig,
					},
					MigratedFromStorage: storageModel,
				},
				Params: models.OutParams{
					EnvName: envName,
					Terraform: models.Terraform{
						Source: "fixtures/aws/",
						Vars: map[string]interface{}{
							"access_key":     accessKey,
							"secret_key":     secretKey,
							"bucket":         bucket,
							"object_key":     s3ObjectPath,
							"object_content": "terraform-is-neat",
							"region":         region,
						},
					},
				},
			}
			expectedMetadata := map[string]string{
				"env_name":    envName,
				"content_md5": calculateMD5("terraform-is-neat"),
			}

			assertOutBehavior(req, expectedMetadata)

			awsVerifier.ExpectS3FileToExist(bucket, s3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, storageS3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, storageStateFilePath)
			awsVerifier.ExpectS3FileToExist(bucket, backendStateFilePath)
			awsVerifier.ExpectS3FileToExist(bucket, fmt.Sprintf("%s.migrated", storageStateFilePath))
		})
	})
	// TODO: can we migrate a legacy tainted env?
	//
	// TODO: can we destroy a backend env?
	//
	// TODO: can we destroy a legacy env?

	It("returns an error if random name clashes with an existing Backend env", func() {
		// pick a name that always clashes with Backend env
		stateFixture, err := os.Open(helpers.FileLocation("fixtures/s3-backend/terraform-current.tfstate"))
		Expect(err).ToNot(HaveOccurred())
		defer stateFixture.Close()
		awsVerifier.UploadObjectToS3(bucket, backendStateFilePath, stateFixture)
		namer.RandomNameReturns(envName)

		req := models.OutRequest{
			Source: models.Source{
				Terraform: models.Terraform{
					BackendType:   backendType,
					BackendConfig: backendConfig,
				},
				MigratedFromStorage: storageModel,
			},
			Params: models.OutParams{
				GenerateRandomName: true,
				Terraform: models.Terraform{
					Source: "fixtures/aws/",
					Vars: map[string]interface{}{
						"access_key":     accessKey,
						"secret_key":     secretKey,
						"bucket":         bucket,
						"object_key":     s3ObjectPath,
						"object_content": "terraform-is-neat",
						"region":         region,
					},
				},
			},
		}

		runner := out.Runner{
			SourceDir: workingDir,
			LogWriter: &bytes.Buffer{},
			Namer:     &namer,
		}
		_, err = runner.Run(req)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("random name"))
		Expect(namer.RandomNameCallCount()).To(Equal(out.NameClashRetries),
			"Expected RandomName to be called %d times", out.NameClashRetries)
	})

	It("returns an error if random name clashes with an existing Legacy Storage env", func() {
		// pick a name that always clashes with Storage env
		stateFixture, err := os.Open(helpers.FileLocation("fixtures/s3-storage/terraform-current.tfstate"))
		Expect(err).ToNot(HaveOccurred())
		defer stateFixture.Close()
		awsVerifier.UploadObjectToS3(bucket, storageStateFilePath, stateFixture)
		namer.RandomNameReturns(envName)

		req := models.OutRequest{
			Source: models.Source{
				Terraform: models.Terraform{
					BackendType:   backendType,
					BackendConfig: backendConfig,
				},
				MigratedFromStorage: storageModel,
			},
			Params: models.OutParams{
				GenerateRandomName: true,
				Terraform: models.Terraform{
					Source: "fixtures/aws/",
					Vars: map[string]interface{}{
						"access_key":     accessKey,
						"secret_key":     secretKey,
						"bucket":         bucket,
						"object_key":     s3ObjectPath,
						"object_content": "terraform-is-neat",
						"region":         region,
					},
				},
			},
		}

		runner := out.Runner{
			SourceDir: workingDir,
			LogWriter: &bytes.Buffer{},
			Namer:     &namer,
		}
		_, err = runner.Run(req)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("random name"))
		Expect(namer.RandomNameCallCount()).To(Equal(out.NameClashRetries),
			"Expected RandomName to be called %d times", out.NameClashRetries)
	})

	assertOutBehavior = func(outRequest models.OutRequest, expectedMetadata map[string]string) {
		runner := out.Runner{
			SourceDir: workingDir,
			LogWriter: &logWriter,
			Namer:     &namer,
		}
		resp, err := runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred(), "Logs: %s", logWriter.String())

		Expect(logWriter.String()).To(ContainSubstring("Apply complete!"))

		Expect(resp.Version.EnvName).To(Equal(envName))

		Expect(resp.Metadata).ToNot(BeEmpty())
		fields := map[string]string{}
		for _, field := range resp.Metadata {
			fields[field.Name] = field.Value
		}

		for key, value := range expectedMetadata {
			Expect(fields[key]).To(Equal(value))
		}

		Expect(fields).To(HaveKey("terraform_version"))
		Expect(fields["terraform_version"]).To(MatchRegexp("Terraform v.*"))
	}

	calculateMD5 = func(content string) string {
		return fmt.Sprintf("%x", md5.Sum([]byte(content)))
	}
})
