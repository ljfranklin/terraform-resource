package out_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/namer/namerfakes"
	"github.com/ljfranklin/terraform-resource/out"
	"github.com/ljfranklin/terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out GCS", func() {

	var (
		backendType   string
		backendConfig map[string]interface{}
		envName       string
		stateFilePath string
		s3ObjectPath  string
		workingDir    string
		namer         namerfakes.FakeNamer
		logWriter     bytes.Buffer
		workspacePath string
	)

	BeforeEach(func() {
		workspacePath = helpers.RandomString("out-backend-test")

		envName = helpers.RandomString("out-test")
		stateFilePath = path.Join(workspacePath, envName, "terraform.tfstate")
		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-test"))

		os.Setenv("BUILD_ID", "sample-build-id")
		os.Setenv("BUILD_NAME", "sample-build-name")
		os.Setenv("BUILD_JOB_NAME", "sample-build-job-name")
		os.Setenv("BUILD_PIPELINE_NAME", "sample-build-pipeline-name")
		os.Setenv("BUILD_TEAM_NAME", "sample-build-team-name")
		os.Setenv("ATC_EXTERNAL_URL", "sample-atc-external-url")

		backendType = "gcs"
		backendConfig = map[string]interface{}{
			"bucket":      gcsBucket,
			"prefix":      workspacePath,
			"credentials": gcsCredentials,
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
		awsVerifier.DeleteObjectFromS3(bucket, stateFilePath)
	})

	It("stores the statefile in GCS bucket", func() {
		req := models.OutRequest{
			Source: models.Source{
				Terraform: models.Terraform{
					BackendType:   backendType,
					BackendConfig: backendConfig,
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

		runner := out.Runner{
			SourceDir: workingDir,
			LogWriter: &logWriter,
			Namer:     &namer,
		}
		_, err := runner.Run(req)
		Expect(err).ToNot(HaveOccurred(), "Logs: %s", logWriter.String())
	})
})
