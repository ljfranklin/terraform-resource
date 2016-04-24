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
		checkInput      models.InRequest
		bucket          string
		pathToS3Fixture string
		awsVerifier     *helpers.AWSVerifier
	)

	BeforeEach(func() {
		accessKey := os.Getenv("AWS_ACCESS_KEY")
		Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")

		secretKey := os.Getenv("AWS_SECRET_KEY")
		Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")

		bucket = os.Getenv("AWS_BUCKET")
		Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")

		bucketPath := os.Getenv("AWS_BUCKET_PATH") // optional

		region := os.Getenv("AWS_REGION") // optional
		if region == "" {
			region = "us-east-1"
		}

		awsVerifier = helpers.NewAWSVerifier(
			accessKey,
			secretKey,
			region,
		)
		pathToS3Fixture = path.Join(bucketPath, randomString("s3-test-fixture"))

		checkInput = models.InRequest{
			Source: models.Source{
				Storage: storage.Model{
					Bucket:          bucket,
					Key:             pathToS3Fixture,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
				},
			},
		}
	})

	AfterEach(func() {
		awsVerifier.DeleteObjectFromS3(bucket, pathToS3Fixture)
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

	Context("when bucket contains state file", func() {
		BeforeEach(func() {

			fixture, err := os.Open(getFileLocation("fixtures/s3/terraform.tfstate"))
			Expect(err).ToNot(HaveOccurred())
			defer fixture.Close()

			awsVerifier.UploadObjectToS3(bucket, pathToS3Fixture, fixture)
		})

		It("returns the version of the fixture on S3", func() {
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

			lastModified := awsVerifier.GetLastModifiedFromS3(bucket, pathToS3Fixture)
			md5 := awsVerifier.GetMD5FromS3(bucket, pathToS3Fixture)

			expectOutput := []storage.Version{
				storage.Version{
					LastModified: lastModified,
					MD5:          md5,
				},
			}
			Expect(actualOutput).To(Equal(expectOutput))
		})

		It("returns an empty version list when current version matches storage version", func() {
			currentLastModified := awsVerifier.GetLastModifiedFromS3(bucket, pathToS3Fixture)
			checkInput.Version = storage.Version{
				LastModified: currentLastModified,
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

	Context("when key is omitted from source", func() {
		BeforeEach(func() {
			checkInput.Source.Storage.Key = ""
		})

		It("returns an empty list of versions", func() {
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
