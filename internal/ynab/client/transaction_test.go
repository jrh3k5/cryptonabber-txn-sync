package client_test

import (
	"context"
	"net/http"
	"time"

	"github.com/jarcoal/httpmock"
	clientpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetTransactions", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("returns parsed transactions on success and sets since_date", func() {
		since := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)

		respBody := `{"data":{"transactions":[{"id":"tx1","amount":1000,"date":"2025-12-01","memo":"test memo","cleared":"uncleared"}]}}`

		httpmock.RegisterResponder(
			"GET",
			"https://api.ynab.com/v1/budgets/budget1/accounts/acct1/transactions",
			func(req *http.Request) (*http.Response, error) {
				Expect(req.URL.Query().Get("since_date")).To(Equal("2025-12-01"))
				Expect(req.Header.Get("Authorization")).To(Equal("Bearer tokengoeshere"))

				return httpmock.NewStringResponse(200, respBody), nil
			},
		)

		txns, err := clientpkg.GetTransactions(ctx, http.DefaultClient, "tokengoeshere", "budget1", "acct1", since)
		Expect(err).ToNot(HaveOccurred())
		Expect(txns).To(HaveLen(1))

		txn := txns[0]
		Expect(txn.ID).To(Equal("tx1"))
		Expect(txn.Amount).To(Equal(int64(1000)))
		Expect(txn.Description).To(Equal("test memo"))
		Expect(txn.Cleared).To(BeFalse())
		Expect(txn.Date.Year()).To(Equal(2025))
		Expect(txn.Date.Month()).To(Equal(time.December))
		Expect(txn.Date.Day()).To(Equal(1))
	})

	It("returns an error on non-200 response", func() {
		httpmock.RegisterResponder(
			"GET",
			"https://api.ynab.com/v1/budgets/budget1/accounts/acct1/transactions",
			func(req *http.Request) (*http.Response, error) {
				Expect(req.Header.Get("Authorization")).To(Equal("Bearer tokengoeshere"))

				return httpmock.NewStringResponse(http.StatusInternalServerError, ""), nil
			},
		)

		_, err := clientpkg.GetTransactions(
			ctx,
			http.DefaultClient,
			"tokengoeshere",
			"budget1",
			"acct1",
			time.Time{},
		)
		Expect(err).To(HaveOccurred())
	})
})
