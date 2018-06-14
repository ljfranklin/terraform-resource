package out_test

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"terraform-resource/models"
	"terraform-resource/out"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out Override", func() {

	var (
		envName       string
		stateFilePath string
		s3ObjectPath  string
		workspacePath string
		workingDir    string
	)

	BeforeEach(func() {
		envName = helpers.RandomString("out-override-test")

		workspacePath = helpers.RandomString("out-override-test")

		stateFilePath = path.Join(workspacePath, envName, "terraform.tfstate")
		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-override-test"))

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-override-test")
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

	It("overrides the existing resource definition", func() {
		req := models.OutRequest{
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

	It("errors when given a directory", func() {
		req := models.OutRequest{
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
					OverrideFiles: []string{
						"fixtures/override/",
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

		_, err := runner.Run(req)
		Expect(err).To(HaveOccurred())

		Expect(err.Error()).To(ContainSubstring("fixtures/override"))
	})

	It("errors when given an invalid path", func() {
		req := models.OutRequest{
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
					OverrideFiles: []string{
						"does-not-exist",
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

		_, err := runner.Run(req)
		Expect(err).To(HaveOccurred())

		Expect(err.Error()).To(ContainSubstring("does-not-exist"))
	})
})
