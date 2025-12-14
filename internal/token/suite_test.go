package token_test

import (
	"testing"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRPCDetailsService(t *testing.T) {
	t.Parallel()

	BeforeSuite(func() {
		httpmock.Activate()
	})

	AfterSuite(func() {
		httpmock.DeactivateAndReset()
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, "RPCDetailsService Suite")
}
