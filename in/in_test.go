package main_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("In", func() {

	var (
		awsVerifier         *helpers.AWSVerifier
		inReq               models.InRequest
		bucket              string
		pathToPrevS3Fixture string
		pathToCurrS3Fixture string
		tmpDir              string
	)

	BeforeEach(func() {
		accessKey := os.Getenv("AWS_ACCESS_KEY")
		Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")

		secretKey := os.Getenv("AWS_SECRET_KEY")
		Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")

		bucket = os.Getenv("AWS_BUCKET")
		Expect(bucket).ToNot(BeEmpty(), "AWS_BUCKET must be set")

		bucketPath := os.Getenv("AWS_BUCKET_PATH")
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

		inReq = models.InRequest{
			Source: models.Source{
				Storage: storage.Model{
					Bucket:          bucket,
					BucketPath:      bucketPath,
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
				},
			},
		}

		var err error
		tmpDir, err = ioutil.TempDir("", "terraform-resource-in-test")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Context("when multiple state files exist on S3", func() {
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

		AfterEach(func() {
			awsVerifier.DeleteObjectFromS3(bucket, pathToPrevS3Fixture)
			awsVerifier.DeleteObjectFromS3(bucket, pathToCurrS3Fixture)
		})

		It("fetches the state file matching the provided version", func() {

			inReq.Version = storage.Version{
				LastModified: awsVerifier.GetLastModifiedFromS3(bucket, pathToPrevS3Fixture),
				StateFileKey: pathToPrevS3Fixture,
			}

			command := exec.Command(pathToInBinary, tmpDir)

			stdin, err := command.StdinPipe()
			Expect(err).ToNot(HaveOccurred())

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			err = json.NewEncoder(stdin).Encode(inReq)
			Expect(err).ToNot(HaveOccurred())
			stdin.Close()

			Eventually(session, 30*time.Second).Should(gexec.Exit(0))

			actualOutput := models.InResponse{}
			err = json.Unmarshal(session.Out.Contents(), &actualOutput)
			Expect(err).ToNot(HaveOccurred())

			_, err = time.Parse(storage.TimeFormat, actualOutput.Version.LastModified)
			Expect(err).ToNot(HaveOccurred())
			Expect(actualOutput.Version.StateFileKey).To(Equal(pathToPrevS3Fixture))

			expectedOutputPath := path.Join(tmpDir, "metadata")
			Expect(expectedOutputPath).To(BeAnExistingFile())
			outputFile, err := os.Open(expectedOutputPath)
			Expect(err).ToNot(HaveOccurred())
			defer outputFile.Close()

			outputContents := map[string]interface{}{}
			err = json.NewDecoder(outputFile).Decode(&outputContents)
			Expect(err).ToNot(HaveOccurred())

			Expect(outputContents["vpc_id"]).ToNot(BeNil())
			Expect(outputContents["tag_name"]).To(Equal("previous"))
		})
	})

	Context("when state file does not exist on S3", func() {

		Context("and it was called as part of the 'destroy' action", func() {

			BeforeEach(func() {
				inReq.Params.Action = models.DestroyAction
				inReq.Version = storage.Version{
					LastModified: time.Now().UTC().Format(storage.TimeFormat),
					StateFileKey: pathToCurrS3Fixture,
				}
			})

			It("returns the deleted version, but does not create the metadata file", func() {

				command := exec.Command(pathToInBinary, tmpDir)

				stdin, err := command.StdinPipe()
				Expect(err).ToNot(HaveOccurred())

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				err = json.NewEncoder(stdin).Encode(inReq)
				Expect(err).ToNot(HaveOccurred())
				stdin.Close()

				Eventually(session).Should(gexec.Exit(0))

				actualOutput := models.InResponse{}
				err = json.Unmarshal(session.Out.Contents(), &actualOutput)
				Expect(err).ToNot(HaveOccurred())

				_, err = time.Parse(storage.TimeFormat, actualOutput.Version.LastModified)
				Expect(err).ToNot(HaveOccurred())

				expectedOutputPath := path.Join(tmpDir, "metadata")
				Expect(expectedOutputPath).ToNot(BeAnExistingFile())
			})
		})

		Context("and it was called as part of update or create", func() {
			BeforeEach(func() {
				inReq.Params.Action = ""
				inReq.Version = storage.Version{
					LastModified: time.Now().UTC().Format(storage.TimeFormat),
					StateFileKey: "fake-path-to-state-file",
				}
			})

			It("returns an error", func() {
				command := exec.Command(pathToInBinary, tmpDir)

				stdin, err := command.StdinPipe()
				Expect(err).ToNot(HaveOccurred())

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				err = json.NewEncoder(stdin).Encode(inReq)
				Expect(err).ToNot(HaveOccurred())
				stdin.Close()

				Eventually(session).Should(gexec.Exit())
				Expect(session.ExitCode()).ToNot(BeZero())
				Expect(session.Err).To(gbytes.Say("StateFile does not exist"))
			})
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
