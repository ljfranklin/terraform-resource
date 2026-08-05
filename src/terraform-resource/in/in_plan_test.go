package in_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/ljfranklin/terraform-resource/in"
	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/out"
	"github.com/ljfranklin/terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JSON Plan", func() {

	var (
		backendType   string
		backendConfig map[string]interface{}
		envName       string
		stateFilePath string
		planFilePath  string
		s3ObjectPath  string
		workingDir    string
		inDir         string
		workspacePath string
		awsVerifier   *helpers.AWSVerifier
		accessKey     string
		secretKey     string
		bucket        string
		bucketPath    string
		region        string
	)

	BeforeEach(func() {
		accessKey = os.Getenv("AWS_ACCESS_KEY")
		Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")
		secretKey = os.Getenv("AWS_SECRET_KEY")
		Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")
		bucket = os.Getenv("AWS_BUCKET")
		Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")
		bucketPath = os.Getenv("AWS_BUCKET_SUBFOLDER")
		Expect(bucketPath).ToNot(BeEmpty(), "AWS_BUCKET_SUBFOLDER must be set")

		region = os.Getenv("AWS_REGION") // optional
		if region == "" {
			region = "us-east-1"
		}

		awsVerifier = helpers.NewAWSVerifier(
			accessKey,
			secretKey,
			region,
			"",
		)

		region := os.Getenv("AWS_REGION") // optional
		if region == "" {
			region = "us-east-1"
		}

		workspacePath = helpers.RandomString("out-backend-test")

		envName = helpers.RandomString("out-test")
		stateFilePath = path.Join(workspacePath, envName, "terraform.tfstate")
		planFilePath = path.Join(workspacePath, fmt.Sprintf("%s-plan", envName), "terraform.tfstate")
		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-test"))

		backendType = "s3"
		backendConfig = map[string]interface{}{
			"bucket":               bucket,
			"key":                  "terraform.tfstate",
			"access_key":           accessKey,
			"secret_key":           secretKey,
			"region":               region,
			"workspace_key_prefix": workspacePath,
		}

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-test")
		Expect(err).ToNot(HaveOccurred())

		inDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-in-test")
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
		_ = os.RemoveAll(inDir)
		awsVerifier.DeleteObjectFromS3(bucket, s3ObjectPath)
		awsVerifier.DeleteObjectFromS3(bucket, planFilePath)
		awsVerifier.DeleteObjectFromS3(bucket, stateFilePath)
	})

	It("plan infrastructure and test it without plan output", func() {
		planOutRequest := models.OutRequest{
			Source: models.Source{
				Terraform: models.Terraform{
					BackendType:   backendType,
					BackendConfig: backendConfig,
				},
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

		By("running 'out' to create the plan file")

		planrunner := out.Runner{
			SourceDir: workingDir,
			LogWriter: GinkgoWriter,
		}
		planOutput, err := planrunner.Run(planOutRequest)
		Expect(err).ToNot(HaveOccurred())

		By("ensuring that plan file exists")

		awsVerifier.ExpectS3FileToExist(
			bucket,
			planFilePath,
		)
		defer awsVerifier.DeleteObjectFromS3(bucket, planFilePath)

		Expect(planOutput.Version.EnvName).To(Equal(planOutRequest.Params.EnvName))
		Expect(planOutput.Version.PlanOnly).To(Equal("true"), "Expected PlanOnly to be true, but was false")
		Expect(planOutput.Version.Serial).To(BeEmpty())
		Expect(planOutput.Version.PlanChecksum).To(MatchRegexp("[0-9|a-f]+"))

		By("does not output the planfile if `output_planfile` is not given")

		inReq := models.InRequest{
			Source: models.Source{
				Terraform: models.Terraform{
					Source: ".",
					Env: map[string]string{
						"HOME": inDir,
					},
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
			Version: planOutput.Version,
			Params: models.InParams{
				OutputJSONPlanfile: false,
			},
		}

		runner := in.Runner{
			OutputDir: inDir,
		}
		_, err = runner.Run(inReq)
		Expect(err).ToNot(HaveOccurred())

		expectedNamePath := path.Join(inDir, "name")
		Expect(expectedNamePath).To(BeAnExistingFile())

		expectedPlanPath := path.Join(inDir, "plan.json")
		Expect(expectedPlanPath).ToNot(BeAnExistingFile())
	})

	It("plan infrastructure and test it", func() {
		planOutRequest := models.OutRequest{
			Source: models.Source{
				Terraform: models.Terraform{
					BackendType:   backendType,
					BackendConfig: backendConfig,
				},
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

		By("running 'out' to create the plan file")

		planrunner := out.Runner{
			SourceDir: workingDir,
			LogWriter: GinkgoWriter,
		}
		planOutput, err := planrunner.Run(planOutRequest)
		Expect(err).ToNot(HaveOccurred())

		By("outputs the planfile if `output_planfile` is given")

		inReq := models.InRequest{
			Source: models.Source{
				Terraform: models.Terraform{
					Source: ".",
					Env: map[string]string{
						"HOME": inDir,
					},
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
			Version: planOutput.Version,
			Params: models.InParams{
				OutputJSONPlanfile: true,
			},
		}

		runner := in.Runner{
			OutputDir: inDir,
		}
		_, err = runner.Run(inReq)
		Expect(err).ToNot(HaveOccurred())

		expectedNamePath := path.Join(inDir, "name")
		Expect(expectedNamePath).To(BeAnExistingFile())

		expectedPlanPath := path.Join(inDir, "plan.json")
		Expect(expectedPlanPath).To(BeAnExistingFile())

		stateContents, err := ioutil.ReadFile(expectedPlanPath)
		Expect(err).To(BeNil())
		Expect(string(stateContents)).To(ContainSubstring("variables"))
		Expect(string(stateContents)).To(ContainSubstring("output_changes"))
		Expect(string(stateContents)).To(ContainSubstring("resource_changes"))
	})

	It("HACK: outputs metadata file if statefile exists", func() {
		planApplyRequest := models.OutRequest{
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

		applyRunner := out.Runner{
			SourceDir: workingDir,
			LogWriter: GinkgoWriter,
		}
		_, err := applyRunner.Run(planApplyRequest)
		Expect(err).ToNot(HaveOccurred())

		planOutRequest := models.OutRequest{
			Source: models.Source{
				Terraform: models.Terraform{
					BackendType:   backendType,
					BackendConfig: backendConfig,
				},
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

		planrunner := out.Runner{
			SourceDir: workingDir,
			LogWriter: GinkgoWriter,
		}
		planOutput, err := planrunner.Run(planOutRequest)
		Expect(err).ToNot(HaveOccurred())

		inReq := models.InRequest{
			Source: models.Source{
				Terraform: models.Terraform{
					Source: ".",
					Env: map[string]string{
						"HOME": inDir,
					},
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
			Version: planOutput.Version,
			Params:  models.InParams{},
		}

		runner := in.Runner{
			OutputDir: inDir,
		}
		_, err = runner.Run(inReq)
		Expect(err).ToNot(HaveOccurred())

		expectedOutputPath := path.Join(inDir, "metadata")
		Expect(expectedOutputPath).To(BeAnExistingFile())
		outputFile, err := os.Open(expectedOutputPath)
		Expect(err).ToNot(HaveOccurred())
		defer outputFile.Close()

		outputContents := map[string]interface{}{}
		err = json.NewDecoder(outputFile).Decode(&outputContents)
		Expect(err).ToNot(HaveOccurred())

		Expect(outputContents).NotTo(BeEmpty())
	})
})
