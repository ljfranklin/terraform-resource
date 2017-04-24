package out_test

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"terraform-resource/models"
	"terraform-resource/namer/namerfakes"
	"terraform-resource/out"
	"terraform-resource/storage"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var workingDir string

var _ = Describe("Out", func() {

	var (
		storageModel      storage.Model
		envName           string
		stateFilePath     string
		s3ObjectPath      string
		workingDir        string
		fixtureEnvName    string
		pathToS3Fixture   string
		namer             namerfakes.FakeNamer
		assertOutBehavior func(models.OutRequest, map[string]string)
		createYAMLTmpFile func(string, interface{}) string
		logWriter         bytes.Buffer
	)

	BeforeEach(func() {
		region := os.Getenv("AWS_REGION") // optional
		if region == "" {
			region = "us-east-1"
		}
		envName = helpers.RandomString("out-test")
		stateFilePath = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", envName))
		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-test"))

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

		fixtureEnvName = helpers.RandomString("s3-test-fixture")
		pathToS3Fixture = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", fixtureEnvName))

		namer = namerfakes.FakeNamer{}
	})

	AfterEach(func() {
		_ = os.RemoveAll(workingDir)
		awsVerifier.DeleteObjectFromS3(bucket, s3ObjectPath)
		awsVerifier.DeleteObjectFromS3(bucket, stateFilePath)
	})

	It("creates IaaS resources from a local terraform source", func() {
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
	})

	It("creates IaaS resources from a local terraform source using modules", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
			},
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					Source: "fixtures/module/",
					Vars: map[string]interface{}{
						"access_key":     accessKey,
						"secret_key":     secretKey,
						"bucket":         bucket,
						"object_key":     s3ObjectPath,
						"object_content": "terraform-is-neat",
						"region":         region,
					},
				},
				OutputModule: "test_module_1",
			},
		}
		expectedMetadata := map[string]string{
			"env_name":    envName,
			"content_md5": calculateMD5("terraform-is-neat"),
		}

		assertOutBehavior(req, expectedMetadata)

		awsVerifier.ExpectS3FileToExist(bucket, s3ObjectPath)
	})

	It("creates IaaS resources from a remote terraform source", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
			},
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					// Note: changes to fixture must be pushed before running this test
					Source: "github.com/ljfranklin/terraform-resource//fixtures/aws/",
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
	})

	It("creates IaaS resources from a terraform module", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
			},
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					Source: "fixtures/module/",
					Vars: map[string]interface{}{
						"access_key":     accessKey,
						"secret_key":     secretKey,
						"bucket":         bucket,
						"object_key":     s3ObjectPath,
						"object_content": "terraform-is-neat",
						"region":         region,
					},
				},
				OutputModule: "test_module_1",
			},
		}
		expectedMetadata := map[string]string{
			"env_name":    envName,
			"content_md5": calculateMD5("terraform-is-neat"),
		}

		assertOutBehavior(req, expectedMetadata)
	})

	It("creates IaaS resources from `source.terraform.vars`", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
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
			Params: models.OutParams{
				EnvName: envName,
			},
		}
		expectedMetadata := map[string]string{
			"env_name":    envName,
			"content_md5": calculateMD5("terraform-is-neat"),
		}

		assertOutBehavior(req, expectedMetadata)
	})

	It("creates IaaS resources from `source.terraform` and `put.params.terraform`", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
				Terraform: models.Terraform{
					Source: "fixtures/aws/",
					Vars: map[string]interface{}{
						"access_key": accessKey,
						"secret_key": "bad-secret-key", // will be overridden
						"region":     region,
					},
				},
			},
			// put params take precedence
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					Vars: map[string]interface{}{
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
	})

	Context("when given a yaml file containing variables", func() {
		var firstVarFile string
		var secondVarFile string

		BeforeEach(func() {
			firstVarFile = createYAMLTmpFile("tf-vars-1", map[string]interface{}{
				"bucket": bucket,
			})
			secondVarFile = createYAMLTmpFile("tf-vars-2", map[string]interface{}{
				"object_key":     s3ObjectPath,
				"object_content": "terraform-files-are-neat",
			})
		})

		It("creates IaaS resources from request vars and file vars", func() {
			req := models.OutRequest{
				Source: models.Source{
					Storage: storageModel,
					Terraform: models.Terraform{
						Source: "fixtures/aws/",
						Vars: map[string]interface{}{
							"access_key": accessKey,
							// will be overridden
							"secret_key": "bad-secret-key",
							"region":     region,
						},
					},
				},
				// put params overrides source
				Params: models.OutParams{
					EnvName: envName,
					Terraform: models.Terraform{
						Vars: map[string]interface{}{
							"secret_key": secretKey,
							// will be overridden
							"object_content": "to-be-overridden",
							"region":         region,
						},
						// var files overrides put.params
						VarFiles: []string{firstVarFile, secondVarFile},
					},
				},
			}
			expectedMetadata := map[string]string{
				"env_name":    envName,
				"content_md5": calculateMD5("terraform-files-are-neat"),
			}

			assertOutBehavior(req, expectedMetadata)
		})

		It("prints a deprecation warning if VarFile is used", func() {
			deadline := time.Date(2017, time.June, 1, 0, 0, 0, 0, time.Local)
			if time.Now().After(deadline) {
				Fail("Time to remove support for `var_file`!")
			}

			req := models.OutRequest{
				Source: models.Source{
					Storage: storageModel,
					Terraform: models.Terraform{
						Source: "fixtures/aws/",
						Vars: map[string]interface{}{
							"access_key": accessKey,
							"bucket":     bucket,
							// will be overridden
							"secret_key": "bad-secret-key",
							"region":     region,
						},
					},
				},
				// put params overrides source
				Params: models.OutParams{
					EnvName: envName,
					Terraform: models.Terraform{
						Vars: map[string]interface{}{
							"secret_key": secretKey,
							// will be overridden
							"object_content": "to-be-overridden",
							"region":         region,
						},
						// var files overrides put.params
						VarFile: secondVarFile,
					},
				},
			}
			expectedMetadata := map[string]string{
				"env_name":    envName,
				"content_md5": calculateMD5("terraform-files-are-neat"),
			}

			assertOutBehavior(req, expectedMetadata)

			Expect(logWriter.String()).To(MatchRegexp("WARNING.*var_file.*June 1"))
		})
	})

	It("sets env variables from `source.terraform` and `put.params.terraform`", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
				Terraform: models.Terraform{
					Source: "fixtures/aws-env/",
					Env: map[string]string{
						"AWS_ACCESS_KEY_ID":     accessKey,
						"AWS_SECRET_ACCESS_KEY": "bad-secret-key", // will be overridden
					},
				},
			},
			// put params take precedence
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					Env: map[string]string{
						"AWS_SECRET_ACCESS_KEY": secretKey,
						"TF_VAR_region":         region, // also supports TF_VAR_ style
					},
					Vars: map[string]interface{}{
						"bucket":         bucket,
						"object_key":     s3ObjectPath,
						"object_content": "terraform-is-neat",
					},
				},
			},
		}
		expectedMetadata := map[string]string{
			"env_name":    envName,
			"content_md5": calculateMD5("terraform-is-neat"),
		}

		assertOutBehavior(req, expectedMetadata)
	})

	It("allows hashes and lists in metadata", func() {
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
						"object_key":     s3ObjectPath,
						"object_content": "terraform-is-neat",
						"region":         region,
					},
				},
			},
		}
		expectedMetadata := map[string]string{
			"map":  `{"key-1":"value-1","key-2":"value-2"}`,
			"list": `["item-1","item-2"]`,
		}

		assertOutBehavior(req, expectedMetadata)
	})

	It("allows an empty set of outputs", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
			},
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					Source: "fixtures/no-outputs/",
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
		expectedMetadata := map[string]string{}

		assertOutBehavior(req, expectedMetadata)
	})

	It("redacts sensitive outputs in metadata and logs", func() {
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
						"object_key":     s3ObjectPath,
						"object_content": "terraform-is-neat",
						"region":         region,
					},
				},
			},
		}
		expectedMetadata := map[string]string{
			"secret": `<sensitive>`,
		}

		assertOutBehavior(req, expectedMetadata)

		Expect(logWriter.String()).ToNot(ContainSubstring("super-secret"))
	})

	It("returns an error if an input var is malformed", func() {
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
						"bucket":         nil,
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
		_, err := runner.Run(req)
		Expect(err).To(HaveOccurred())
		Expect(logWriter.String()).To(ContainSubstring("bucket"))
		Expect(logWriter.String()).To(ContainSubstring("null"))
	})

	It("replaces spaces in env_name with hyphens", func() {
		spaceName := strings.Replace(envName, "-", " ", -1)
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
			},
			Params: models.OutParams{
				EnvName: spaceName,
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
	})

	It("trims whitespace from env_name", func() {
		spaceName := fmt.Sprintf(" %s \n", envName)
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
			},
			Params: models.OutParams{
				EnvName: spaceName,
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
	})

	Context("when env_name_file is specified", func() {
		var (
			envNameFile string
		)

		BeforeEach(func() {
			tmpFile, err := ioutil.TempFile(workingDir, "env-file")
			Expect(err).ToNot(HaveOccurred())
			_, err = tmpFile.WriteString(envName)
			Expect(err).ToNot(HaveOccurred())
			envNameFile = tmpFile.Name()
		})

		AfterEach(func() {
			_ = os.RemoveAll(envNameFile)
		})

		It("Allows env name to be specified via env_name_file", func() {
			req := models.OutRequest{
				Source: models.Source{
					Storage: storageModel,
				},
				Params: models.OutParams{
					EnvNameFile: envNameFile,
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
				"env_name": envName,
			}

			assertOutBehavior(req, expectedMetadata)
		})
	})

	It("creates an env with a random name when generate_random_name is true", func() {
		namer.RandomNameReturns(envName)

		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
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
		expectedMetadata := map[string]string{
			"env_name": envName,
		}

		assertOutBehavior(req, expectedMetadata)

		Expect(namer.RandomNameCallCount()).To(Equal(1), "Expected RandomName to be called once")
	})

	It("encrypts the state file when server_side_encryption is given", func() {
		storageModel.ServerSideEncryption = "AES256"
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

		awsVerifier.ExpectS3ServerSideEncryption(bucket, stateFilePath, "AES256")
	})

	It("encrypts the state file with a key ID when sse_kms_key_id is given", func() {
		if kmsKeyID == "" {
			Skip("S3_KMS_KEY_ID is not set, skipping sse_kms_key_id test...")
		}

		storageModel.SSEKMSKeyId = kmsKeyID
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

		awsVerifier.ExpectS3ServerSideEncryption(bucket, stateFilePath, "aws:kms", kmsKeyID)
	})

	Context("when bucket contains a state file", func() {
		BeforeEach(func() {
			currFixture, err := os.Open(helpers.FileLocation("fixtures/s3/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToS3Fixture, currFixture)
		})

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, pathToS3Fixture)
		})

		It("returns an error if random name clashes", func() {
			// pick a name that always clashes
			namer.RandomNameReturns(fixtureEnvName)

			req := models.OutRequest{
				Source: models.Source{
					Storage: storageModel,
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
			_, err := runner.Run(req)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("random name"))
			Expect(namer.RandomNameCallCount()).To(Equal(out.NameClashRetries),
				"Expected RandomName to be called %d times", out.NameClashRetries)
		})
	})

	Context("given an invalid terraform var", func() {
		var req models.OutRequest
		BeforeEach(func() {
			req = models.OutRequest{
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
							"object_key":           s3ObjectPath,
							"object_content":       "terraform-is-neat",
							"invalid_object_count": "1",
							"region":               region,
						},
					},
				},
			}
		})

		It("does not delete partially created resources by default", func() {
			runner := out.Runner{
				SourceDir: workingDir,
				LogWriter: &logWriter,
			}
			_, err := runner.Run(req)

			Expect(err).To(HaveOccurred())
			Expect(logWriter.String()).To(ContainSubstring("invalid_object"))
			awsVerifier.ExpectS3FileToExist(bucket, s3ObjectPath)

			originalStateFilePath := stateFilePath
			stateFilePath = path.Join(bucketPath, fmt.Sprintf("%s.tfstate.tainted", envName))
			awsVerifier.ExpectS3FileToNotExist(bucket, originalStateFilePath)
			awsVerifier.ExpectS3FileToExist(bucket, stateFilePath)

			// cleanup
			req.Params.Action = models.DestroyAction
			_, err = runner.Run(req)
			Expect(err).ToNot(HaveOccurred())
			awsVerifier.ExpectS3FileToNotExist(bucket, s3ObjectPath)
			awsVerifier.ExpectS3FileToNotExist(bucket, originalStateFilePath)
			awsVerifier.ExpectS3FileToNotExist(bucket, stateFilePath)
		})

		It("untaints the state file after the next successful put", func() {
			runner := out.Runner{
				SourceDir: workingDir,
				LogWriter: &logWriter,
			}
			_, err := runner.Run(req)
			Expect(err).To(HaveOccurred())
			Expect(logWriter.String()).To(ContainSubstring("invalid_object"))

			taintedStateFilePath := path.Join(bucketPath, fmt.Sprintf("%s.tfstate.tainted", envName))
			awsVerifier.ExpectS3FileToNotExist(bucket, stateFilePath)
			awsVerifier.ExpectS3FileToExist(bucket, taintedStateFilePath)

			req.Params.Terraform.Vars["invalid_object_count"] = "0"
			_, err = runner.Run(req)
			Expect(err).ToNot(HaveOccurred())
			awsVerifier.ExpectS3FileToExist(bucket, s3ObjectPath)
			awsVerifier.ExpectS3FileToExist(bucket, stateFilePath)
			awsVerifier.ExpectS3FileToNotExist(bucket, taintedStateFilePath)
		})

		It("deletes all resources on failure if delete_on_failure is true", func() {
			req.Params.Terraform.DeleteOnFailure = true

			runner := out.Runner{
				SourceDir: workingDir,
				LogWriter: &logWriter,
			}
			_, err := runner.Run(req)

			Expect(err).To(HaveOccurred())
			Expect(logWriter.String()).To(ContainSubstring("invalid_object"))
			awsVerifier.ExpectS3FileToNotExist(bucket, s3ObjectPath)

			originalStateFilePath := stateFilePath
			stateFilePath = path.Join(bucketPath, fmt.Sprintf("%s.tfstate.tainted", envName))
			awsVerifier.ExpectS3FileToNotExist(bucket, originalStateFilePath)
			awsVerifier.ExpectS3FileToNotExist(bucket, stateFilePath)
		})
	})

	Context("when an s3 compatible storage is used", func() {
		var s3Verifier *helpers.AWSVerifier

		BeforeEach(func() {
			storageModel = storage.Model{
				Endpoint:        s3CompatibleEndpoint,
				Bucket:          s3CompatibleBucket,
				BucketPath:      bucketPath,
				AccessKeyID:     s3CompatibleAccessKey,
				SecretAccessKey: s3CompatibleSecretKey,
				RegionName:      region,
			}

			s3Verifier = helpers.NewAWSVerifier(
				s3CompatibleAccessKey,
				s3CompatibleSecretKey,
				region,
				s3CompatibleEndpoint,
			)
		})

		AfterEach(func() {
			s3Verifier.DeleteObjectFromS3(s3CompatibleBucket, stateFilePath)
		})

		It("stores the state file successfully", func() {
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

			s3Verifier.ExpectS3FileToExist(s3CompatibleBucket, stateFilePath)
		})
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

	createYAMLTmpFile = func(filePrefix string, content interface{}) string {
		fileContent, err := yaml.Marshal(content)
		Expect(err).ToNot(HaveOccurred())

		varFileName := fmt.Sprintf("%s.yml", helpers.RandomString(filePrefix))
		varFilePath := path.Join(workingDir, varFileName)
		varFile, err := os.Create(varFilePath)
		Expect(err).ToNot(HaveOccurred())
		defer varFile.Close()

		_, err = varFile.Write(fileContent)
		Expect(err).ToNot(HaveOccurred())

		return varFileName
	}
})

func calculateMD5(content string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(content)))
}
