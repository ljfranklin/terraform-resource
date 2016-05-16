package namer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNamer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Namer Suite")
}
