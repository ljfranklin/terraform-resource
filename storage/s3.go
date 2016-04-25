package storage

import (
	"fmt"
	"io"
	"sort"

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
	bucketPath string
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
		bucketPath: m.BucketPath,
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

func (s *s3) Version(key string) (Version, error) {

	client := awss3.New(s.session, s.awsConfig)

	params := &awss3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	resp, err := client.HeadObject(params)
	if err != nil {
		if reqErr, ok := err.(awserr.RequestFailure); ok && reqErr.StatusCode() == 404 {
			return Version{}, nil // no versions exist
		}
		return Version{}, fmt.Errorf("HeadObject request failed.\nError: %s", err.Error())
	}

	version := Version{
		LastModified: resp.LastModified.Format(TimeFormat),
		StateFileKey: key,
	}
	if err = version.Validate(); err != nil {
		return Version{}, fmt.Errorf("Failed to validate state file version: %s", err)
	}
	return version, nil
}

func (s *s3) LatestVersion() (Version, error) {

	client := awss3.New(s.session, s.awsConfig)

	params := &awss3.ListObjectsInput{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(s.bucketPath),
	}

	resp, err := client.ListObjects(params)
	if err != nil {
		return Version{}, fmt.Errorf("ListObjects request failed.\nError: %s", err)
	}

	fileObjects := resp.Contents
	if len(fileObjects) == 0 {
		return Version{}, nil // no versions exist
	}
	sort.Sort(ByLastModified(fileObjects))

	latest := fileObjects[len(fileObjects)-1]
	version := Version{
		LastModified: latest.LastModified.Format(TimeFormat),
		StateFileKey: *latest.Key,
	}
	if err = version.Validate(); err != nil {
		return Version{}, fmt.Errorf("Failed to validate state file version: %s", err)
	}
	return version, nil
}

type ByLastModified []*awss3.Object

func (a ByLastModified) Len() int           { return len(a) }
func (a ByLastModified) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByLastModified) Less(i, j int) bool { return a[i].LastModified.Before(*a[j].LastModified) }
