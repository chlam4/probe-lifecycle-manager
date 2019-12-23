package t8c_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestProbeController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Probe Controller Suite")
}
