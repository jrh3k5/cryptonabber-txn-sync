package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	ctshttp "github.com/jrh3k5/cryptonabber-txn-sync/internal/http"
)

type Budget struct {
	ID   string
	Name string
}

func GetBudgets(ctx context.Context, client ctshttp.Doer, accessToken string) ([]*Budget, error) {
	requestPath, err := url.JoinPath(apiURL, "budgets")
	if err != nil {
		return nil, fmt.Errorf("failed to build request path for fetching budgets: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for fetching budgets: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for fetching budgets: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ynab API returned status %d", resp.StatusCode)
	}

	var envelope struct {
		Data struct {
			Budgets []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"budgets"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode budgets response: %w", err)
	}

	out := make([]*Budget, 0, len(envelope.Data.Budgets))
	for _, b := range envelope.Data.Budgets {
		out = append(out, &Budget{ID: b.ID, Name: b.Name})
	}

	return out, nil
}
