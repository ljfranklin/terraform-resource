package out_test

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/namer/namerfakes"
	"github.com/ljfranklin/terraform-resource/out"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/test/helpers"

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
		planFilePath         string
		storageStateFilePath string
		s3ObjectPath         string
		workingDir           string
		namer                namerfakes.FakeNamer
		assertOutBehavior    func(models.OutRequest, map[string]string)
		calculateMD5         func(string) string
		logWriter            bytes.Buffer
		workspacePath        string
	)

	BeforeEach(func() {
		region := os.Getenv("AWS_REGION") // optional
		if region == "" {
			region = "us-east-1"
		}

		workspacePath = helpers.RandomString("out-backend-test")

		envName = helpers.RandomString("out-test")
		backendStateFilePath = path.Join(workspacePath, envName, "terraform.tfstate")
		planFilePath = path.Join(workspacePath, fmt.Sprintf("%s-plan", envName), "terraform.tfstate")
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

		logWriter = bytes.Buffer{}
	})

	AfterEach(func() {
		_ = os.RemoveAll(workingDir)
		awsVerifier.DeleteObjectFromS3(bucket, s3ObjectPath)
		awsVerifier.DeleteObjectFromS3(bucket, backendStateFilePath)
		awsVerifier.DeleteObjectFromS3(bucket, storageStateFilePath)
		awsVerifier.DeleteObjectFromS3(bucket, planFilePath)
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
			storageS3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-migrated-s3-object"))

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

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, storageS3ObjectPath)
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

		It("migrates the env even if the Apply fails", func() {
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
							"access_key":           accessKey,
							"secret_key":           secretKey,
							"bucket":               bucket,
							"object_key":           s3ObjectPath,
							"object_content":       "terraform-is-neat",
							"region":               region,
							"invalid_object_count": 1,
						},
					},
				},
			}
			runner := out.Runner{
				SourceDir: workingDir,
				LogWriter: &logWriter,
			}
			_, err := runner.Run(req)

			Expect(err).To(HaveOccurred())
			Expect(logWriter.String()).To(ContainSubstring("invalid_object"))

			awsVerifier.ExpectS3FileToExist(bucket, s3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, storageS3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, storageStateFilePath)
			awsVerifier.ExpectS3FileToExist(bucket, backendStateFilePath)
			awsVerifier.ExpectS3FileToExist(bucket, fmt.Sprintf("%s.migrated", storageStateFilePath))
		})

		It("destroys a legacy storage env if action=destroy", func() {
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
					Action:  "destroy",
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
				LogWriter: &logWriter,
				Namer:     &namer,
			}
			resp, err := runner.Run(req)
			Expect(err).ToNot(HaveOccurred(), "Logs: %s", logWriter.String())
			Expect(resp.Version.EnvName).To(Equal(envName))

			awsVerifier.ExpectS3FileToNotExist(bucket, s3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, storageS3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, storageStateFilePath)
			awsVerifier.ExpectS3FileToNotExist(bucket, backendStateFilePath)
			awsVerifier.ExpectS3FileToNotExist(bucket, fmt.Sprintf("%s.migrated", storageStateFilePath))
		})
	})

	Context("when the tainted env exists in Legacy Storage", func() {
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
							"access_key":           accessKey,
							"secret_key":           secretKey,
							"bucket":               bucket,
							"object_key":           storageS3ObjectPath,
							"object_content":       "terraform-is-neat",
							"region":               region,
							"invalid_object_count": 1,
						},
					},
				},
			}
			runner := out.Runner{
				SourceDir: workingDir,
				LogWriter: &logWriter,
			}
			_, err := runner.Run(req)

			Expect(err).To(HaveOccurred())
			Expect(logWriter.String()).To(ContainSubstring("invalid_object"))

			awsVerifier.ExpectS3FileToExist(bucket, storageS3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, storageStateFilePath)
			awsVerifier.ExpectS3FileToExist(bucket, fmt.Sprintf("%s.tainted", storageStateFilePath))
		})

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, storageS3ObjectPath)
		})

		It("migrates the tainted env to the Backend", func() {
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
			awsVerifier.ExpectS3FileToNotExist(bucket, fmt.Sprintf("%s.tainted", storageStateFilePath))
			awsVerifier.ExpectS3FileToExist(bucket, backendStateFilePath)
			awsVerifier.ExpectS3FileToExist(bucket, fmt.Sprintf("%s.migrated", storageStateFilePath))
		})

		It("destroys the tained env when action=destroy", func() {
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
					Action:  "destroy",
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
				LogWriter: &logWriter,
				Namer:     &namer,
			}
			resp, err := runner.Run(req)
			Expect(err).ToNot(HaveOccurred(), "Logs: %s", logWriter.String())
			Expect(resp.Version.EnvName).To(Equal(envName))

			awsVerifier.ExpectS3FileToNotExist(bucket, s3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, storageS3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, storageStateFilePath)
			awsVerifier.ExpectS3FileToNotExist(bucket, fmt.Sprintf("%s.tainted", storageStateFilePath))
			awsVerifier.ExpectS3FileToNotExist(bucket, backendStateFilePath)
			awsVerifier.ExpectS3FileToNotExist(bucket, fmt.Sprintf("%s.migrated", storageStateFilePath))
		})
	})

	Context("when the env exists in Backend", func() {
		var (
			backendS3ObjectPath string
		)

		BeforeEach(func() {
			backendS3ObjectPath = helpers.RandomString("out-migrated-s3-object")

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
							"object_key":     backendS3ObjectPath,
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

			awsVerifier.ExpectS3FileToExist(bucket, backendStateFilePath)
			awsVerifier.ExpectS3FileToExist(bucket, backendS3ObjectPath)
		})

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, backendS3ObjectPath)
		})

		It("destroys the env when action=destroy", func() {
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
					Action:  "destroy",
					Terraform: models.Terraform{
						Source: "fixtures/aws/",
						Vars: map[string]interface{}{
							"access_key":     accessKey,
							"secret_key":     secretKey,
							"bucket":         bucket,
							"object_key":     backendS3ObjectPath,
							"object_content": "terraform-is-neat",
							"region":         region,
						},
					},
				},
			}
			runner := out.Runner{
				SourceDir: workingDir,
				LogWriter: &logWriter,
				Namer:     &namer,
			}
			resp, err := runner.Run(req)
			Expect(err).ToNot(HaveOccurred(), "Logs: %s", logWriter.String())
			Expect(resp.Version.EnvName).To(Equal(envName))

			awsVerifier.ExpectS3FileToNotExist(bucket, backendS3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, backendStateFilePath)
		})
	})

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

	Context("when applying a plan", func() {
		BeforeEach(func() {
			err := helpers.DownloadStatefulPlugin(workingDir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("plan infrastructure and apply it", func() {
			planRequest := models.OutRequest{
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
						Source:   "fixtures/aws/",
						PlanOnly: true,
						Vars: map[string]interface{}{
							"access_key":     accessKey,
							"secret_key":     secretKey,
							"bucket":         bucket,
							"object_key":     s3ObjectPath,
							"object_content": "terraform-is-neat",
							"region":         region,
						},
						Env: map[string]string{
							"HOME": workingDir, // in prod plugin is installed system-wide
						},
					},
				},
			}
			applyRequest := models.OutRequest{
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
						Source:  "fixtures/aws/",
						PlanRun: true,
						Env: map[string]string{
							"HOME": workingDir, // in prod plugin is installed system-wide
						},
					},
				},
			}

			By("running 'out' to create the plan file")

			planrunner := out.Runner{
				SourceDir: workingDir,
				LogWriter: GinkgoWriter,
			}
			planOutput, err := planrunner.Run(planRequest)
			Expect(err).ToNot(HaveOccurred())

			By("ensuring that plan file exists")

			awsVerifier.ExpectS3FileToExist(
				bucket,
				planFilePath,
			)
			defer awsVerifier.DeleteObjectFromS3(bucket, planFilePath)

			Expect(planOutput.Version.EnvName).To(Equal(planRequest.Params.EnvName))
			Expect(planOutput.Version.PlanOnly).To(Equal("true"), "Expected PlanOnly to be true, but was false")
			Expect(planOutput.Version.Serial).To(BeEmpty())
			Expect(planOutput.Version.PlanChecksum).To(MatchRegexp("[0-9|a-f]+"))

			By("ensuring s3 file does not already exist")

			awsVerifier.ExpectS3FileToNotExist(
				bucket,
				s3ObjectPath,
			)

			By("applying the plan")

			applyrunner := out.Runner{
				SourceDir: workingDir,
				LogWriter: GinkgoWriter,
			}
			createOutput, err := applyrunner.Run(applyRequest)
			Expect(err).ToNot(HaveOccurred())

			Expect(createOutput.Version.PlanOnly).To(BeEmpty())
			Expect(createOutput.Version.Serial).ToNot(BeEmpty())
			Expect(createOutput.Version.PlanChecksum).To(BeEmpty())

			Expect(createOutput.Metadata).ToNot(BeEmpty())
			fields := map[string]interface{}{}
			for _, field := range createOutput.Metadata {
				fields[field.Name] = field.Value
			}
			Expect(fields["env_name"]).To(Equal(envName))
			expectedMD5 := fmt.Sprintf("%x", md5.Sum([]byte("terraform-is-neat")))
			Expect(fields["content_md5"]).To(Equal(expectedMD5))

			awsVerifier.ExpectS3FileToExist(
				bucket,
				s3ObjectPath,
			)

			By("ensuring that plan file no longer exists after the apply")

			awsVerifier.ExpectS3FileToNotExist(
				bucket,
				planFilePath,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("takes the existing statefile into account when generating a plan", func() {
			initialApplyRequest := models.OutRequest{
				Source: models.Source{
					Storage: storageModel,
				},
				Params: models.OutParams{
					EnvName: envName,
					Terraform: models.Terraform{
						Source: "fixtures/aws/",
						Env: map[string]string{
							"HOME": workingDir, // in prod plugin is installed system-wide
						},
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

			planRequest := models.OutRequest{
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
						Source:   "fixtures/aws/",
						PlanOnly: true,
						Env: map[string]string{
							"HOME": workingDir, // in prod plugin is installed system-wide
						},
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

			applyPlanRequest := models.OutRequest{
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
						Source:  "fixtures/aws/",
						PlanRun: true,
						Env: map[string]string{
							"HOME": workingDir, // in prod plugin is installed system-wide
						},
					},
				},
			}

			By("ensuring plan and state files does not already exist")

			awsVerifier.ExpectS3FileToNotExist(
				bucket,
				storageStateFilePath,
			)
			awsVerifier.ExpectS3FileToNotExist(
				bucket,
				backendStateFilePath,
			)
			awsVerifier.ExpectS3FileToNotExist(
				bucket,
				planFilePath,
			)

			By("running 'out' to create the legacy statefile")

			runner := out.Runner{
				SourceDir: workingDir,
				LogWriter: GinkgoWriter,
			}
			_, err := runner.Run(initialApplyRequest)
			Expect(err).ToNot(HaveOccurred())

			By("ensuring that legacy statefile exists and backend/plan do not")

			awsVerifier.ExpectS3FileToExist(
				bucket,
				storageStateFilePath,
			)
			awsVerifier.ExpectS3FileToNotExist(
				bucket,
				backendStateFilePath,
			)
			awsVerifier.ExpectS3FileToNotExist(
				bucket,
				planFilePath,
			)

			initialLastModified := awsVerifier.GetLastModifiedFromS3(bucket, s3ObjectPath)

			time.Sleep(1 * time.Second) // ensure LastModified has time to change

			By("creating the plan")

			_, err = runner.Run(planRequest)
			Expect(err).ToNot(HaveOccurred())

			awsVerifier.ExpectS3FileToExist(
				bucket,
				s3ObjectPath,
			)

			By("ensuring that state and plan files exist")

			awsVerifier.ExpectS3FileToNotExist(
				bucket,
				storageStateFilePath,
			)
			awsVerifier.ExpectS3FileToExist(
				bucket,
				fmt.Sprintf("%s.migrated", storageStateFilePath),
			)
			awsVerifier.ExpectS3FileToExist(
				bucket,
				backendStateFilePath,
			)
			awsVerifier.ExpectS3FileToExist(
				bucket,
				planFilePath,
			)

			By("applying the plan")

			_, err = runner.Run(applyPlanRequest)
			Expect(err).ToNot(HaveOccurred())

			By("ensuring that existing statefile was used and S3 Object was unchanged")
			finalLastModified := awsVerifier.GetLastModifiedFromS3(bucket, s3ObjectPath)
			Expect(finalLastModified).To(Equal(initialLastModified))
		})

		It("plan should be deleted on destroy", func() {
			planOutRequest := models.OutRequest{
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
						Source:   "fixtures/aws/",
						PlanOnly: true,
						Env: map[string]string{
							"HOME": workingDir, // in prod plugin is installed system-wide
						},
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

			By("ensuring state and plan file does not already exist")

			awsVerifier.ExpectS3FileToNotExist(
				bucket,
				planFilePath,
			)

			By("running 'out' to create the plan file")

			planrunner := out.Runner{
				SourceDir: workingDir,
				LogWriter: GinkgoWriter,
			}
			_, err := planrunner.Run(planOutRequest)
			Expect(err).ToNot(HaveOccurred())

			By("ensuring that plan file exists with valid version (LastModified)")

			awsVerifier.ExpectS3FileToExist(
				bucket,
				planFilePath,
			)

			By("running 'out' to delete the plan file")

			planOutRequest.Params.Terraform.PlanOnly = false
			planOutRequest.Params.Action = models.DestroyAction
			_, err = planrunner.Run(planOutRequest)
			Expect(err).ToNot(HaveOccurred())

			By("ensuring that plan file no longer exists")

			awsVerifier.ExpectS3FileToNotExist(
				bucket,
				planFilePath,
			)
		})

		It("overrides the existing resource definition when generating a plan", func() {
			planRequest := models.OutRequest{
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
						PlanOnly: true,
						OverrideFiles: []string{
							"fixtures/override/example_override.tf",
						},
						Source: "fixtures/aws/",
						Vars: map[string]interface{}{
							"access_key":     accessKey,
							"secret_key":     secretKey,
							"bucket":         bucket,
							"object_key":     s3ObjectPath,
							"object_content": "terraform-is-neat",
							"region":         region,
						},
						Env: map[string]string{
							"HOME": workingDir, // in prod plugin is installed system-wide
						},
					},
				},
			}

			applyPlanRequest := models.OutRequest{
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
						Source:  "fixtures/aws/",
						PlanRun: true,
						Env: map[string]string{
							"HOME": workingDir, // in prod plugin is installed system-wide
						},
					},
				},
			}

			runner := out.Runner{
				SourceDir: workingDir,
				LogWriter: GinkgoWriter,
			}

			_, err := runner.Run(planRequest)
			Expect(err).ToNot(HaveOccurred())

			output, err := runner.Run(applyPlanRequest)
			Expect(err).ToNot(HaveOccurred())

			Expect(output.Metadata).ToNot(BeEmpty())
			fields := map[string]interface{}{}
			for _, field := range output.Metadata {
				fields[field.Name] = field.Value
			}
			Expect(fields["env_name"]).To(Equal(envName))
			Expect(fields["object_content"]).To(Equal("OVERRIDE"))
			expectedMD5 := fmt.Sprintf("%x", md5.Sum([]byte("OVERRIDE")))
			Expect(fields["content_md5"]).To(Equal(expectedMD5))

			awsVerifier.ExpectS3FileToExist(bucket, s3ObjectPath)
		})

	})

	It("overrides the existing resource definition", func() {
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
					OverrideFiles: []string{
						"fixtures/override/example_override.tf",
					},
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
			LogWriter: GinkgoWriter,
		}

		output, err := runner.Run(req)
		Expect(err).ToNot(HaveOccurred())

		Expect(output.Metadata).ToNot(BeEmpty())
		fields := map[string]interface{}{}
		for _, field := range output.Metadata {
			fields[field.Name] = field.Value
		}
		Expect(fields["env_name"]).To(Equal(envName))
		Expect(fields["object_content"]).To(Equal("OVERRIDE"))
		expectedMD5 := fmt.Sprintf("%x", md5.Sum([]byte("OVERRIDE")))
		Expect(fields["content_md5"]).To(Equal(expectedMD5))

		awsVerifier.ExpectS3FileToExist(bucket, s3ObjectPath)
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
