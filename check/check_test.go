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

	"github.com/ljfranklin/terraform-resource/in/models"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/test/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Check", func() {

	var (
		checkInput          models.InRequest
		bucket              string
		pathToPrevS3Fixture string
		pathToCurrS3Fixture string
		awsVerifier         *helpers.AWSVerifier
	)

	BeforeEach(func() {
		accessKey := os.Getenv("AWS_ACCESS_KEY")
		Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")

		secretKey := os.Getenv("AWS_SECRET_KEY")
		Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")

		bucket = os.Getenv("AWS_BUCKET")
		Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")

		bucketPath := os.Getenv("AWS_BUCKET_PATH") // optional
		Expect(bucketPath).ToNot(BeEmpty(), "AWS_BUCKET_PATH must be set")

		region := os.Getenv("AWS_REGION") // optional
		if region == "" {
			region = "us-east-1"
		}

		awsVerifier = helpers.NewAWSVerifier(
			accessKey,
			secretKey,
			region,
		)
		pathToPrevS3Fixture = path.Join(bucketPath, randomString("s3-test-fixture-previous"))
		pathToCurrS3Fixture = path.Join(bucketPath, randomString("s3-test-fixture-current"))

		checkInput = models.InRequest{
			Source: models.Source{
				Storage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
				},
			},
		}
	})

	AfterEach(func() {
		awsVerifier.DeleteObjectFromS3(bucket, pathToPrevS3Fixture)
		awsVerifier.DeleteObjectFromS3(bucket, pathToCurrS3Fixture)
	})

	Context("when bucket is empty", func() {
		It("returns an empty version list", func() {
			command := exec.Command(pathToCheckBinary)

			stdin, err := command.StdinPipe()
			Expect(err).ToNot(HaveOccurred())

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			err = json.NewEncoder(stdin).Encode(checkInput)
			Expect(err).ToNot(HaveOccurred())
			stdin.Close()

			Eventually(session, 15*time.Second).Should(gexec.Exit(0))

			actualOutput := []storage.Version{}
			err = json.Unmarshal(session.Out.Contents(), &actualOutput)
			Expect(err).ToNot(HaveOccurred())

			expectedOutput := []storage.Version{}
			Expect(actualOutput).To(Equal(expectedOutput))
		})
	})

	Context("when bucket contains multiple state files", func() {
		BeforeEach(func() {
			prevFixture, err := os.Open(getFileLocation("fixtures/s3/terraform-previous.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer prevFixture.Close()

			awsVerifier.UploadObjectToS3(bucket, pathToPrevS3Fixture, prevFixture)
			time.Sleep(5 * time.Second) // ensure last modified is different

			currFixture, err := os.Open(getFileLocation("fixtures/s3/terraform-current.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer currFixture.Close()
			awsVerifier.UploadObjectToS3(bucket, pathToCurrS3Fixture, currFixture)
		})

		It("returns the latest version on S3", func() {
			command := exec.Command(pathToCheckBinary)

			stdin, err := command.StdinPipe()
			Expect(err).ToNot(HaveOccurred())

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			err = json.NewEncoder(stdin).Encode(checkInput)
			Expect(err).ToNot(HaveOccurred())
			stdin.Close()

			Eventually(session, 15*time.Second).Should(gexec.Exit(0))

			actualOutput := []storage.Version{}
			err = json.Unmarshal(session.Out.Contents(), &actualOutput)
			Expect(err).ToNot(HaveOccurred())

			lastModified := awsVerifier.GetLastModifiedFromS3(bucket, pathToCurrS3Fixture)

			expectOutput := []storage.Version{
				storage.Version{
					LastModified: lastModified,
					StateFileKey: pathToCurrS3Fixture,
				},
			}
			Expect(actualOutput).To(Equal(expectOutput))
		})

		It("returns an empty version list when current version matches storage version", func() {
			currentLastModified := awsVerifier.GetLastModifiedFromS3(bucket, pathToCurrS3Fixture)
			checkInput.Version = storage.Version{
				LastModified: currentLastModified,
				StateFileKey: pathToCurrS3Fixture,
			}

			command := exec.Command(pathToCheckBinary)

			stdin, err := command.StdinPipe()
			Expect(err).ToNot(HaveOccurred())

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			err = json.NewEncoder(stdin).Encode(checkInput)
			Expect(err).ToNot(HaveOccurred())
			stdin.Close()

			Eventually(session, 15*time.Second).Should(gexec.Exit(0))

			actualOutput := []storage.Version{}
			err = json.Unmarshal(session.Out.Contents(), &actualOutput)
			Expect(err).ToNot(HaveOccurred())

			expectOutput := []storage.Version{}
			Expect(actualOutput).To(Equal(expectOutput))
		})
	})
})

func randomString(prefix string) string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	Expect(err).ToNot(HaveOccurred())
	return fmt.Sprintf("%s-%x", prefix, b)
}

func getFileLocation(relativePath string) string {
	_, filename, _, _ := runtime.Caller(1)
	return path.Join(path.Dir(filename), "..", relativePath)
}
