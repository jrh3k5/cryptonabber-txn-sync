package client

import (
	"context"
	"fmt"
	"net/url"
	"time"

	ctshttp "github.com/jrh3k5/cryptonabber-txn-sync/internal/http"
)

type Transaction struct {
	ID          string
	Amount      int64
	Date        time.Time
	Description string
	Cleared     bool
}

func GetTransactions(ctx context.Context, client ctshttp.Doer, budgetID, accountID string, sinceDate time.Time) ([]*Transaction, error) {
	requestPath, err := url.JoinPath(apiURL, "budgets", budgetID, "accounts", accountID, "transactions")
	if err != nil {
		return nil, fmt.Errorf("failed to build request path for fetching transactions: %w", err)
	}
}
