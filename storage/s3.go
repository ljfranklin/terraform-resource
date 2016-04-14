package storage

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type s3 struct {
	session    *awsSession.Session
	awsConfig  *aws.Config
	bucketName string
}

const (
	maxRetries    = 10
	defaultRegion = "us-east-1"
)

func NewS3(m Model) Storage {

	creds := credentials.NewStaticCredentials(m.AccessKeyID, m.SecretAccessKey, "")

	regionName := m.RegionName
	if len(regionName) == 0 {
		regionName = defaultRegion
	}

	awsConfig := &aws.Config{
		Region:           aws.String(regionName),
		Credentials:      creds,
		S3ForcePathStyle: aws.Bool(true),
		MaxRetries:       aws.Int(maxRetries),
	}

	session := awsSession.New(awsConfig)
	return &s3{
		session:    session,
		awsConfig:  awsConfig,
		bucketName: m.Bucket,
	}
}

func (s *s3) Download(key string, destination io.Writer) error {
	client := awss3.New(s.session, s.awsConfig)

	params := &awss3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	resp, err := client.GetObject(params)
	if err != nil {
		return fmt.Errorf("GetObject request failed.\nError: %s", err.Error())
	}
	defer resp.Body.Close()

	_, err = io.Copy(destination, resp.Body)
	return nil
}

func (s *s3) Upload(key string, content io.Reader) error {

	uploader := s3manager.NewUploader(s.session)

	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
		Body:   content,
	})
	if err != nil {
		return fmt.Errorf("Failed to Upload to S3: %s", err.Error())
	}
	return nil
}

func (s *s3) Delete(key string) error {
	client := awss3.New(s.session, s.awsConfig)

	params := &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	_, err := client.DeleteObject(params)
	if err != nil {
		return fmt.Errorf("DeleteObject request failed.\nError: %s", err.Error())
	}

	return nil
}

func (s *s3) Version(key string) (string, error) {

	client := awss3.New(s.session, s.awsConfig)

	params := &awss3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	resp, err := client.HeadObject(params)
	if err != nil {
		if reqErr, ok := err.(awserr.RequestFailure); ok && reqErr.StatusCode() == 404 {
			return "", nil // no versions exist
		}
		return "", fmt.Errorf("HeadObject request failed.\nError: %s", err.Error())
	}

	lastModified := resp.LastModified.Format(time.RFC3339) // e.g. "2006-01-02T15:04:05Z"
	return lastModified, nil
}
