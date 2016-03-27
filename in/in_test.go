package main_test

import (
	"encoding/json"
	"os/exec"

	"github.com/ljfranklin/terraform-resource/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("In", func() {

	It("writes version and metadata to stdout", func() {
		command := exec.Command(pathToCheckBinary)

		stdin, err := command.StdinPipe()
		Expect(err).ToNot(HaveOccurred())

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		input := models.InRequest{
			Source: models.Source{},
			Version: models.Version{
				Version: "1.1.1",
			},
		}
		err = json.NewEncoder(stdin).Encode(input)
		Expect(err).ToNot(HaveOccurred())
		stdin.Close()

		Eventually(session).Should(gexec.Exit(0))

		actualOutput := models.InResponse{}
		err = json.Unmarshal(session.Out.Contents(), &actualOutput)
		Expect(err).ToNot(HaveOccurred())

		expectedOutput := models.InResponse{
			Version: models.Version{
				Version: "1.1.1",
			},
			Metadata: []models.MetadataField{},
		}

		Expect(actualOutput).To(Equal(expectedOutput))
	})
})
