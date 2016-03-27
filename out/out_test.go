package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"runtime"
	"time"

	"github.com/ljfranklin/terraform-resource/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Out", func() {

	It("applies the AWS template from a local directory", func() {
		accessKey := os.Getenv("AWS_ACCESS_KEY")
		Expect(accessKey).ToNot(BeEmpty(), "AWS_ACCESS_KEY must be set")

		secretKey := os.Getenv("AWS_SECRET_KEY")
		Expect(secretKey).ToNot(BeEmpty(), "AWS_SECRET_KEY must be set")

		pathToSources := getProjectRoot()

		command := exec.Command(pathToOutBinary, pathToSources)

		stdin, err := command.StdinPipe()
		Expect(err).ToNot(HaveOccurred())

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		input := models.OutRequest{
			Params: models.Params{
				"terraform_source": "fixtures/aws/",
				"access_key":       accessKey,
				"secret_key":       secretKey,
			},
		}
		err = json.NewEncoder(stdin).Encode(input)
		Expect(err).ToNot(HaveOccurred())
		stdin.Close()

		Eventually(session, 2*time.Minute).Should(gexec.Exit(0))

		actualOutput := models.OutResponse{}
		err = json.Unmarshal(session.Out.Contents(), &actualOutput)
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
	})
})

func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(1)
	return path.Join(path.Dir(filename), "..")
}
