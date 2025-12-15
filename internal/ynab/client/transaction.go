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
			Cleared:     !strings.EqualFold(t.Cleared, transactionClearedStatusUncleared),
		})
	}

	return txns, nil
}

// MarkTransactionClearedAndAppendMemo fetches the transaction, marks it as cleared,
// and appends the given transaction hash to the memo if not already present.
func MarkTransactionClearedAndAppendMemo(
	ctx context.Context,
	client ctshttp.Doer,
	accessToken string,
	budgetID string,
	transactionID string,
	txHash string,
) error {
	reqPath, err := url.JoinPath(apiURL, "budgets", budgetID, "transactions", transactionID)
	if err != nil {
		return fmt.Errorf("failed to build request path for transaction: %w", err)
	}

	txn, err := fetchTransaction(ctx, client, accessToken, reqPath)
	if err != nil {
		return err
	}

	memo := computeMemo(strings.TrimSpace(txn.Memo), txHash)

	payload := struct {
		Transaction struct {
			Memo    string `json:"memo"`
			Cleared string `json:"cleared"`
		} `json:"transaction"`
	}{}

	payload.Transaction.Memo = memo
	payload.Transaction.Cleared = "cleared"

	if err := updateTransaction(ctx, client, accessToken, reqPath, payload); err != nil {
		return err
	}

	return nil
}

type fetchedTransaction struct {
	ID      string `json:"id"`
	Memo    string `json:"memo"`
	Cleared string `json:"cleared"`
}

func fetchTransaction(
	ctx context.Context,
	client ctshttp.Doer,
	accessToken, requestPath string,
) (*fetchedTransaction, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for fetching transaction: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for fetching transaction: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ynab API returned status %d", resp.StatusCode)
	}

	var envelope struct {
		Data struct {
			Transaction fetchedTransaction `json:"transaction"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode transaction response: %w", err)
	}

	return &envelope.Data.Transaction, nil
}

func computeMemo(existing, txHash string) string {
	if txHash == "" {
		return existing
	}

	if existing == "" {
		return txHash
	}

	if strings.Contains(existing, txHash) {
		return existing
	}

	return existing + " " + txHash
}

func updateTransaction(
	ctx context.Context,
	client ctshttp.Doer,
	accessToken, requestPath string,
	payload any,
) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction update: %w", err)
	}

	putReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		requestPath,
		strings.NewReader(string(bodyBytes)),
	)
	if err != nil {
		return fmt.Errorf("failed to create request for updating transaction: %w", err)
	}
	putReq.Header.Set("Authorization", "Bearer "+accessToken)
	putReq.Header.Set("Content-Type", "application/json")

	putResp, err := client.Do(putReq)
	if err != nil {
		return fmt.Errorf("failed to execute request for updating transaction: %w", err)
	}
	defer func() { _ = putResp.Body.Close() }()

	if putResp.StatusCode != http.StatusOK {
		return fmt.Errorf("ynab API returned status %d on update", putResp.StatusCode)
	}

	return nil
}
