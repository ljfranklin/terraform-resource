package storage

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type s3 struct {
	client *awss3.S3
	model  Model
}

const (
	maxRetries    = 10
	defaultRegion = "us-east-1"
)

func NewS3(m Model) Storage {

	regionName := m.RegionName
	if len(regionName) == 0 {
		regionName = defaultRegion
	}

	awsConfig := &aws.Config{
		Region:           aws.String(regionName),
		S3ForcePathStyle: aws.Bool(true),
		MaxRetries:       aws.Int(maxRetries),
		Logger:           nil,
	}

	if m.AccessKeyID != "" && m.SecretAccessKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentials(m.AccessKeyID, m.SecretAccessKey, "")
	}

	if len(m.Endpoint) > 0 {
		awsConfig.Endpoint = aws.String(m.Endpoint)
	}

	session := awsSession.Must(awsSession.NewSession())
	client := awss3.New(session, awsConfig)

	if m.ShouldUseSigningV2() {
		Setv2Handlers(client)
	}

	return &s3{
		client: client,
		model:  m,
	}
}

func (s *s3) Download(filename string, destination io.Writer) (Version, error) {
	key := path.Join(s.model.BucketPath, filename)
	params := &awss3.GetObjectInput{
		Bucket: aws.String(s.model.Bucket),
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

	key := path.Join(s.model.BucketPath, filename)
	uploadInput := &s3manager.UploadInput{
		Bucket: aws.String(s.model.Bucket),
		Key:    aws.String(key),
		Body:   content,
	}
	if s.model.ServerSideEncryption != "" {
		uploadInput.ServerSideEncryption = aws.String(s.model.ServerSideEncryption)
	}
	if s.model.SSEKMSKeyId != "" {
		uploadInput.ServerSideEncryption = aws.String("aws:kms")
		uploadInput.SSEKMSKeyId = aws.String(s.model.SSEKMSKeyId)
	}

	_, err := uploader.Upload(uploadInput)
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
	key := path.Join(s.model.BucketPath, filename)
	params := &awss3.DeleteObjectInput{
		Bucket: aws.String(s.model.Bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(params)
	if err != nil {
		if reqErr, ok := err.(awserr.RequestFailure); ok && reqErr.StatusCode() == 404 {
			return nil // already gone
		}
		return fmt.Errorf("DeleteObject request failed.\nError: %s", err.Error())
	}

	return nil
}

func (s *s3) Version(filename string) (Version, error) {
	key := path.Join(s.model.BucketPath, filename)
	params := &awss3.HeadObjectInput{
		Bucket: aws.String(s.model.Bucket),
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

func (s *s3) LatestVersion(filterRegex string) (Version, error) {
	regex := regexp.MustCompile(filterRegex)

	params := &awss3.ListObjectsInput{
		Bucket: aws.String(s.model.Bucket),
		Prefix: aws.String(s.model.BucketPath),
	}

	resp, err := s.client.ListObjects(params)
	if err != nil {
		return Version{}, fmt.Errorf("ListObjects request failed.\nError: %s", err)
	}

	filteredObjects := resp.Contents[:0]
	for _, file := range resp.Contents {
		if regex.MatchString(*file.Key) {
			filteredObjects = append(filteredObjects, file)
		}
	}
	sort.Sort(ByLastModified(filteredObjects))
	if len(filteredObjects) == 0 {
		return Version{}, nil // no versions exist
	}

	latest := filteredObjects[len(filteredObjects)-1]
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
