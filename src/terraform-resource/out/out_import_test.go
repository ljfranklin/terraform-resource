package out_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/out"
	"github.com/ljfranklin/terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out Import", func() {

	var (
		envName       string
		stateFilePath string
		s3ObjectPath  string
		workspacePath string
		workingDir    string
	)

	BeforeEach(func() {
		envName = helpers.RandomString("out-test")

		workspacePath = helpers.RandomString("out-backend-test")

		stateFilePath = path.Join(workspacePath, envName, "terraform.tfstate")
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

		err = helpers.DownloadStatefulPlugin(workingDir)
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

	It("imports the existing resource during plan", func() {
		awsVerifier.ExpectS3BucketToExist(bucket)

		importsFilePath := path.Join(workingDir, "imports")
		importsFileContents := fmt.Sprintf("aws_s3_bucket.bucket: %s", bucket)
		err := ioutil.WriteFile(importsFilePath, []byte(importsFileContents), 0700)
		Expect(err).ToNot(HaveOccurred())

		By("Ensuring that an existing bucket is imported prior to plan")

		importRequest := models.OutRequest{
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
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					PlanOnly: true,
					ImportFiles: []string{
						importsFilePath,
					},
					Source: "fixtures/import/",
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

		logs := bytes.Buffer{}
		runner := out.Runner{
			SourceDir: workingDir,
			LogWriter: &logs,
		}
		_, err = runner.Run(importRequest)
		Expect(err).ToNot(HaveOccurred())
		Expect(logs.String()).To(ContainSubstring("Importing `aws_s3_bucket.bucket"))
	})
})
