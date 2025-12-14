package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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

// GetTransactions fetches transactions from the YNAB API for a given budget and account.
// If sinceDate is non-zero, the `since_date` query parameter will be set.
func GetTransactions(
	ctx context.Context,
	client ctshttp.Doer,
	accessToken string,
	budgetID string,
	accountID string,
	sinceDate time.Time,
) ([]*Transaction, error) {
	requestPath, err := url.JoinPath(
		apiURL,
		"budgets",
		budgetID,
		"accounts",
		accountID,
		"transactions",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build request path for fetching transactions: %w", err)
	}

	// Build request URL and add since_date when provided
	reqURL, err := url.Parse(requestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request URL '%s': %w", requestPath, err)
	}

	q := reqURL.Query()
	if !sinceDate.IsZero() {
		q.Set("since_date", sinceDate.Format("2006-01-02"))
		reqURL.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for fetching transactions: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for fetching transactions: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ynab API returned status %d", resp.StatusCode)
	}

	// Expected response: { "data": { "transactions": [ ... ] } }
	return parseTransactionsFromBody(resp.Body)
}

func parseTransactionsFromBody(body io.Reader) ([]*Transaction, error) {
	var envelope struct {
		Data struct {
			Transactions []struct {
				ID      string `json:"id"`
				Amount  int64  `json:"amount"`
				Date    string `json:"date"`
				Memo    string `json:"memo"`
				Cleared string `json:"cleared"`
			} `json:"transactions"`
		} `json:"data"`
	}

	if err := json.NewDecoder(body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode transactions response: %w", err)
	}

	txns := make([]*Transaction, 0, len(envelope.Data.Transactions))
	for _, t := range envelope.Data.Transactions {
		var dt time.Time
		if t.Date != "" {
			dt, _ = time.Parse("2006-01-02", t.Date)
		}

		txns = append(txns, &Transaction{
			ID:          t.ID,
			Amount:      t.Amount,
			Date:        dt,
			Description: t.Memo,
			Cleared:     strings.EqualFold(t.Cleared, "cleared"),
		})
	}

	return txns, nil
}
