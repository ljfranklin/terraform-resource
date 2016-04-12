package helpers

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/ljfranklin/terraform-resource/storage"
	. "github.com/onsi/gomega"
)

type AWSVerifier struct {
	ec2       *awsec2.EC2
	accessKey string
	secretKey string
	region    string
}

func NewAWSVerifier(accessKey string, secretKey string, region string) *AWSVerifier {
	awsConfig := &aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
	}
	ec2 := awsec2.New(awsSession.New(awsConfig))
	return &AWSVerifier{
		ec2:       ec2,
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
	}
}

func (a AWSVerifier) ExpectS3FileToExist(bucketName string, key string) {
	s3 := storage.NewS3(a.accessKey, a.secretKey, a.region, bucketName)

	version, err := s3.Version(key)
	Expect(err).ToNot(HaveOccurred())

	Expect(version).ToNot(BeEmpty(),
		"Expected S3 file '%s' to exist in bucket '%s', but it does not",
		key,
		bucketName)
}

func (a AWSVerifier) ExpectS3FileToNotExist(bucketName string, key string) {
	s3 := storage.NewS3(a.accessKey, a.secretKey, a.region, bucketName)

	version, err := s3.Version(key)
	Expect(err).ToNot(HaveOccurred())

	Expect(version).To(BeEmpty(),
		"Expected S3 file '%s' to not exist in bucket '%s', but it does",
		key,
		bucketName)
}

func (a AWSVerifier) ExpectVPCToExist(vpcID string) {
	vpcParams := &awsec2.DescribeVpcsInput{
		VpcIds: []*string{
			aws.String(vpcID),
		},
	}
	resp, err := a.ec2.DescribeVpcs(vpcParams)
	Expect(err).ToNot(HaveOccurred())

	Expect(resp.Vpcs).To(HaveLen(1), fmt.Sprintf("Expected VPC '%s' to exist, but it does not", vpcID))
	Expect(*resp.Vpcs[0].VpcId).To(Equal(vpcID))
}

func (a AWSVerifier) ExpectVPCToNotExist(vpcID string) {
	vpcParams := &awsec2.DescribeVpcsInput{
		VpcIds: []*string{
			aws.String(vpcID),
		},
	}
	_, err := a.ec2.DescribeVpcs(vpcParams)
	Expect(err).To(HaveOccurred())
	ec2err := err.(awserr.Error)

	Expect(ec2err.Code()).To(Equal("InvalidVpcID.NotFound"), fmt.Sprintf("Expected VPC '%s' to not exist, but it does", vpcID))
}

func (a AWSVerifier) ExpectVPCToHaveTags(vpcID string, expectedTags map[string]string) {
	vpcParams := &awsec2.DescribeVpcsInput{
		VpcIds: []*string{
			aws.String(vpcID),
		},
	}
	resp, err := a.ec2.DescribeVpcs(vpcParams)
	Expect(err).ToNot(HaveOccurred())

	Expect(resp.Vpcs).To(HaveLen(1))
	Expect(*resp.Vpcs[0].VpcId).To(Equal(vpcID))

	tags := resp.Vpcs[0].Tags
	Expect(tags).To(HaveLen(len(expectedTags)))

	formattedTags := []*awsec2.Tag{}
	for key, value := range expectedTags {
		formattedTags = append(formattedTags, &awsec2.Tag{
			Key:   &key,
			Value: &value,
		})
	}
	Expect(tags).To(ConsistOf(formattedTags))
}

func (a AWSVerifier) DeleteVPC(vpcID string) {
	vpcParams := &awsec2.DeleteVpcInput{
		VpcId: aws.String(vpcID),
	}
	_, err := a.ec2.DeleteVpc(vpcParams)
	Expect(err).ToNot(HaveOccurred())
}
