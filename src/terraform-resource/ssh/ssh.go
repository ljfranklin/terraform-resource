package ssh

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	sshPkg "golang.org/x/crypto/ssh/agent"
)

type Agent struct {
	agent          sshPkg.Agent
	socket         string
	serverListener net.Listener
}

func SpawnAgent() (*Agent, error) {
	agent := sshPkg.NewKeyring()

	socketPath, err := tempFilepath("ssh-agent")
	if err != nil {
		return nil, err
	}
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			fd, err := l.Accept()
			if err != nil {
				break
			}
			go func() { _ = sshPkg.ServeAgent(agent, fd) }()
		}
	}()

	return &Agent{
		agent:          agent,
		socket:         socketPath,
		serverListener: l,
	}, nil
}

func (a *Agent) AddKey(pemEncodedKey []byte) error {
	privateKey, err := ssh.ParseRawPrivateKey(pemEncodedKey)
	if err != nil {
		return fmt.Errorf("failed to parse key: %s", err)
	}
	if err = a.agent.Add(sshPkg.AddedKey{PrivateKey: privateKey}); err != nil {
		return err
	}
	return nil
}

func (a *Agent) SSHAuthSock() string {
	return a.socket
}

func (a *Agent) Shutdown() error {
	return a.serverListener.Close()
}

func tempFilepath(prefix string) (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return filepath.Join(os.TempDir(), prefix+base64.URLEncoding.EncodeToString(b)), nil
}
