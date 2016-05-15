package out_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/out"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/terraform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out Lifecycle", func() {

	var (
		subnetCIDR    string
		envName       string
		stateFilePath string
		workingDir    string
	)

	BeforeEach(func() {
		// TODO: avoid random clashes here
		rand.Seed(time.Now().UnixNano())
		subnetCIDR = fmt.Sprintf("10.0.%d.0/24", rand.Intn(256))

		envName = randomString("out-test")
		stateFilePath = path.Join(bucketPath, fmt.Sprintf("%s.tfstate", envName))

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-test")
		Expect(err).ToNot(HaveOccurred())

		// ensure relative paths resolve correctly
		err = os.Chdir(workingDir)
		Expect(err).ToNot(HaveOccurred())

		fixturesDir := path.Join(getProjectRoot(), "fixtures")
		err = exec.Command("cp", "-r", fixturesDir, workingDir).Run()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(workingDir)
		awsVerifier.DeleteSubnetWithCIDR(subnetCIDR, vpcID)
		awsVerifier.DeleteObjectFromS3(bucket, stateFilePath)
	})

	It("creates, updates, and deletes infrastructure", func() {
		outRequest := models.OutRequest{
			Source: models.Source{
				Storage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
				},
			},
			Params: models.OutParams{
				EnvName: envName,
				Terraform: terraform.Model{
					Source: "fixtures/aws/",
					Vars: map[string]interface{}{
						"access_key":  accessKey,
						"secret_key":  secretKey,
						"vpc_id":      vpcID,
						"subnet_cidr": subnetCIDR,
					},
				},
			},
		}

		By("ensuring state file does not already exist")

		awsVerifier.ExpectS3FileToNotExist(
			outRequest.Source.Storage.Bucket,
			stateFilePath,
		)

		By("running 'out' to create an AWS subnet")

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
		Expect(fields["vpc_id"]).To(Equal(vpcID))
		Expect(fields["subnet_cidr"]).To(Equal(subnetCIDR))
		Expect(fields["tag_name"]).To(Equal("terraform-resource-test")) // template default tag

		Expect(fields["subnet_id"]).ToNot(BeEmpty())
		subnetID := fields["subnet_id"].(string)

		awsVerifier.ExpectSubnetToExist(subnetID)
		awsVerifier.ExpectSubnetToHaveTags(subnetID, map[string]string{
			"Name": "terraform-resource-test",
		})

		By("ensuring that state file exists with valid version (LastModified)")

		awsVerifier.ExpectS3FileToExist(
			outRequest.Source.Storage.Bucket,
			stateFilePath,
		)

		createVersion, err := time.Parse(storage.TimeFormat, createOutput.Version.LastModified)
		Expect(err).ToNot(HaveOccurred())
		Expect(createOutput.Version.EnvName).To(Equal(outRequest.Params.EnvName))

		By("running 'out' to update the VPC")

		// omits vpc_id and subnet_cidr to ensure the resource feeds
		// previous output back in as input
		outRequest.Params.Terraform.Vars = map[string]interface{}{
			"access_key": accessKey,
			"secret_key": secretKey,
			"tag_name":   "terraform-resource-test-updated",
		}
		updateOutput, err := runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred())

		awsVerifier.ExpectSubnetToHaveTags(subnetID, map[string]string{
			"Name": "terraform-resource-test-updated",
		})

		By("ensuring that state file has been updated")

		awsVerifier.ExpectS3FileToExist(
			outRequest.Source.Storage.Bucket,
			stateFilePath,
		)

		updatedVersion, err := time.Parse(storage.TimeFormat, updateOutput.Version.LastModified)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedVersion).To(BeTemporally(">", createVersion))
		Expect(updateOutput.Version.EnvName).To(Equal(outRequest.Params.EnvName))

		By("running 'out' to delete the VPC")

		outRequest.Params.Action = models.DestroyAction
		deleteOutput, err := runner.Run(outRequest)
		Expect(err).ToNot(HaveOccurred())

		awsVerifier.ExpectSubnetToNotExist(subnetID)

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
})
