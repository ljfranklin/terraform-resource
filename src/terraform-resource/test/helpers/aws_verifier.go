package helpers

import (
	"io"
	"time"

	"terraform-resource/storage"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	. "github.com/onsi/gomega"
)

type AWSVerifier struct {
	ec2 *awsec2.EC2
	s3  *awss3.S3
}

func NewAWSVerifier(accessKey string, secretKey string, region string, endpoint string) *AWSVerifier {
	if len(region) == 0 {
		region = " " // aws sdk complains if region is empty
	}
	awsConfig := &aws.Config{
		Region:           aws.String(region),
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		S3ForcePathStyle: aws.Bool(true),
		MaxRetries:       aws.Int(10),
	}
	if len(endpoint) > 0 {
		awsConfig.Endpoint = aws.String(endpoint)
	}

	ec2 := awsec2.New(awsSession.New(awsConfig))
	s3 := awss3.New(awsSession.New(awsConfig))
	if len(endpoint) > 0 {
		// many s3-compatible endpoints only support v2 signing
		storage.Setv2Handlers(s3)
	}

	return &AWSVerifier{
		ec2: ec2,
		s3:  s3,
	}
}

func (a AWSVerifier) ExpectS3BucketToExist(bucketName string) {
	params := &awss3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	}

	_, err := a.s3.HeadBucket(params)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(),
		"Expected S3 bucket '%s' to exist, but it does not",
		bucketName)
}

func (a AWSVerifier) ExpectS3FileToExist(bucketName string, key string) {
	params := &awss3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	var lastErr error
	for i := 0; i < 5; i++ {
		_, lastErr = a.s3.HeadObject(params)
		if lastErr == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	ExpectWithOffset(1, lastErr).ToNot(HaveOccurred(),
		"Expected S3 file '%s' to exist in bucket '%s', but it does not",
		key,
		bucketName)
}

func (a AWSVerifier) ExpectS3FileToNotExist(bucketName string, key string) {
	params := &awss3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	var lastErr error
	for i := 0; i < 5; i++ {
		_, lastErr = a.s3.HeadObject(params)
		if lastErr != nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	ExpectWithOffset(1, lastErr).To(HaveOccurred(),
		"Expected S3 file '%s' to not exist in bucket '%s', but it does",
		key,
		bucketName)
	reqErr, ok := lastErr.(awserr.RequestFailure)
	ExpectWithOffset(1, ok).To(BeTrue(), "Invalid AWS error type: %s", lastErr)
	ExpectWithOffset(1, reqErr.StatusCode()).To(Equal(404))
}

func (a AWSVerifier) ExpectS3ServerSideEncryption(bucketName string, key string, expectedAlgo string, expectedKMSKeyID ...string) {
	params := &awss3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	headResp, err := a.s3.HeadObject(params)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	ExpectWithOffset(1, headResp.ServerSideEncryption).ToNot(BeNil(), "Expected ServerSideEncryption to be set, but it was not")
	ExpectWithOffset(1, *headResp.ServerSideEncryption).To(Equal(expectedAlgo))

	if len(expectedKMSKeyID) > 0 {
		ExpectWithOffset(1, headResp.SSEKMSKeyId).ToNot(BeNil(), "Expected SSEKMSKeyId to be set, but it was not")
		// the returned KeyId may have the `arn::.../` prefix
		ExpectWithOffset(1, *headResp.SSEKMSKeyId).To(ContainSubstring(expectedKMSKeyID[0]))
	}
}

func (a AWSVerifier) GetLastModifiedFromS3(bucketName string, key string) string {
	params := &awss3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	resp, err := a.s3.HeadObject(params)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	return resp.LastModified.Format(storage.TimeFormat)
}

func (a AWSVerifier) GetMD5FromS3(bucketName string, key string) string {
	params := &awss3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	resp, err := a.s3.HeadObject(params)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	return *resp.ETag
}

func (a AWSVerifier) UploadObjectToS3(bucketName string, key string, content io.Reader) {
	uploader := s3manager.NewUploaderWithClient(a.s3)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   content,
	})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
}

func (a AWSVerifier) DeleteObjectFromS3(bucketName string, key string) {
	deleteInput := &awss3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}
	_, err := a.s3.DeleteObject(deleteInput)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
}
