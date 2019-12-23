package k8s_secret_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRegistrar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registrar Suite")
}
