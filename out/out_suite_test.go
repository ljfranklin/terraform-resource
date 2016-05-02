package out_test

import (
	"os"

	"github.com/ljfranklin/terraform-resource/test/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOut(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Out Suite")
}

var (
	awsVerifier *helpers.AWSVerifier
	accessKey   string
	secretKey   string
	bucket      string
	vpcID       string
	bucketPath  string
	region      string
)

var _ = BeforeSuite(func() {
	accessKey = os.Getenv("AWS_ACCESS_KEY")
	Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")
	secretKey = os.Getenv("AWS_SECRET_KEY")
	Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")
	bucket = os.Getenv("AWS_BUCKET")
	Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")
	bucketPath = os.Getenv("AWS_BUCKET_PATH")
	Expect(bucketPath).ToNot(BeEmpty(), "AWS_BUCKET_PATH must be set")
	vpcID = os.Getenv("AWS_TEST_VPC_ID")
	Expect(vpcID).ToNot(BeEmpty(), "AWS_TEST_VPC_ID must be set")

	region = os.Getenv("AWS_REGION") // optional
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
