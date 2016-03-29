package main_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Out", func() {

	var terraformSource string

	assertOutLifecycle := func() {
		It("succeeds in creating, outputing, and deleting infrastructure", func() {

			accessKey := os.Getenv("AWS_ACCESS_KEY")
			Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")

			secretKey := os.Getenv("AWS_SECRET_KEY")
			Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")

			bucket := os.Getenv("AWS_BUCKET")
			Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")

			bucketPath := os.Getenv("AWS_BUCKET_PATH") // optional

			region := os.Getenv("AWS_REGION") // optional
			if region == "" {
				region = "us-east-1"
			}

			awsConfig := &aws.Config{
				Region:      aws.String(region),
				Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
			}
			ec2 := awsec2.New(awsSession.New(awsConfig))
			s3 := storage.NewS3(accessKey, secretKey, region, bucket)

			pathToSources := getProjectRoot()

			stateFileKey := path.Join(bucketPath, randomString("out-test"))

			version, err := s3.Version(stateFileKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(version).To(BeEmpty())

			command := exec.Command(pathToOutBinary, pathToSources)

			stdin, err := command.StdinPipe()
			Expect(err).ToNot(HaveOccurred())

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			input := models.OutRequest{
				Source: models.Source{
					Bucket:          bucket,
					Key:             stateFileKey,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
				},
				Params: models.Params{
					"terraform_source": terraformSource,
					"access_key":       accessKey,
					"secret_key":       secretKey,
				},
			}
			err = json.NewEncoder(stdin).Encode(input)
			Expect(err).ToNot(HaveOccurred())
			stdin.Close()

			Eventually(session, 2*time.Minute).Should(gexec.Exit(0))

			version, err = s3.Version(stateFileKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(version).ToNot(BeEmpty(), fmt.Sprintf("Failed to find state file at %s", stateFileKey))

			actualOutput := models.OutResponse{}
			err = json.Unmarshal(session.Out.Contents(), &actualOutput)
			Expect(err).ToNot(HaveOccurred())

			// does version match format "2006-01-02T15:04:05Z"
			_, err = time.Parse(time.RFC3339, actualOutput.Version.Version)
			Expect(err).ToNot(HaveOccurred())

			Expect(actualOutput.Metadata).ToNot(BeEmpty())
			vpcID := ""
			for _, field := range actualOutput.Metadata {
				if field.Name == "vpc_id" {
					vpcID = field.Value.(string)
					break
				}
			}
			Expect(vpcID).ToNot(BeEmpty())

			vpcParams := &awsec2.DescribeVpcsInput{
				VpcIds: []*string{
					aws.String(vpcID),
				},
			}
			resp, err := ec2.DescribeVpcs(vpcParams)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.Vpcs).To(HaveLen(1))
			Expect(*resp.Vpcs[0].VpcId).To(Equal(vpcID))
		})
	}

	Context("when provided a local terraform source", func() {
		BeforeEach(func() {
			terraformSource = "fixtures/aws/"
		})

		assertOutLifecycle()
	})

	Context("when provided a remote terraform source", func() {
		BeforeEach(func() {
			// changes to fixture must be pushed before running this test
			terraformSource = "github.com/ljfranklin/terraform-resource//fixtures/aws/"
		})

		assertOutLifecycle()
	})
})

func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(1)
	return path.Join(path.Dir(filename), "..")
}

func randomString(prefix string) string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	Expect(err).ToNot(HaveOccurred())
	return fmt.Sprintf("%s-%x", prefix, b)
}
