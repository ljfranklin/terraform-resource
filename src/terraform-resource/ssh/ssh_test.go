package ssh_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"github.com/ljfranklin/terraform-resource/ssh"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SSH", func() {
	var (
		agent *ssh.Agent
	)

	BeforeEach(func() {
		var err error
		agent, err = ssh.SpawnAgent()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		_ = agent.Shutdown()
	})

	Describe("AddKey", func() {
		It("adds a private key to the agent", func() {
			sshKey, err := ioutil.ReadFile(filepath.Join("fixtures", "ssh.pem"))
			Expect(err).To(BeNil())

			// expect ssh-add to exit non-zero due to no identities
			_, err = runSSHAdd(agent.SSHAuthSock())
			Expect(err).ToNot(BeNil())

			err = agent.AddKey(sshKey)
			Expect(err).To(BeNil())

			// expect ssh-add to exit zero as there's now an identity added
			output, err := runSSHAdd(agent.SSHAuthSock())
			Expect(err).To(BeNil(), string(output))
		})

		It("errors when given an invalid key", func() {
			err := agent.AddKey([]byte("not-a-valid-key"))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to parse key"))
		})
	})

	Describe("Shutdown", func() {
		It("cleans up the socket file and stop the listener", func() {
			Expect(agent.SSHAuthSock()).To(BeAnExistingFile())

			err := agent.Shutdown()
			Expect(err).To(BeNil())

			Expect(agent.SSHAuthSock()).ToNot(BeAnExistingFile())
		})
	})
})

func runSSHAdd(sshAuthSockPath string) ([]byte, error) {
	cmd := exec.Command("ssh-add", "-l")
	cmd.Env = []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		fmt.Sprintf("SSH_AUTH_SOCK=%s", sshAuthSockPath),
	}

	return cmd.CombinedOutput()
}
