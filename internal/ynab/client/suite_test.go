package client_test

import (
	"testing"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClient(t *testing.T) {
	t.Parallel()
	BeforeSuite(func() {
		httpmock.Activate()
	})

	AfterSuite(func() {
		httpmock.DeactivateAndReset()
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}
