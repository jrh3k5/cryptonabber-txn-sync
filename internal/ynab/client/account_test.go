package client_test

import (
	"context"
	"net/http"

	"github.com/jarcoal/httpmock"
	clientpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetAccounts", func() {
	It("returns parsed accounts and sends Authorization header", func() {
		respBody := `{"data":{"accounts":[{"id":"a1","name":"Checking"}]}}`

		httpmock.RegisterResponder(
			"GET",
			"https://api.ynab.com/v1/budgets/budget1/accounts",
			func(req *http.Request) (*http.Response, error) {
				Expect(req.Header.Get("Authorization")).To(Equal("Bearer tokengoeshere"))

				return httpmock.NewStringResponse(http.StatusOK, respBody), nil
			},
		)

		accounts, err := clientpkg.GetAccounts(
			context.Background(),
			http.DefaultClient,
			"tokengoeshere",
			"budget1",
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(accounts).To(HaveLen(1))
		Expect(accounts[0].ID).To(Equal("a1"))
		Expect(accounts[0].Name).To(Equal("Checking"))
	})

	It("returns an error on non-200 response", func() {
		httpmock.RegisterResponder(
			"GET",
			"https://api.ynab.com/v1/budgets/budget1/accounts",
			httpmock.NewStringResponder(http.StatusInternalServerError, ""),
		)

		_, err := clientpkg.GetAccounts(
			context.Background(),
			http.DefaultClient,
			"tokengoeshere",
			"budget1",
		)
		Expect(err).To(HaveOccurred())
	})
})
