package token_test

import (
	"context"

	tokenpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("rpcDetailsService", func() {
	const rpcURL = "http://example.local"


	It("returns decimals on successful response", func() {
		// return 0x12 (18) padded
		res := `{"jsonrpc":"2.0","id":1,"result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`
		httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

		svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
		d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
		Expect(err).ToNot(HaveOccurred())
		Expect(d).ToNot(BeNil())
		Expect(d.Decimals).To(Equal(18))
	})

	It("returns nil when result is 0x", func() {
		res := `{"jsonrpc":"2.0","id":1,"result":"0x"}`
		httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

		svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
		d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
		Expect(err).ToNot(HaveOccurred())
		Expect(d).To(BeNil())
	})

	It("returns error when rpc response has error field", func() {
		res := `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`
		httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

		svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
		d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
		Expect(d).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("rpc error"))
	})

	It("returns error on invalid hex result", func() {
		res := `{"jsonrpc":"2.0","id":1,"result":"0xzz"}`
		httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

		svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
		d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
		Expect(d).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("decode hex result"))
	})

	It("returns error when http client is nil", func() {
		svc := tokenpkg.NewRPCDetailsService(nil, rpcURL)
		d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
		Expect(d).To(BeNil())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("http client is nil"))
	})
})
