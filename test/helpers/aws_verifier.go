package helpers

import (
	"github.com/ljfranklin/terraform-resource/storage"
	. "github.com/onsi/gomega"
)

type AWSVerifier struct {
	AccessKey string
	SecretKey string
	Region    string
}

func (a AWSVerifier) ExpectS3FileToExist(bucketName string, key string) {
	Expect(a.doesS3FileExist(bucketName, key)).To(BeTrue(),
		"Expected S3 file '%s' to exist in bucket '%s', but it does not",
		key,
		bucketName)
}

func (a AWSVerifier) ExpectS3FileToNotExist(bucketName string, key string) {
	Expect(a.doesS3FileExist(bucketName, key)).To(BeFalse(),
		"Expected S3 file '%s' to not exist in bucket '%s', but it does",
		key,
		bucketName)
}

func (a AWSVerifier) doesS3FileExist(bucketName string, key string) bool {
	s3 := storage.NewS3(a.AccessKey, a.SecretKey, a.Region, bucketName)

	version, err := s3.Version(key)
	Expect(err).ToNot(HaveOccurred())

	return (version != "")
}
