package token_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jarcoal/httpmock"
	tokenpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("rpcDetailsService", func() {
	const rpcURL = "http://example.local"

	It("returns decimals on successful response", func() {
		res := `{"jsonrpc":"2.0","id":1,"result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`
		httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

		svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
		d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
		Expect(err).ToNot(HaveOccurred())
		Expect(d).ToNot(BeNil())
		Expect(d.Decimals).To(Equal(18))
		httpmock.Reset()
	})

	When("result is 0x", func() {
		It("returns nil", func() {
			res := `{"jsonrpc":"2.0","id":1,"result":"0x"}`
			httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

			svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
			d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
			Expect(err).ToNot(HaveOccurred())
			Expect(d).To(BeNil())
			httpmock.Reset()
		})
	})

	When("rpc response has an error field", func() {
		It("returns an error", func() {
			res := `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`
			httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

			svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
			d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
			Expect(d).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("rpc error"))
			httpmock.Reset()
		})
	})

	When("result contains invalid hex", func() {
		It("returns an error", func() {
			res := `{"jsonrpc":"2.0","id":1,"result":"0xzz"}`
			httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

			svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
			d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
			Expect(d).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("decode hex result"))
			httpmock.Reset()
		})
	})

	It("verifies JSON-RPC request payload contains correct method selector and to address", func() {
		contract := "0xdeadbeef"
		httpmock.RegisterResponder("POST", rpcURL, func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			Expect(payload["method"]).To(Equal("eth_call"))
			params, ok := payload["params"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(params)).To(BeNumerically(">=", 1))
			callObj, ok := params[0].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(callObj["to"]).To(Equal(contract))
			Expect(callObj["data"]).To(Equal("0x313ce567"))
			return httpmock.NewStringResponse(200, `{"jsonrpc":"2.0","id":1,"result":"0x12"}`), nil
		})

		svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
		d, err := svc.GetTokenDetails(context.Background(), contract)
		Expect(err).ToNot(HaveOccurred())
		Expect(d).ToNot(BeNil())
		Expect(d.Decimals).To(Equal(18))
		httpmock.Reset()
	})

	When("decimals value exceeds max int", func() {
		It("returns an error", func() {
			// 0x8000000000000000 is 2^63 which exceeds int64 max on 64-bit
			res := `{"jsonrpc":"2.0","id":1,"result":"0x8000000000000000"}`
			httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

			svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
			d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
			Expect(d).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("decimals value too large"))
			httpmock.Reset()
		})
	})

	When("HTTP response is non-200", func() {
		It("returns an error", func() {
			httpmock.RegisterResponder(
				"POST",
				rpcURL,
				httpmock.NewStringResponder(500, "internal server error"),
			)
			svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
			d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
			Expect(d).To(BeNil())
			Expect(err).To(HaveOccurred())
			httpmock.Reset()
		})
	})

	When("there is a network error from the HTTP client", func() {
		It("returns an error", func() {
			httpmock.RegisterResponder(
				"POST",
				rpcURL,
				httpmock.NewErrorResponder(fmt.Errorf("network error")),
			)
			svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
			d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
			Expect(d).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("rpc call"))
			httpmock.Reset()
		})
	})

	DescribeTable("various hex results", func(resultHex string, expectedDecimals int) {
		httpmock.RegisterResponder(
			"POST",
			rpcURL,
			httpmock.NewStringResponder(
				200,
				`{"jsonrpc":"2.0","id":1,"result":"`+resultHex+`"}`,
			),
		)
		svc := tokenpkg.NewRPCDetailsService(client, rpcURL)
		d, err := svc.GetTokenDetails(context.Background(), "0xdeadbeef")
		Expect(err).ToNot(HaveOccurred())
		Expect(d).ToNot(BeNil())
		Expect(d.Decimals).To(Equal(expectedDecimals))
		httpmock.Reset()
	},
		Entry("single byte", "0x12", 18),
		Entry("padded two bytes", "0x0012", 18),
		Entry(
			"padded 32 bytes",
			"0x0000000000000000000000000000000000000000000000000000000000000014",
			20,
		),
	)
})
