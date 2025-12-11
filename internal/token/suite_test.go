package token_test

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var client *http.Client

func TestRPCDetailsService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RPCDetailsService Suite")
}

var _ = BeforeSuite(func() {
	client = &http.Client{}
	httpmock.ActivateNonDefault(client)
})

var _ = AfterSuite(func() {
	httpmock.DeactivateAndReset()
})
