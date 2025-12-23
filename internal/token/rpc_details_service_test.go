package token_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/jarcoal/httpmock"
	tokenpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("rpcDetailsService", func() {
	const rpcURL = "http://example.local"
	var detailsService *tokenpkg.RPCDetailsService

	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()

		detailsService = tokenpkg.NewRPCDetailsService(http.DefaultClient, rpcURL)
	})

	It("returns decimals on successful response", func() {
		res := `{"jsonrpc":"2.0","id":1,"result":"0x0000000000000000000000000000000000000000000000000000000000000012"}`
		httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

		tokenDetails, err := detailsService.GetTokenDetails(ctx, "0xdeadbeef")
		Expect(err).ToNot(HaveOccurred())
		Expect(tokenDetails).ToNot(BeNil())
		Expect(tokenDetails.Decimals).To(Equal(18))
	})

	It("returns the token name on successful name() response", func() {
		// ERC20 name() returns: offset (32 bytes), length (32 bytes), then utf-8 bytes ("USD Coin")
		// offset: 0x20, length: 0x08, value: "USD Coin"
		// 0000000000000000000000000000000000000000000000000000000000000020 (offset)
		// 0000000000000000000000000000000000000000000000000000000000000008 (length)
		// 55534420436f696e ("USD Coin")
		nameHex := "0x" +
			"0000000000000000000000000000000000000000000000000000000000000020" +
			"0000000000000000000000000000000000000000000000000000000000000008" +
			"55534420436f696e"
		decimalsHex := "0x0000000000000000000000000000000000000000000000000000000000000006"
		callCount := 0
		httpmock.RegisterResponder("POST", rpcURL, func(req *http.Request) (*http.Response, error) {
			callCount++
			if callCount == 1 {
				return httpmock.NewStringResponse(
					200,
					`{"jsonrpc":"2.0","id":1,"result":"`+decimalsHex+`"}`,
				), nil
			}

			return httpmock.NewStringResponse(
				200,
				`{"jsonrpc":"2.0","id":2,"result":"`+nameHex+`"}`,
			), nil
		})

		tokenDetails, err := detailsService.GetTokenDetails(ctx, "0xdeadbeef")
		Expect(err).ToNot(HaveOccurred())
		Expect(tokenDetails).ToNot(BeNil())
		Expect(tokenDetails.Decimals).To(Equal(6))
		Expect(tokenDetails.Name).To(Equal("USD Coin"))
	})

	When("result is 0x", func() {
		It("returns nil", func() {
			res := `{"jsonrpc":"2.0","id":1,"result":"0x"}`
			httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

			tokenDetails, err := detailsService.GetTokenDetails(ctx, "0xdeadbeef")
			Expect(err).ToNot(HaveOccurred())
			Expect(tokenDetails).To(BeNil())
		})
	})

	When("rpc response has an error field", func() {
		It("returns an error", func() {
			res := `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`
			httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

			_, err := detailsService.GetTokenDetails(ctx, "0xdeadbeef")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("rpc error"))
		})
	})

	When("result contains invalid hex", func() {
		It("returns an error", func() {
			res := `{"jsonrpc":"2.0","id":1,"result":"0xzz"}`
			httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

			_, err := detailsService.GetTokenDetails(ctx, "0xdeadbeef")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("decode hex result"))
		})
	})

	It("verifies JSON-RPC request payload contains correct method selector and to address", func() {
		contract := "0xdeadbeef"
		httpmock.RegisterResponder("POST", rpcURL, func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			Expect(payload["method"]).To(Equal("eth_call"))
			params, ok := payload["params"].([]any)
			Expect(ok).To(BeTrue())
			Expect(params).ToNot(BeEmpty())
			callObj, ok := params[0].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(callObj["to"]).To(Equal(contract))
			Expect(callObj["data"]).To(Equal("0x313ce567"))

			return httpmock.NewStringResponse(200, `{"jsonrpc":"2.0","id":1,"result":"0x12"}`), nil
		})

		tokenDetails, err := detailsService.GetTokenDetails(ctx, contract)
		Expect(err).ToNot(HaveOccurred())
		Expect(tokenDetails).ToNot(BeNil())
		Expect(tokenDetails.Decimals).To(Equal(18))
	})

	When("decimals value exceeds max int", func() {
		It("returns an error", func() {
			// 0x8000000000000000 is 2^63 which exceeds int64 max on 64-bit
			res := `{"jsonrpc":"2.0","id":1,"result":"0x8000000000000000"}`
			httpmock.RegisterResponder("POST", rpcURL, httpmock.NewStringResponder(200, res))

			_, err := detailsService.GetTokenDetails(ctx, "0xdeadbeef")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("decimals value too large"))
		})
	})

	When("HTTP response is non-200", func() {
		It("returns an error", func() {
			httpmock.RegisterResponder(
				"POST",
				rpcURL,
				httpmock.NewStringResponder(500, "internal server error"),
			)

			_, err := detailsService.GetTokenDetails(ctx, "0xdeadbeef")
			Expect(err).To(HaveOccurred())
		})
	})

	When("there is a network error from the HTTP client", func() {
		It("returns an error", func() {
			httpmock.RegisterResponder(
				"POST",
				rpcURL,
				httpmock.NewErrorResponder(errors.New("network error")),
			)

			_, err := detailsService.GetTokenDetails(ctx, "0xdeadbeef")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("rpc call"))
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

		tokenDetails, err := detailsService.GetTokenDetails(ctx, "0xdeadbeef")
		Expect(err).ToNot(HaveOccurred())
		Expect(tokenDetails).ToNot(BeNil())
		Expect(tokenDetails.Decimals).To(Equal(expectedDecimals))
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
