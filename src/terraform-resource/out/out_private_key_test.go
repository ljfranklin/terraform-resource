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
	"terraform-resource/storage"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out - Private Key", func() {

	var (
		storageModel    storage.Model
		envName         string
		stateFilePath   string
		s3ObjectPath    string
		workingDir      string
		fixtureEnvName  string
		pathToS3Fixture string
		privateKey      string
	)

	BeforeEach(func() {
		privateKey = os.Getenv("GITHUB_PRIVATE_KEY")
		Expect(privateKey).ToNot(BeEmpty(), "GITHUB_PRIVATE_KEY must be set")

		region := os.Getenv("AWS_REGION") // optional
		if region == "" {
			region = "us-east-1"
		}

		envName = helpers.RandomString("out-test")
		stateFilePath = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", envName))
		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-test"))

		os.Setenv("BUILD_ID", "sample-build-id")
		os.Setenv("BUILD_NAME", "sample-build-name")
		os.Setenv("BUILD_JOB_NAME", "sample-build-job-name")
		os.Setenv("BUILD_PIPELINE_NAME", "sample-build-pipeline-name")
		os.Setenv("BUILD_TEAM_NAME", "sample-build-team-name")
		os.Setenv("ATC_EXTERNAL_URL", "sample-atc-external-url")

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
	})

	AfterEach(func() {
		_ = os.RemoveAll(workingDir)
		awsVerifier.DeleteObjectFromS3(bucket, s3ObjectPath)
		awsVerifier.DeleteObjectFromS3(bucket, stateFilePath)
	})

	It("fetches modules over ssh", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
			},
			Params: models.OutParams{
				EnvName: envName,
				Terraform: models.Terraform{
					Source:     "fixtures/private-module/",
					PrivateKey: privateKey,
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
		createOutput, err := runner.Run(req)
		Expect(err).ToNot(HaveOccurred())

		Expect(createOutput.Metadata).ToNot(BeEmpty())
		fields := map[string]interface{}{}
		for _, field := range createOutput.Metadata {
			fields[field.Name] = field.Value
		}
		Expect(fields["env_name"]).To(Equal(envName))
		expectedMD5 := fmt.Sprintf("%x", md5.Sum([]byte("terraform-is-neat")))
		Expect(fields["content_md5"]).To(Equal(expectedMD5))

		awsVerifier.ExpectS3FileToExist(bucket, s3ObjectPath)
	})
})
