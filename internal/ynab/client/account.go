package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	ctshttp "github.com/jrh3k5/cryptonabber-txn-sync/internal/http"
)

type Account struct {
	ID   string
	Name string
}

func GetAccounts(
	ctx context.Context,
	client ctshttp.Doer,
	accessToken string,
	budgetID string,
) ([]*Account, error) {
	requestPath, err := url.JoinPath(apiURL, "budgets", budgetID, "accounts")
	if err != nil {
		return nil, fmt.Errorf("failed to build request path for fetching accounts: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for fetching accounts: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for fetching accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ynab API returned status %d", resp.StatusCode)
	}

	var envelope struct {
		Data struct {
			Accounts []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"accounts"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode accounts response: %w", err)
	}

	var out []*Account
	for _, a := range envelope.Data.Accounts {
		out = append(out, &Account{ID: a.ID, Name: a.Name})
	}

	return out, nil
}
