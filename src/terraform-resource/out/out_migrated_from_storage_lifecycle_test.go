package out_test

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"terraform-resource/models"
	"terraform-resource/out"
	"terraform-resource/storage"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out MigratedFromStorage Lifecycle", func() {

	var (
		envName         string
		stateFilePath   string
		s3ObjectPath    string
		workingDir      string
		workspacePath   string
		resetWorkingDir func()
	)

	BeforeEach(func() {
		envName = helpers.RandomString("out-backend-test")

		workspacePath = helpers.RandomString("out-backend-test")

		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-lifecycle"))
		stateFilePath = path.Join(workspacePath, envName, "terraform.tfstate")

		resetWorkingDir()
	})

	AfterEach(func() {
		_ = os.RemoveAll(workingDir)
		awsVerifier.DeleteObjectFromS3(bucket, s3ObjectPath)
		awsVerifier.DeleteObjectFromS3(bucket, stateFilePath)
	})

	It("creates, updates, and deletes infrastructure", func() {
		outRequest := models.OutRequest{
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
			bucket,
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
			bucket,
			s3ObjectPath,
		)

		Expect(createOutput.Version.Serial).ToNot(Equal(0))

		By("ensuring that state file exists")

		awsVerifier.ExpectS3FileToExist(
			bucket,
			stateFilePath,
		)
		lastModified := awsVerifier.GetLastModifiedFromS3(
			bucket,
			stateFilePath,
		)
		stateFileCreatedTime, err := time.Parse(storage.TimeFormat, lastModified)
		Expect(err).ToNot(HaveOccurred())
		time.Sleep(1 * time.Second) // ensure LastModified changes

		Expect(createOutput.Version.EnvName).To(Equal(outRequest.Params.EnvName))

		By("running 'out' to update the S3 file")

		resetWorkingDir()

		outRequest.Params.Terraform.Vars["object_content"] = "terraform-is-still-neat"
		updateOutput, err := runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred())

		Expect(updateOutput.Metadata).ToNot(BeEmpty())
		fields = map[string]interface{}{}
		for _, field := range updateOutput.Metadata {
			fields[field.Name] = field.Value
		}
		expectedMD5 = fmt.Sprintf("%x", md5.Sum([]byte("terraform-is-still-neat")))
		Expect(fields["content_md5"]).To(Equal(expectedMD5))

		createSerial, err := strconv.Atoi(createOutput.Version.Serial)
		Expect(err).ToNot(HaveOccurred())
		updateSerial, err := strconv.Atoi(updateOutput.Version.Serial)
		Expect(err).ToNot(HaveOccurred())
		Expect(updateSerial).To(BeNumerically(">", createSerial))

		By("ensuring that state file has been updated")

		awsVerifier.ExpectS3FileToExist(
			bucket,
			stateFilePath,
		)

		lastModified = awsVerifier.GetLastModifiedFromS3(
			bucket,
			stateFilePath,
		)
		stateFileUpdatedTime, err := time.Parse(storage.TimeFormat, lastModified)
		Expect(err).ToNot(HaveOccurred())
		Expect(stateFileUpdatedTime).To(BeTemporally(">", stateFileCreatedTime))
		time.Sleep(1 * time.Second) // ensure LastModified changes

		Expect(updateOutput.Version.EnvName).To(Equal(outRequest.Params.EnvName))

		By("running 'out' to delete the S3 file")

		resetWorkingDir()

		outRequest.Params.Action = models.DestroyAction
		deleteOutput, err := runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred())

		awsVerifier.ExpectS3FileToNotExist(
			bucket,
			s3ObjectPath,
		)

		By("ensuring that state file no longer exists")

		awsVerifier.ExpectS3FileToNotExist(
			bucket,
			stateFilePath,
		)

		Expect(deleteOutput.Version.EnvName).To(Equal(outRequest.Params.EnvName))
	})

	resetWorkingDir = func() {
		err := os.RemoveAll(workingDir)
		Expect(err).ToNot(HaveOccurred())

		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-test")
		Expect(err).ToNot(HaveOccurred())

		// ensure relative paths resolve correctly
		err = os.Chdir(workingDir)
		Expect(err).ToNot(HaveOccurred())

		fixturesDir := path.Join(helpers.ProjectRoot(), "fixtures")
		err = exec.Command("cp", "-r", fixturesDir, workingDir).Run()
		Expect(err).ToNot(HaveOccurred())
	}
})
