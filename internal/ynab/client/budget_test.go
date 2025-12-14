package client_test

import (
	"context"
	"net/http"

	"github.com/jarcoal/httpmock"
	clientpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetBudgets", func() {
	It("returns parsed budgets and sends Authorization header", func() {
		respBody := `{"data":{"budgets":[{"id":"b1","name":"Main Budget"}]}}`

		httpmock.RegisterResponder(
			"GET",
			"https://api.ynab.com/v1/budgets",
			func(req *http.Request) (*http.Response, error) {
				Expect(req.Header.Get("Authorization")).To(Equal("Bearer tokengoeshere"))

				return httpmock.NewStringResponse(200, respBody), nil
			},
		)

		budgets, err := clientpkg.GetBudgets(
			context.Background(),
			http.DefaultClient,
			"tokengoeshere",
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(budgets).To(HaveLen(1))
		Expect(budgets[0].ID).To(Equal("b1"))
		Expect(budgets[0].Name).To(Equal("Main Budget"))
	})

	It("returns an error on non-200 response", func() {
		httpmock.RegisterResponder(
			"GET",
			"https://api.ynab.com/v1/budgets",
			httpmock.NewStringResponder(http.StatusInternalServerError, ""),
		)

		_, err := clientpkg.GetBudgets(context.Background(), http.DefaultClient, "tokengoeshere")
		Expect(err).To(HaveOccurred())
	})
})
