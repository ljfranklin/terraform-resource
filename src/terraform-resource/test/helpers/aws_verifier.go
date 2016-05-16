package helpers

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"terraform-resource/storage"
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

func (a AWSVerifier) ExpectS3FileToExist(bucketName string, key string) {
	params := &awss3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	_, err := a.s3.HeadObject(params)
	Expect(err).ToNot(HaveOccurred(),
		"Expected S3 file '%s' to exist in bucket '%s', but it does not",
		key,
		bucketName)
}

func (a AWSVerifier) ExpectS3FileToNotExist(bucketName string, key string) {

	params := &awss3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	_, err := a.s3.HeadObject(params)
	Expect(err).To(HaveOccurred(),
		"Expected S3 file '%s' to not exist in bucket '%s', but it does",
		key,
		bucketName)

	reqErr, ok := err.(awserr.RequestFailure)
	Expect(ok).To(BeTrue(), "Invalid AWS error type: %s", err)
	Expect(reqErr.StatusCode()).To(Equal(404))
}

func (a AWSVerifier) GetLastModifiedFromS3(bucketName string, key string) string {
	params := &awss3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	resp, err := a.s3.HeadObject(params)
	Expect(err).ToNot(HaveOccurred())
	return resp.LastModified.Format(storage.TimeFormat)
}

func (a AWSVerifier) GetMD5FromS3(bucketName string, key string) string {
	params := &awss3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	resp, err := a.s3.HeadObject(params)
	Expect(err).ToNot(HaveOccurred())
	return *resp.ETag
}

func (a AWSVerifier) UploadObjectToS3(bucketName string, key string, content io.Reader) {
	uploader := s3manager.NewUploaderWithClient(a.s3)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   content,
	})
	Expect(err).ToNot(HaveOccurred())
}

func (a AWSVerifier) DeleteObjectFromS3(bucketName string, key string) {
	deleteInput := &awss3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}
	_, err := a.s3.DeleteObject(deleteInput)
	Expect(err).ToNot(HaveOccurred())
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

func (a AWSVerifier) ExpectSubnetToExist(subnetID string) {
	subnetParams := &awsec2.DescribeSubnetsInput{
		SubnetIds: []*string{
			aws.String(subnetID),
		},
	}
	resp, err := a.ec2.DescribeSubnets(subnetParams)
	Expect(err).ToNot(HaveOccurred())

	Expect(resp.Subnets).To(HaveLen(1), fmt.Sprintf("Expected Subnet '%s' to exist, but it does not", subnetID))
	Expect(*resp.Subnets[0].SubnetId).To(Equal(subnetID))
}

func (a AWSVerifier) ExpectSubnetToNotExist(subnetID string) {
	subnetParams := &awsec2.DescribeSubnetsInput{
		SubnetIds: []*string{
			aws.String(subnetID),
		},
	}
	_, err := a.ec2.DescribeSubnets(subnetParams)
	Expect(err).To(HaveOccurred())
	ec2err := err.(awserr.Error)

	Expect(ec2err.Code()).To(Equal("InvalidSubnetID.NotFound"), fmt.Sprintf("Expected Subnet '%s' to not exist, but it does", subnetID))
}

func (a AWSVerifier) ExpectSubnetToHaveTags(subnetID string, expectedTags map[string]string) {
	subnetParams := &awsec2.DescribeSubnetsInput{
		SubnetIds: []*string{
			aws.String(subnetID),
		},
	}
	resp, err := a.ec2.DescribeSubnets(subnetParams)
	Expect(err).ToNot(HaveOccurred())

	Expect(resp.Subnets).To(HaveLen(1))
	Expect(*resp.Subnets[0].SubnetId).To(Equal(subnetID))

	tags := resp.Subnets[0].Tags

	for key, value := range expectedTags {
		Expect(tags).To(ContainElement(&awsec2.Tag{
			Key:   &key,
			Value: &value,
		}))
	}
}

func (a AWSVerifier) DeleteSubnet(subnetID string) {
	subnetParams := &awsec2.DeleteSubnetInput{
		SubnetId: aws.String(subnetID),
	}
	_, err := a.ec2.DeleteSubnet(subnetParams)
	Expect(err).ToNot(HaveOccurred())
}

func (a AWSVerifier) ExpectSubnetWithCIDRToExist(cidr string, vpcID string) {
	subnetParams := &awsec2.DescribeSubnetsInput{
		Filters: []*awsec2.Filter{
			&awsec2.Filter{
				Name:   aws.String("cidrBlock"),
				Values: []*string{aws.String(cidr)},
			},
			&awsec2.Filter{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcID)},
			},
		},
	}
	resp, err := a.ec2.DescribeSubnets(subnetParams)
	Expect(err).ToNot(HaveOccurred())

	Expect(len(resp.Subnets)).To(Equal(1), "Unexpected number of subnets with CIDR")
}

func (a AWSVerifier) ExpectSubnetWithCIDRToNotExist(cidr string, vpcID string) {
	subnetParams := &awsec2.DescribeSubnetsInput{
		Filters: []*awsec2.Filter{
			&awsec2.Filter{
				Name:   aws.String("cidrBlock"),
				Values: []*string{aws.String(cidr)},
			},
			&awsec2.Filter{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcID)},
			},
		},
	}
	resp, err := a.ec2.DescribeSubnets(subnetParams)
	Expect(err).ToNot(HaveOccurred())

	Expect(len(resp.Subnets)).To(BeZero(), "Expected subnet with CIDR %s to not exist", cidr)
}

func (a AWSVerifier) DeleteSubnetWithCIDR(cidr string, vpcID string) {
	subnetParams := &awsec2.DescribeSubnetsInput{
		Filters: []*awsec2.Filter{
			&awsec2.Filter{
				Name:   aws.String("cidrBlock"),
				Values: []*string{aws.String(cidr)},
			},
			&awsec2.Filter{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcID)},
			},
		},
	}
	resp, err := a.ec2.DescribeSubnets(subnetParams)
	Expect(err).ToNot(HaveOccurred())

	if len(resp.Subnets) == 0 {
		return // nothing to delete
	}
	Expect(len(resp.Subnets)).ToNot(
		BeNumerically(">", 1),
		"Failed to delete subnet. Filter matched more than one subnet.",
	)

	a.DeleteSubnet(*resp.Subnets[0].SubnetId)
}
