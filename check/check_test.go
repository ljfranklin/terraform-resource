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

	"github.com/ljfranklin/terraform-resource/models"
	"github.com/ljfranklin/terraform-resource/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Check", func() {

	var (
		s3              storage.Storage
		checkInput      models.InRequest
		pathToS3Fixture string
	)

	BeforeEach(func() {
		accessKey := os.Getenv("AWS_ACCESS_KEY")
		Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")

		secretKey := os.Getenv("AWS_SECRET_KEY")
		Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")

		bucket := os.Getenv("AWS_BUCKET")
		Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")

		bucketPath := os.Getenv("AWS_BUCKET_PATH") // optional

		region := os.Getenv("AWS_REGION") // optional

		s3 = storage.NewS3(accessKey, secretKey, region, bucket)

		pathToS3Fixture = path.Join(bucketPath, randomString("s3-test-fixture"))

		fixture, err := os.Open(getFileLocation("fixtures/s3/terraform.tfstate"))
		Expect(err).ToNot(HaveOccurred())
		defer fixture.Close()

		err = s3.Upload(pathToS3Fixture, fixture)
		Expect(err).ToNot(HaveOccurred())

		checkInput = models.InRequest{
			Source: models.Source{
				Bucket:          bucket,
				Key:             pathToS3Fixture,
				AccessKeyID:     accessKey,
				SecretAccessKey: secretKey,
			},
		}
	})

	AfterEach(func() {
		_ = s3.Delete(pathToS3Fixture) // ignore error on cleanup
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

		actualOutput := []models.Version{}
		err = json.Unmarshal(session.Out.Contents(), &actualOutput)
		Expect(err).ToNot(HaveOccurred())

		version, err := s3.Version(pathToS3Fixture)
		Expect(err).ToNot(HaveOccurred())
		_, err = time.Parse(time.RFC3339, version) // does version match format "2006-01-02T15:04:05Z"
		Expect(err).ToNot(HaveOccurred())

		expectOutput := []models.Version{
			models.Version{
				Version: version,
			},
		}
		Expect(actualOutput).To(Equal(expectOutput))
	})

	Context("when key is omitted from source", func() {
		BeforeEach(func() {
			checkInput.Source.Key = ""
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

			actualOutput := []models.Version{}
			err = json.Unmarshal(session.Out.Contents(), &actualOutput)
			Expect(err).ToNot(HaveOccurred())

			expectOutput := []models.Version{}
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
