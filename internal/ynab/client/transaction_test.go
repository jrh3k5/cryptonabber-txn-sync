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

		respBody := `{"data":{"transactions":[{"id":"tx1","payee_name":"John Doe","amount":1000,"date":"2025-12-01","memo":"test memo","cleared":"uncleared"}]}}`

		httpmock.RegisterResponder(
			"GET",
			"https://api.ynab.com/v1/budgets/budget1/accounts/acct1/transactions",
			func(req *http.Request) (*http.Response, error) {
				Expect(req.URL.Query().Get("since_date")).To(Equal("2025-12-01"))
				Expect(req.Header.Get("Authorization")).To(Equal("Bearer tokengoeshere"))

				return httpmock.NewStringResponse(200, respBody), nil
			},
		)

		txns, err := clientpkg.GetTransactions(
			ctx,
			http.DefaultClient,
			"tokengoeshere",
			"budget1",
			"acct1",
			since,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(txns).To(HaveLen(1))

		txn := txns[0]
		Expect(txn.ID).To(Equal("tx1"))
		Expect(txn.Payee).To(Equal("John Doe"))
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

var _ = Describe("CreateTransaction", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("creates a transaction with minimal required fields", func() {
		respBody := `{"data":{"transaction":{"id":"tx-created","account_id":"acct1","date":"2025-12-24","amount":5000,"payee_name":"","memo":"","cleared":"uncleared","approved":false}}}`

		httpmock.RegisterResponder(
			"POST",
			"https://api.ynab.com/v1/budgets/budget1/transactions",
			func(req *http.Request) (*http.Response, error) {
				Expect(req.Header.Get("Authorization")).To(Equal("Bearer tokengoeshere"))
				Expect(req.Header.Get("Content-Type")).To(Equal("application/json"))

				return httpmock.NewStringResponse(http.StatusCreated, respBody), nil
			},
		)

		txn, err := clientpkg.CreateTransaction(
			ctx,
			http.DefaultClient,
			"tokengoeshere",
			"budget1",
			clientpkg.CreateTransactionRequest{
				AccountID: "acct1",
				Date:      time.Date(2025, 12, 24, 0, 0, 0, 0, time.UTC),
				Amount:    5000,
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(txn).ToNot(BeNil())
		Expect(txn.ID).To(Equal("tx-created"))
		Expect(txn.Amount).To(Equal(int64(5000)))
		Expect(txn.Date.Year()).To(Equal(2025))
		Expect(txn.Date.Month()).To(Equal(time.December))
		Expect(txn.Date.Day()).To(Equal(24))
		Expect(txn.Cleared).To(BeFalse())
	})

	It("creates a transaction with all optional fields", func() {
		payeeID := "payee-123"
		payeeName := "Test Payee"
		categoryID := "cat-456"
		memo := "Test transaction memo"
		cleared := "cleared"
		approved := true
		flagColor := "red"

		respBody := `{"data":{"transaction":{"id":"tx-full","account_id":"acct1","date":"2025-12-24","amount":-10000,"payee_id":"payee-123","payee_name":"Test Payee","category_id":"cat-456","memo":"Test transaction memo","cleared":"cleared","approved":true,"flag_color":"red"}}}`

		httpmock.RegisterResponder(
			"POST",
			"https://api.ynab.com/v1/budgets/budget1/transactions",
			func(req *http.Request) (*http.Response, error) {
				Expect(req.Header.Get("Authorization")).To(Equal("Bearer tokengoeshere"))
				Expect(req.Header.Get("Content-Type")).To(Equal("application/json"))

				return httpmock.NewStringResponse(http.StatusCreated, respBody), nil
			},
		)

		txn, err := clientpkg.CreateTransaction(
			ctx,
			http.DefaultClient,
			"tokengoeshere",
			"budget1",
			clientpkg.CreateTransactionRequest{
				AccountID:  "acct1",
				Date:       time.Date(2025, 12, 24, 0, 0, 0, 0, time.UTC),
				Amount:     -10000,
				PayeeID:    &payeeID,
				PayeeName:  &payeeName,
				CategoryID: &categoryID,
				Memo:       &memo,
				Cleared:    &cleared,
				Approved:   &approved,
				FlagColor:  &flagColor,
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(txn).ToNot(BeNil())
		Expect(txn.ID).To(Equal("tx-full"))
		Expect(txn.Amount).To(Equal(int64(-10000)))
		Expect(txn.Payee).To(Equal("Test Payee"))
		Expect(txn.Description).To(Equal("Test transaction memo"))
		Expect(txn.Cleared).To(BeTrue())
	})

	It("returns an error on non-201 response", func() {
		httpmock.RegisterResponder(
			"POST",
			"https://api.ynab.com/v1/budgets/budget1/transactions",
			func(req *http.Request) (*http.Response, error) {
				return httpmock.NewStringResponse(
					http.StatusBadRequest,
					`{"error":{"id":"400","name":"bad_request","detail":"Invalid request"}}`,
				), nil
			},
		)

		_, err := clientpkg.CreateTransaction(
			ctx,
			http.DefaultClient,
			"tokengoeshere",
			"budget1",
			clientpkg.CreateTransactionRequest{
				AccountID: "acct1",
				Date:      time.Date(2025, 12, 24, 0, 0, 0, 0, time.UTC),
				Amount:    5000,
			},
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("400"))
	})
})
