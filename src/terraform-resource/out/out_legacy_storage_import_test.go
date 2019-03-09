package out_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"terraform-resource/models"
	"terraform-resource/out"
	"terraform-resource/storage"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out Legacy Storage Import", func() {

	var (
		envName       string
		stateFilePath string
		s3ObjectPath  string
		workingDir    string
	)

	BeforeEach(func() {
		envName = helpers.RandomString("out-test")
		stateFilePath = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", envName))
		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-import"))

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-import-test")
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
	})

	It("imports the existing resource and applys it", func() {
		awsVerifier.ExpectS3BucketToExist(bucket)

		importsFilePath := path.Join(workingDir, "imports")
		importsFileContents := fmt.Sprintf("aws_s3_bucket.bucket: %s", bucket)
		err := ioutil.WriteFile(importsFilePath, []byte(importsFileContents), 0700)
		Expect(err).ToNot(HaveOccurred())

		By("Ensuring that an existing bucket is imported prior to an apply")

		importRequest := models.OutRequest{
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
					ImportFiles: []string{
						importsFilePath,
					},
					Source: "fixtures/import/",
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
		_, err = runner.Run(importRequest)
		Expect(err).ToNot(HaveOccurred())

		awsVerifier.ExpectS3BucketToExist(bucket)
		awsVerifier.ExpectS3FileToExist(
			bucket,
			s3ObjectPath,
		)

		By("Running again to ensure imports are merged with existing state file")

		_, err = runner.Run(importRequest)
		Expect(err).ToNot(HaveOccurred())

		awsVerifier.ExpectS3BucketToExist(bucket)
		awsVerifier.ExpectS3FileToExist(
			bucket,
			s3ObjectPath,
		)
	})
})
