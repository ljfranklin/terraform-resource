package out_test

import (
	"archive/zip"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"

	"terraform-resource/models"
	"terraform-resource/out"
	"terraform-resource/storage"
	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out Lifecycle with Custom Plugins", func() {

	var (
		envName       string
		stateFilePath string
		planFilePath  string
		s3ObjectPath  string
		workingDir    string
		pluginDir     string
	)

	BeforeEach(func() {
		envName = helpers.RandomString("out-test")
		stateFilePath = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", envName))
		planFilePath = path.Join(bucketPath, fmt.Sprintf("%s.plan", envName))
		s3ObjectPath = path.Join(bucketPath, helpers.RandomString("out-lifecycle"))

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-test")
		Expect(err).ToNot(HaveOccurred())

		pluginDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-test-plugins")
		Expect(err).ToNot(HaveOccurred())

		awsProviderURL := fmt.Sprintf("https://releases.hashicorp.com/terraform-provider-aws/1.2.0/terraform-provider-aws_1.2.0_%s_%s.zip", runtime.GOOS, runtime.GOARCH)
		err = downloadPlugins(pluginDir, awsProviderURL)
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
		_ = os.RemoveAll(pluginDir)
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
					Source:    "fixtures/aws/",
					PluginDir: pluginDir,
					PlanOnly:  true,
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
					Source:  "fixtures/aws/",
					PlanRun: true,
				},
			},
		}

		By("running 'out' to create the plan file")

		planrunner := out.Runner{
			SourceDir: workingDir,
			LogWriter: GinkgoWriter,
		}
		_, err := planrunner.Run(planOutRequest)
		Expect(err).ToNot(HaveOccurred())

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

		Expect(createOutput.Version.PlanOnly).To(BeEmpty())

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
					PluginDir: pluginDir,
					Source:    "fixtures/aws/",
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

		By("running 'out' to delete the S3 file")

		outRequest.Params.Action = models.DestroyAction
		_, err = runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred())

		awsVerifier.ExpectS3FileToNotExist(
			outRequest.Source.Storage.Bucket,
			s3ObjectPath,
		)
	})

	It("honors plugins stored in Terraform.Source/terraform.d/plugins", func() {
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
					Source: "fixtures/custom-plugin/",
				},
			},
		}

		customProviderURL := fmt.Sprintf("https://releases.hashicorp.com/terraform-provider-tls/1.0.1/terraform-provider-tls_1.0.1_%s_%s.zip", runtime.GOOS, runtime.GOARCH)
		thirdPartyPluginDir := fmt.Sprintf("fixtures/custom-plugin/terraform.d/plugins/%s_%s/", runtime.GOOS, runtime.GOARCH)
		err := os.MkdirAll(thirdPartyPluginDir, 0700)
		Expect(err).ToNot(HaveOccurred())

		err = downloadPlugins(thirdPartyPluginDir, customProviderURL)
		Expect(err).ToNot(HaveOccurred())

		extractedFiles, err := filepath.Glob(filepath.Join(thirdPartyPluginDir, "terraform-provider-tls_*"))
		Expect(err).ToNot(HaveOccurred())

		err = os.Rename(extractedFiles[0], filepath.Join(thirdPartyPluginDir, "terraform-provider-tls_v999.999.999"))
		Expect(err).ToNot(HaveOccurred())

		By("running 'out' to verify custom plugin is detected")

		runner := out.Runner{
			SourceDir: workingDir,
			LogWriter: GinkgoWriter,
		}
		_, err = runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred())
	})
})

func downloadPlugins(pluginPath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	zipFile, err := ioutil.TempFile("", "terraform-resource-out-test")
	if err != nil {
		return err
	}
	defer zipFile.Close()

	if _, err := io.Copy(zipFile, resp.Body); err != nil {
		return err
	}

	zipReader, err := zip.OpenReader(zipFile.Name())
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, sourceFile := range zipReader.File {
		path := filepath.Join(pluginPath, sourceFile.Name)

		reader, err := sourceFile.Open()
		if err != nil {
			return err
		}
		defer reader.Close()

		writer, err := os.Create(path)
		if err != nil {
			return err
		}
		defer writer.Close()

		if _, err := io.Copy(writer, reader); err != nil {
			return err
		}

		if err := os.Chmod(path, 0700); err != nil {
			return err
		}
	}

	return nil
}
