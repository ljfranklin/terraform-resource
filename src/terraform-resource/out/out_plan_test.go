package out_test

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"terraform-resource/models"
	"terraform-resource/out"
	"terraform-resource/storage"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out Plan", func() {

	var (
		envName       string
		stateFilePath string
		planFilePath  string
		s3ObjectPath  string
		workingDir    string
	)

	BeforeEach(func() {
		envName = helpers.RandomString("out-test")
		stateFilePath = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", envName))
		planFilePath = path.Join(bucketPath, fmt.Sprintf("%s.plan", envName))
		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-plan"))

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-plan-test")
		Expect(err).ToNot(HaveOccurred())

		// ensure relative paths resolve correctly
		err = os.Chdir(workingDir)
		Expect(err).ToNot(HaveOccurred())

		fixturesDir := path.Join(helpers.ProjectRoot(), "fixtures")
		err = exec.Command("cp", "-r", fixturesDir, workingDir).Run()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(workingDir)
		awsVerifier.DeleteObjectFromS3(bucket, s3ObjectPath)
		awsVerifier.DeleteObjectFromS3(bucket, stateFilePath)
		awsVerifier.DeleteObjectFromS3(bucket, planFilePath)
	})

	It("plan infrastructure and apply it", func() {
		planOutRequest := models.OutRequest{
			Source: models.Source{
				Storage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					RegionName:      region,
				},
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
				},
			},
		}

		applyRequest := models.OutRequest{
			Source: models.Source{
				Storage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					RegionName:      region,
				},
			},
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					PlanRun: true,
				},
			},
		}

		By("ensuring plan file does not already exist")

		awsVerifier.ExpectS3FileToNotExist(
			planOutRequest.Source.Storage.Bucket,
			planFilePath,
		)

		By("running 'out' to create the plan file")

		planrunner := out.Runner{
			SourceDir: workingDir,
			LogWriter: GinkgoWriter,
		}
		planOutput, err := planrunner.Run(planOutRequest)
		Expect(err).ToNot(HaveOccurred())

		By("ensuring that plan file exists with valid version (LastModified)")

		awsVerifier.ExpectS3FileToExist(
			planOutRequest.Source.Storage.Bucket,
			planFilePath,
		)

		_, err = time.Parse(storage.TimeFormat, planOutput.Version.LastModified)
		Expect(err).ToNot(HaveOccurred())
		Expect(planOutput.Version.EnvName).To(Equal(planOutRequest.Params.EnvName))
		Expect(planOutput.Version.PlanOnly).To(BeTrue(), "Expected PlanOnly to be True, but was False")

		time.Sleep(1 * time.Second) // ensure LastModified changes

		By("ensuring state file does not already exist")

		awsVerifier.ExpectS3FileToNotExist(
			applyRequest.Source.Storage.Bucket,
			stateFilePath,
		)

		By("applying the plan")

		applyrunner := out.Runner{
			SourceDir: workingDir,
			LogWriter: GinkgoWriter,
		}
		createOutput, err := applyrunner.Run(applyRequest)
		Expect(err).ToNot(HaveOccurred())

		Expect(createOutput.Version.PlanOnly).To(BeFalse(), "Expected PlanOnly to be False, but was True")

		Expect(createOutput.Metadata).ToNot(BeEmpty())
		fields := map[string]interface{}{}
		for _, field := range createOutput.Metadata {
			fields[field.Name] = field.Value
		}
		Expect(fields["env_name"]).To(Equal(envName))
		expectedMD5 := fmt.Sprintf("%x", md5.Sum([]byte("terraform-is-neat")))
		Expect(fields["content_md5"]).To(Equal(expectedMD5))

		awsVerifier.ExpectS3FileToExist(
			applyRequest.Source.Storage.Bucket,
			s3ObjectPath,
		)

		By("ensuring that plan file no longer exists after the apply")

		awsVerifier.ExpectS3FileToNotExist(
			applyRequest.Source.Storage.Bucket,
			planFilePath,
		)
		Expect(err).ToNot(HaveOccurred())

	})

	It("takes the existing statefile into account when generating a plan", func() {
		initialApplyRequest := models.OutRequest{
			Source: models.Source{
				Storage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					RegionName:      region,
				},
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

		planRequest := models.OutRequest{
			Source: models.Source{
				Storage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					RegionName:      region,
				},
			},
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					PlanOnly: true,
					Source:   "fixtures/aws/",
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
				Storage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					RegionName:      region,
				},
			},
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					PlanRun: true,
				},
			},
		}

		By("ensuring plan and state files does not already exist")

		awsVerifier.ExpectS3FileToNotExist(
			initialApplyRequest.Source.Storage.Bucket,
			stateFilePath,
		)
		awsVerifier.ExpectS3FileToNotExist(
			initialApplyRequest.Source.Storage.Bucket,
			planFilePath,
		)

		By("running 'out' to create the statefile")

		runner := out.Runner{
			SourceDir: workingDir,
			LogWriter: GinkgoWriter,
		}
		_, err := runner.Run(initialApplyRequest)
		Expect(err).ToNot(HaveOccurred())

		By("ensuring that statefile exists and plan does not")

		awsVerifier.ExpectS3FileToExist(
			bucket,
			stateFilePath,
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

		awsVerifier.ExpectS3FileToExist(
			bucket,
			stateFilePath,
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
				Storage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					RegionName:      region,
				},
			},
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					PlanOnly: true,
					Source:   "fixtures/aws/",
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
			planOutRequest.Source.Storage.Bucket,
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
			planOutRequest.Source.Storage.Bucket,
			planFilePath,
		)

		By("running 'out' to delete the plan file")

		planOutRequest.Params.Terraform.PlanOnly = false
		planOutRequest.Params.Action = models.DestroyAction
		_, err = planrunner.Run(planOutRequest)
		Expect(err).ToNot(HaveOccurred())

		By("ensuring that plan file no longer exists")

		awsVerifier.ExpectS3FileToNotExist(
			planOutRequest.Source.Storage.Bucket,
			planFilePath,
		)
	})
})
