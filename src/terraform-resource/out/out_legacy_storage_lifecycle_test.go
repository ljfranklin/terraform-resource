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

	"terraform-resource/models"
	"terraform-resource/out"
	"terraform-resource/storage"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out Legacy Storage Lifecycle", func() {

	var (
		envName       string
		stateFilePath string
		planFilePath  string
		s3ObjectPath  string
		workingDir    string
	)

	BeforeEach(func() {
		envName = helpers.RandomString("out-legacy-test")
		stateFilePath = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", envName))
		planFilePath = path.Join(bucketPath, fmt.Sprintf("%s.plan", envName))
		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-lifecycle"))

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-test")
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

	It("creates, updates, and deletes infrastructure", func() {
		outRequest := models.OutRequest{
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

		By("ensuring state file does not already exist")

		awsVerifier.ExpectS3FileToNotExist(
			outRequest.Source.Storage.Bucket,
			stateFilePath,
		)

		By("running 'out' to create the S3 file")

		runner := out.Runner{
			SourceDir: workingDir,
			LogWriter: GinkgoWriter,
		}
		createOutput, err := runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred())

		Expect(createOutput.Metadata).ToNot(BeEmpty())
		fields := map[string]interface{}{}
		for _, field := range createOutput.Metadata {
			fields[field.Name] = field.Value
		}
		Expect(fields["env_name"]).To(Equal(envName))
		expectedMD5 := fmt.Sprintf("%x", md5.Sum([]byte("terraform-is-neat")))
		Expect(fields["content_md5"]).To(Equal(expectedMD5))

		awsVerifier.ExpectS3FileToExist(
			outRequest.Source.Storage.Bucket,
			s3ObjectPath,
		)

		By("ensuring that state file exists with valid version (LastModified)")

		awsVerifier.ExpectS3FileToExist(
			outRequest.Source.Storage.Bucket,
			stateFilePath,
		)

		createVersion, err := time.Parse(storage.TimeFormat, createOutput.Version.LastModified)
		Expect(err).ToNot(HaveOccurred())
		Expect(createOutput.Version.EnvName).To(Equal(outRequest.Params.EnvName))

		time.Sleep(1 * time.Second) // ensure LastModified changes

		By("running 'out' to update the S3 file")

		// omits some fields to ensure the resource feeds previous output back in as input
		outRequest.Params.Terraform.Vars = map[string]interface{}{
			"access_key":     accessKey,
			"secret_key":     secretKey,
			"object_content": "terraform-is-still-neat",
			"region":         region,
		}

		updateOutput, err := runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred())

		Expect(updateOutput.Metadata).ToNot(BeEmpty())
		fields = map[string]interface{}{}
		for _, field := range updateOutput.Metadata {
			fields[field.Name] = field.Value
		}
		expectedMD5 = fmt.Sprintf("%x", md5.Sum([]byte("terraform-is-still-neat")))
		Expect(fields["content_md5"]).To(Equal(expectedMD5))

		By("ensuring that state file has been updated")

		awsVerifier.ExpectS3FileToExist(
			outRequest.Source.Storage.Bucket,
			stateFilePath,
		)

		updatedVersion, err := time.Parse(storage.TimeFormat, updateOutput.Version.LastModified)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedVersion).To(BeTemporally(">", createVersion))
		Expect(updateOutput.Version.EnvName).To(Equal(outRequest.Params.EnvName))

		time.Sleep(1 * time.Second) // ensure LastModified changes

		By("running 'out' to delete the S3 file")

		outRequest.Params.Action = models.DestroyAction
		deleteOutput, err := runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred())

		awsVerifier.ExpectS3FileToNotExist(
			outRequest.Source.Storage.Bucket,
			s3ObjectPath,
		)

		By("ensuring that state file no longer exists")

		awsVerifier.ExpectS3FileToNotExist(
			outRequest.Source.Storage.Bucket,
			stateFilePath,
		)

		deletedVersion, err := time.Parse(storage.TimeFormat, deleteOutput.Version.LastModified)
		Expect(err).ToNot(HaveOccurred())
		Expect(deletedVersion).To(BeTemporally(">", updatedVersion))
		Expect(deleteOutput.Version.EnvName).To(Equal(outRequest.Params.EnvName))
	})

	It("can delete after a failed put", func() {
		outRequest := models.OutRequest{
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

		By("running 'out' to create the tainted S3 file")

		logWriter := bytes.Buffer{}
		runner := out.Runner{
			SourceDir: workingDir,
			LogWriter: &logWriter,
		}
		_, err := runner.Run(outRequest)
		Expect(err).To(HaveOccurred())
		Expect(logWriter.String()).To(ContainSubstring("invalid_object"))

		By("ensuring that tainted state file exists")

		awsVerifier.ExpectS3FileToExist(
			outRequest.Source.Storage.Bucket,
			fmt.Sprintf("%s.tainted", stateFilePath),
		)

		By("running 'out' to delete the environment and state file")

		outRequest.Params.Action = models.DestroyAction
		deleteOutput, err := runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred())

		awsVerifier.ExpectS3FileToNotExist(
			outRequest.Source.Storage.Bucket,
			stateFilePath,
		)

		Expect(deleteOutput.Version.EnvName).To(Equal(outRequest.Params.EnvName))
	})
})
