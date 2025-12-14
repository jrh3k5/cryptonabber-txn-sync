package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
)

const (
	rpcNodeURLBase  = "https://mainnet.base.org"
	usdcAddressBase = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	walletAddress   = "0x9134fc7112b478e97eE6F0E6A7bf81EcAfef19ED"

	// YNAB mock values
	acountName = "Base USDC Hot Storage"
)

func main() {
	ctx := context.Background()

	httpClient := http.DefaultClient

	slog.InfoContext(
		ctx,
		fmt.Sprintf("Retrieving token details for contract '%s'", usdcAddressBase),
	)

	tokenDetailsService := token.NewRPCDetailsService(httpClient, rpcNodeURLBase)

	tokenDetails, err := tokenDetailsService.GetTokenDetails(ctx, usdcAddressBase)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to retrieve token details", "error", err)

		return
	}

	transfers, err := getTransfers(ctx, tokenDetails)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to retrieve transfers", "error", err)

		return
	}

	slog.InfoContext(ctx, fmt.Sprintf("Parsed %d transfers", len(transfers)))

	slog.InfoContext(
		ctx,
		fmt.Sprintf(
			"Synchronizing transactions for contract '%s' for wallet '%s'",
			usdcAddressBase,
			walletAddress,
		),
	)

	ynabAccessToken, err := getAccessToken()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get YNAB access token", "error", err)

		return
	}

	allBudgets, err := client.GetBudgets(ctx, httpClient, ynabAccessToken)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to retrieve YNAB budgets", "error", err)

		return
	} else if len(allBudgets) == 0 {
		slog.ErrorContext(ctx, "No YNAB budgets found; at least one budget is required")

		return
	} else if len(allBudgets) > 1 {
		slog.InfoContext(ctx, fmt.Sprintf("%d budgets returned; using the first ('%s')", len(allBudgets), allBudgets[0].Name))
	}

	budget := allBudgets[0]

	accounts, err := client.GetAccounts(ctx, httpClient, ynabAccessToken, budget.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to retrieve YNAB accounts", "error", err)

		return
	}

	var accountFound bool
	for _, acct := range accounts {
		if acct.Name == acountName {
			accountFound = true

			break
		}
	}
	if !accountFound {
		slog.ErrorContext(
			ctx,
			fmt.Sprintf("Account '%s' not found in budget '%s'", acountName, budget.Name),
		)

		return
	}

	transactions, err := client.GetTransactions(
		ctx,
		httpClient,
		ynabAccessToken,
		budget.ID,
		accounts[0].ID,
		time.Now().Add(-7*24*time.Hour),
	)

	var unclearedTransactions []*client.Transaction
	for _, txn := range transactions {
		if !txn.Cleared {
			unclearedTransactions = append(unclearedTransactions, txn)
		}
	}

	slog.InfoContext(
		ctx,
		fmt.Sprintf("Retrieved %d uncleared transactions", len(unclearedTransactions)),
	)

	for _, unclearedTransaction := range unclearedTransactions {
		slog.InfoContext(
			ctx,
			fmt.Sprintf("Uncleared transaction: ID=%s, Date=%s, Amount=%d",
				unclearedTransaction.ID,
				unclearedTransaction.Date,
				unclearedTransaction.Amount,
			),
		)
	}
}

func getAccessToken() (string, error) {
	var accessToken string
	for _, arg := range os.Args[1:] {
		parsedToken, hasPrefix := strings.CutPrefix(arg, "--ynab-access-token=")
		if hasPrefix {
			accessToken = parsedToken

			break
		}
	}

	if accessToken == "" {
		return "", errors.New("--ynab-access-token argument is required")
	}

	return accessToken, nil
}

func getTransfers(
	ctx context.Context,
	tokenDetails *token.Details,
) ([]transaction.Transfer, error) {
	var csvFile string
	for _, arg := range os.Args[1:] {
		parsedFile, hasPrefix := strings.CutPrefix(arg, "--csv-file=")
		if hasPrefix {
			csvFile = parsedFile

			break
		}
	}

	if csvFile == "" {
		return nil, errors.New("--csv-file argument is required")
	}

	file, err := os.Open(csvFile) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer func() { _ = file.Close() }()

	transfers, err := transaction.TransfersFromEtherscanCSV(ctx, tokenDetails, file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transfers from CSV: %w", err)
	}

	return transfers, nil
}
