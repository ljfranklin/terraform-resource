package storage

import (
	"fmt"
	"io"
	"path"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type s3 struct {
	client     *awss3.S3
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
	if len(m.Endpoint) > 0 {
		awsConfig.Endpoint = aws.String(m.Endpoint)
	}

	session := awsSession.New(awsConfig)
	client := awss3.New(session, awsConfig)
	if m.ShouldUseSigningV2() {
		Setv2Handlers(client)
	}

	return &s3{
		client:     client,
		bucketName: m.Bucket,
		bucketPath: m.BucketPath,
	}
}

func (s *s3) Download(filename string, destination io.Writer) (Version, error) {
	key := path.Join(s.bucketPath, filename)
	params := &awss3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	resp, err := s.client.GetObject(params)
	if err != nil {
		return Version{}, fmt.Errorf("GetObject request failed.\nError: %s", err.Error())
	}
	defer resp.Body.Close()

	_, err = io.Copy(destination, resp.Body)
	if err != nil {
		return Version{}, fmt.Errorf("Failed to copy download to local file: %s", err)
	}

	version := Version{
		LastModified: *resp.LastModified,
		StateFile:    filename,
	}
	return version, nil
}

func (s *s3) Upload(filename string, content io.Reader) (Version, error) {

	uploader := s3manager.NewUploaderWithClient(s.client)

	key := path.Join(s.bucketPath, filename)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
		Body:   content,
	})
	if err != nil {
		return Version{}, fmt.Errorf("Failed to Upload to S3: %s", err.Error())
	}

	version, err := s.Version(filename)
	if err != nil {
		return Version{}, err
	}
	return version, nil
}

func (s *s3) Delete(filename string) error {
	key := path.Join(s.bucketPath, filename)
	params := &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(params)
	if err != nil {
		return fmt.Errorf("DeleteObject request failed.\nError: %s", err.Error())
	}

	return nil
}

func (s *s3) Version(filename string) (Version, error) {
	key := path.Join(s.bucketPath, filename)
	params := &awss3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}

	resp, err := s.client.HeadObject(params)
	if err != nil {
		if reqErr, ok := err.(awserr.RequestFailure); ok && reqErr.StatusCode() == 404 {
			return Version{}, nil // no versions exist
		}
		return Version{}, fmt.Errorf("HeadObject request failed.\nError: %s", err.Error())
	}

	version := Version{
		LastModified: *resp.LastModified,
		StateFile:    filename,
	}
	return version, nil
}

func (s *s3) LatestVersion() (Version, error) {
	params := &awss3.ListObjectsInput{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(s.bucketPath),
	}

	resp, err := s.client.ListObjects(params)
	if err != nil {
		return Version{}, fmt.Errorf("ListObjects request failed.\nError: %s", err)
	}

	fileObjects := resp.Contents
	if len(fileObjects) == 0 {
		return Version{}, nil // no versions exist
	}
	sort.Sort(ByLastModified(fileObjects))

	latest := fileObjects[len(fileObjects)-1]
	stateFile := path.Base(*latest.Key)
	version := Version{
		LastModified: *latest.LastModified,
		StateFile:    stateFile,
	}
	return version, nil
}

type ByLastModified []*awss3.Object

func (a ByLastModified) Len() int           { return len(a) }
func (a ByLastModified) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByLastModified) Less(i, j int) bool { return a[i].LastModified.Before(*a[j].LastModified) }
