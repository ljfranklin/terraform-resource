package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"time"

	"github.com/ljfranklin/terraform-resource/test/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestOut(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Out Suite")
}

var (
	pathToOutBinary string
	awsVerifier     *helpers.AWSVerifier
	accessKey       string
	secretKey       string
	bucket          string
	vpcID           string
	bucketPath      string
	region          string
)

var _ = BeforeSuite(func() {
	var err error
	pathToOutBinary, err = gexec.Build("github.com/ljfranklin/terraform-resource/out")
	Expect(err).ToNot(HaveOccurred())

	accessKey = os.Getenv("AWS_ACCESS_KEY")
	Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")
	secretKey = os.Getenv("AWS_SECRET_KEY")
	Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")
	bucket = os.Getenv("AWS_BUCKET")
	Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")
	vpcID = os.Getenv("AWS_TEST_VPC_ID")
	Expect(vpcID).ToNot(BeEmpty(), "AWS_TEST_VPC_ID must be set")

	bucketPath = os.Getenv("AWS_BUCKET_PATH") // optional
	region = os.Getenv("AWS_REGION")          // optional
	if region == "" {
		region = "us-east-1"
	}

	awsVerifier = helpers.NewAWSVerifier(
		accessKey,
		secretKey,
		region,
	)

	awsVerifier.ExpectVPCToExist(vpcID)
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func runOutCommand(input interface{}, output interface{}, workingDir string) {
	command := exec.Command(pathToOutBinary, workingDir)

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

	Expect(session.Err).To(gbytes.Say("Apply complete!"))
}
