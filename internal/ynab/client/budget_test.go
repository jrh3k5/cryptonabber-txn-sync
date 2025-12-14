package client_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	clientpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
	. "github.com/onsi/gomega"
)

func TestGetBudgets_Success(t *testing.T) {
	RegisterTestingT(t)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	respBody := `{"data":{"budgets":[{"id":"b1","name":"Main Budget"}]}}`

	httpmock.RegisterResponder(
		"GET",
		"https://api.ynab.com/v1/budgets",
		func(req *http.Request) (*http.Response, error) {
			Expect(req.Header.Get("Authorization")).To(Equal("Bearer tokengoeshere"))

			return httpmock.NewStringResponse(200, respBody), nil
		},
	)

	budgets, err := clientpkg.GetBudgets(context.Background(), http.DefaultClient, "tokengoeshere")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(budgets) != 1 {
		t.Fatalf("expected 1 budget, got %d", len(budgets))
	}
	if budgets[0].ID != "b1" || budgets[0].Name != "Main Budget" {
		t.Fatalf("unexpected budget data: %+v", budgets[0])
	}
}

func TestGetBudgets_Non200(t *testing.T) {
	RegisterTestingT(t)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(
		"GET",
		"https://api.ynab.com/v1/budgets",
		httpmock.NewStringResponder(http.StatusInternalServerError, ""),
	)

	_, err := clientpkg.GetBudgets(context.Background(), http.DefaultClient, "tokengoeshere")
	if err == nil {
		t.Fatalf("expected error for non-200 response")
	}
}
