package main_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"time"

	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/test/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Out", func() {

	var (
		outRequest  models.OutRequest
		awsVerifier *helpers.AWSVerifier
	)

	BeforeEach(func() {
		accessKey := os.Getenv("AWS_ACCESS_KEY")
		Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")

		secretKey := os.Getenv("AWS_SECRET_KEY")
		Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")

		bucket := os.Getenv("AWS_BUCKET")
		Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")

		bucketPath := os.Getenv("AWS_BUCKET_PATH") // optional

		region := os.Getenv("AWS_REGION") // optional
		if region == "" {
			region = "us-east-1"
		}

		awsVerifier = helpers.NewAWSVerifier(
			accessKey,
			secretKey,
			region,
		)

		stateFileKey := path.Join(bucketPath, randomString("out-test"))

		outRequest = models.OutRequest{
			Source: models.Source{
				Bucket:          bucket,
				Key:             stateFileKey,
				AccessKeyID:     accessKey,
				SecretAccessKey: secretKey,
			},
			Params: models.Params{
				TerraformSource: "", // overridden in contexts
				TerraformVars: map[string]interface{}{
					"access_key": accessKey,
					"secret_key": secretKey,
				},
			},
		}
	})

	assertOutLifecycle := func() {
		It("succeeds in creating, outputing, and deleting infrastructure", func() {
			By("ensuring state file does not already exist")

			awsVerifier.ExpectS3FileToNotExist(
				outRequest.Source.Bucket,
				outRequest.Source.Key,
			)

			By("running 'out' to create an AWS VPC")

			createOutput := models.OutResponse{}
			runOutCommand(outRequest, &createOutput)

			Expect(createOutput.Metadata).ToNot(BeEmpty())
			vpcID := ""
			for _, field := range createOutput.Metadata {
				if field.Name == "vpc_id" {
					vpcID = field.Value.(string)
					break
				}
			}
			Expect(vpcID).ToNot(BeEmpty())

			awsVerifier.ExpectVPCToExist(vpcID)

			By("ensuring that state file exists with valid version (LastModified)")

			awsVerifier.ExpectS3FileToExist(
				outRequest.Source.Bucket,
				outRequest.Source.Key,
			)

			// does version match format "2006-01-02T15:04:05Z"?
			createVersion, err := time.Parse(time.RFC3339, createOutput.Version.Version)
			Expect(err).ToNot(HaveOccurred())

			By("running 'out' to update the VPC")

			outRequest.Params.TerraformVars["tag_name"] = "terraform-resource-test-updated"
			updateOutput := models.OutResponse{}
			runOutCommand(outRequest, &updateOutput)

			awsVerifier.ExpectVPCToHaveTags(vpcID, map[string]string{
				"Name": "terraform-resource-test-updated",
			})

			By("ensuring that state file has been updated")

			awsVerifier.ExpectS3FileToExist(
				outRequest.Source.Bucket,
				outRequest.Source.Key,
			)

			updatedVersion, err := time.Parse(time.RFC3339, updateOutput.Version.Version)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedVersion).To(BeTemporally(">", createVersion))

			By("running 'out' to delete the VPC")

			outRequest.Params.Action = models.DestroyAction
			deleteOutput := models.OutResponse{}
			runOutCommand(outRequest, &deleteOutput)

			awsVerifier.ExpectVPCToNotExist(vpcID)

			By("ensuring that state file no longer exists")

			awsVerifier.ExpectS3FileToNotExist(
				outRequest.Source.Bucket,
				outRequest.Source.Key,
			)

			deletedVersion, err := time.Parse(time.RFC3339, deleteOutput.Version.Version)
			Expect(err).ToNot(HaveOccurred())
			Expect(deletedVersion).To(BeTemporally(">", updatedVersion))
		})
	}

	Context("when provided a local terraform source", func() {
		BeforeEach(func() {
			outRequest.Params.TerraformSource = "fixtures/aws/"
		})

		assertOutLifecycle()
	})

	Context("when provided a remote terraform source", func() {
		BeforeEach(func() {
			// Note: changes to fixture must be pushed before running this test
			outRequest.Params.TerraformSource = "github.com/ljfranklin/terraform-resource//fixtures/aws/"
		})

		assertOutLifecycle()
	})
})

func runOutCommand(input interface{}, output interface{}) {
	pathToSources := getProjectRoot()
	command := exec.Command(pathToOutBinary, pathToSources)

	stdin, err := command.StdinPipe()
	Expect(err).ToNot(HaveOccurred())

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	err = json.NewEncoder(stdin).Encode(input)
	Expect(err).ToNot(HaveOccurred())
	stdin.Close()

	Eventually(session, 2*time.Minute).Should(gexec.Exit(0))

	err = json.Unmarshal(session.Out.Contents(), output)
	Expect(err).ToNot(HaveOccurred())
}

func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(1)
	return path.Join(path.Dir(filename), "..")
}

func randomString(prefix string) string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	Expect(err).ToNot(HaveOccurred())
	return fmt.Sprintf("%s-%x", prefix, b)
}
