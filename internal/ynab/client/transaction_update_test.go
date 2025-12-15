package client_test

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/jarcoal/httpmock"
	clientpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MarkTransactionClearedAndAppendMemo", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("appends the hash when not present and marks cleared", func() {
		getResp := `{"data":{"transaction":{"id":"tx1","memo":"original memo","cleared":"uncleared"}}}`

		httpmock.RegisterResponder(
			"GET",
			"https://api.ynab.com/v1/budgets/budget1/transactions/tx1",
			func(req *http.Request) (*http.Response, error) {
				Expect(req.Header.Get("Authorization")).To(Equal("Bearer tokengoeshere"))

				return httpmock.NewStringResponse(200, getResp), nil
			},
		)

		var sawPut bool
		httpmock.RegisterResponder(
			"PUT",
			"https://api.ynab.com/v1/budgets/budget1/transactions/tx1",
			func(req *http.Request) (*http.Response, error) {
				Expect(req.Header.Get("Authorization")).To(Equal("Bearer tokengoeshere"))
				Expect(req.Header.Get("Content-Type")).To(Equal("application/json"))

				body, _ := io.ReadAll(req.Body)
				s := string(body)
				Expect(s).To(ContainSubstring(`"memo":"original memo; transaction hash:`))
				Expect(s).To(ContainSubstring(`txhash123"`))
				Expect(s).To(ContainSubstring(`"cleared":"cleared"`))

				sawPut = true

				return httpmock.NewStringResponse(200, `{"data":{}}`), nil
			},
		)

		err := clientpkg.MarkTransactionClearedAndAppendMemo(
			ctx,
			http.DefaultClient,
			"tokengoeshere",
			"budget1",
			"tx1",
			"txhash123",
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(sawPut).To(BeTrue())
	})

	It("does not duplicate the hash if already present", func() {
		getResp := `{"data":{"transaction":{"id":"tx1","memo":"original memo txhash123","cleared":"uncleared"}}}`

		httpmock.RegisterResponder(
			"GET",
			"https://api.ynab.com/v1/budgets/budget1/transactions/tx1",
			func(req *http.Request) (*http.Response, error) {
				return httpmock.NewStringResponse(200, getResp), nil
			},
		)

		httpmock.RegisterResponder(
			"PUT",
			"https://api.ynab.com/v1/budgets/budget1/transactions/tx1",
			func(req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				s := string(body)
				// should only contain one instance of the hash
				Expect(strings.Count(s, "txhash123")).To(Equal(1))

				return httpmock.NewStringResponse(200, `{"data":{}}`), nil
			},
		)

		err := clientpkg.MarkTransactionClearedAndAppendMemo(
			ctx,
			http.DefaultClient,
			"tokengoeshere",
			"budget1",
			"tx1",
			"txhash123",
		)
		Expect(err).ToNot(HaveOccurred())
	})

	It("returns an error when GET returns non-200", func() {
		httpmock.RegisterResponder(
			"GET",
			"https://api.ynab.com/v1/budgets/budget1/transactions/tx1",
			httpmock.NewStringResponder(http.StatusInternalServerError, ""),
		)

		err := clientpkg.MarkTransactionClearedAndAppendMemo(
			ctx,
			http.DefaultClient,
			"tokengoeshere",
			"budget1",
			"tx1",
			"txhash123",
		)
		Expect(err).To(HaveOccurred())
	})
})
