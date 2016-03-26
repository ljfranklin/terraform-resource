package main_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Check", func() {

	It("returns an empty list of versions", func() {
		command := exec.Command(pathToCheckBinary)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))

		stdout := session.Out.Contents()
		Expect(stdout).To(Equal([]byte("[]")))
	})
})
