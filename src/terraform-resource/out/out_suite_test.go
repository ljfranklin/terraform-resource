package out_test

import (
	"os"
	"github.com/ljfranklin/terraform-resource/test/helpers"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOut(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Out Suite")
}

var (
	awsVerifier    *helpers.AWSVerifier
	accessKey      string
	secretKey      string
	bucket         string
	gcsAccessKey   string
	gcsSecretKey   string
	gcsBucket      string
	gcsRegion      string
	gcsEndpoint    string
	gcsCredentials string
	bucketPath     string
	region         string
	kmsKeyID       string
)

var _ = BeforeSuite(func() {
	accessKey = os.Getenv("AWS_ACCESS_KEY")
	Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")
	secretKey = os.Getenv("AWS_SECRET_KEY")
	Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")
	bucket = os.Getenv("AWS_BUCKET")
	Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")
	bucketPath = os.Getenv("AWS_BUCKET_SUBFOLDER")
	Expect(bucketPath).ToNot(BeEmpty(), "AWS_BUCKET_SUBFOLDER must be set")

	gcsBucket = os.Getenv("GCS_BUCKET")
	Expect(gcsBucket).ToNot(BeEmpty(), "GCS_BUCKET must be set")
	gcsCredentials = os.Getenv("GCS_CREDENTIALS_JSON")
	Expect(gcsCredentials).ToNot(BeEmpty(), "GCS_CREDENTIALS_JSON must be set")
	gcsAccessKey = os.Getenv("GCS_ACCESS_KEY")
	Expect(gcsAccessKey).ToNot(BeEmpty(), "GCS_ACCESS_KEY must be set")
	gcsSecretKey = os.Getenv("GCS_SECRET_KEY")
	Expect(gcsSecretKey).ToNot(BeEmpty(), "GCS_SECRET_KEY must be set")
	gcsEndpoint = "storage.googleapis.com"
	gcsRegion = os.Getenv("GCS_REGION")
	if gcsRegion == "" {
		gcsRegion = "us-central1"
	}

	kmsKeyID = os.Getenv("S3_KMS_KEY_ID") // optional
	region = os.Getenv("AWS_REGION")      // optional
	if region == "" {
		region = "us-east-1"
	}

	awsVerifier = helpers.NewAWSVerifier(
		accessKey,
		secretKey,
		region,
		"",
	)
})
