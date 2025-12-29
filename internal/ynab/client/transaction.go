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
	Payee       string
	Amount      int64
	Date        time.Time
	Description string
	Cleared     bool
}

// GetFormattedAmount returns the transaction amount formatted as a string in dollars and cents.
func (t *Transaction) GetFormattedAmount() string {
	if t.Amount == 0 {
		return "$0.00"
	}

	toFormat := t.Amount
	isNegative := toFormat < 0
	if isNegative {
		toFormat = -toFormat
	}

	amountCents := toFormat % 1000             //nolint:mnd
	dollars := (toFormat - amountCents) / 1000 //nolint:mnd
	cents := amountCents / 10                  //nolint:mnd

	signPrefix := ""
	if isNegative {
		signPrefix = "-"
	}

	return fmt.Sprintf("%s$%d.%02d", signPrefix, dollars, cents)
}

// IsOutbound returns true if the transaction amount is negative (i.e., money leaving the account).
func (t *Transaction) IsOutbound() bool {
	return t.Amount < 0
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
				ID        string `json:"id"`
				PayeeName string `json:"payee_name"`
				Amount    int64  `json:"amount"`
				Date      string `json:"date"`
				Memo      string `json:"memo"`
				Cleared   string `json:"cleared"`
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
			var err error
			dt, err = time.Parse("2006-01-02", t.Date)
			if err != nil {
				return nil, fmt.Errorf("failed to parse transaction date '%s': %w", t.Date, err)
			}
		}

		txns = append(txns, &Transaction{
			ID:          t.ID,
			Payee:       t.PayeeName,
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
		return "Transaction hash: " + txHash
	}

	if strings.Contains(existing, txHash) {
		return existing
	}

	return existing + "; transaction hash: " + txHash
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

// CreateTransactionRequest represents the data needed to create a new transaction.
type CreateTransactionRequest struct {
	AccountID  string
	Date       time.Time
	Amount     int64
	PayeeID    *string
	PayeeName  *string
	CategoryID *string
	Memo       *string
	Cleared    *string
	Approved   *bool
	FlagColor  *string
}

// CreateTransaction creates a new transaction in YNAB.
// Required fields: AccountID, Date, Amount.
// Returns the created transaction details.
func CreateTransaction(
	ctx context.Context,
	client ctshttp.Doer,
	accessToken string,
	budgetID string,
	req CreateTransactionRequest,
) (*Transaction, error) {
	requestPath, err := url.JoinPath(apiURL, "budgets", budgetID, "transactions")
	if err != nil {
		return nil, fmt.Errorf("failed to build request path for creating transaction: %w", err)
	}

	// Build the request payload
	payload := struct {
		Transaction struct {
			AccountID  string  `json:"account_id"`
			Date       string  `json:"date"`
			Amount     int64   `json:"amount"`
			PayeeID    *string `json:"payee_id,omitempty"`
			PayeeName  *string `json:"payee_name,omitempty"`
			CategoryID *string `json:"category_id,omitempty"`
			Memo       *string `json:"memo,omitempty"`
			Cleared    *string `json:"cleared,omitempty"`
			Approved   *bool   `json:"approved,omitempty"`
			FlagColor  *string `json:"flag_color,omitempty"`
		} `json:"transaction"`
	}{}

	payload.Transaction.AccountID = req.AccountID
	payload.Transaction.Date = req.Date.Format("2006-01-02")
	payload.Transaction.Amount = req.Amount
	payload.Transaction.PayeeID = req.PayeeID
	payload.Transaction.PayeeName = req.PayeeName
	payload.Transaction.CategoryID = req.CategoryID
	payload.Transaction.Memo = req.Memo
	payload.Transaction.Cleared = req.Cleared
	payload.Transaction.Approved = req.Approved
	payload.Transaction.FlagColor = req.FlagColor

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction create request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		requestPath,
		strings.NewReader(string(bodyBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for creating transaction: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for creating transaction: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("ynab API returned status %d on create", resp.StatusCode)
	}

	// Parse the response
	var envelope struct {
		Data struct {
			Transaction struct {
				ID         string  `json:"id"`
				AccountID  string  `json:"account_id"`
				PayeeName  string  `json:"payee_name"`
				PayeeID    *string `json:"payee_id"`
				Amount     int64   `json:"amount"`
				Date       string  `json:"date"`
				Memo       string  `json:"memo"`
				Cleared    string  `json:"cleared"`
				Approved   bool    `json:"approved"`
				CategoryID *string `json:"category_id"`
				FlagColor  *string `json:"flag_color"`
			} `json:"transaction"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode create transaction response: %w", err)
	}

	t := envelope.Data.Transaction
	var dt time.Time
	if t.Date != "" {
		var err error
		dt, err = time.Parse("2006-01-02", t.Date)
		if err != nil {
			return nil, fmt.Errorf("failed to parse transaction date '%s': %w", t.Date, err)
		}
	}

	return &Transaction{
		ID:          t.ID,
		Payee:       t.PayeeName,
		Amount:      t.Amount,
		Date:        dt,
		Description: t.Memo,
		Cleared:     !strings.EqualFold(t.Cleared, transactionClearedStatusUncleared),
	}, nil
}
