package out_test

import (
	"os"

	"terraform-resource/test/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOut(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Out Suite")
}

var (
	awsVerifier           *helpers.AWSVerifier
	accessKey             string
	secretKey             string
	bucket                string
	s3CompatibleAccessKey string
	s3CompatibleSecretKey string
	s3CompatibleBucket    string
	s3CompatibleEndpoint  string
	vpcID                 string
	bucketPath            string
	region                string
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

	s3CompatibleAccessKey = os.Getenv("S3_COMPATIBLE_ACCESS_KEY")
	Expect(s3CompatibleAccessKey).ToNot(BeEmpty(), "S3_COMPATIBLE_ACCESS_KEY must be set")
	s3CompatibleSecretKey = os.Getenv("S3_COMPATIBLE_SECRET_KEY")
	Expect(s3CompatibleSecretKey).ToNot(BeEmpty(), "S3_COMPATIBLE_SECRET_KEY must be set")
	s3CompatibleBucket = os.Getenv("S3_COMPATIBLE_BUCKET")
	Expect(s3CompatibleBucket).ToNot(BeEmpty(), "S3_COMPATIBLE_BUCKET must be set")
	s3CompatibleEndpoint = os.Getenv("S3_COMPATIBLE_ENDPOINT")
	Expect(s3CompatibleEndpoint).ToNot(BeEmpty(), "S3_COMPATIBLE_ENDPOINT must be set")

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

	awsVerifier.ExpectVPCToExist(vpcID)
})
