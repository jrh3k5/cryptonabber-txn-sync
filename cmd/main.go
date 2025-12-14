package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/davidsteinsland/ynab-go/ynab"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
	ctsynab "github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab"
)

const (
	rpcNodeURLBase  = "https://mainnet.base.org"
	usdcAddressBase = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	walletAddress   = "0x9134fc7112b478e97eE6F0E6A7bf81EcAfef19ED"

	ynabURL = "https://api.ynab.com/v1/"

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

	parsedURL, err := url.Parse(ynabURL)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to parse YNAB URL", "error", err)

		return
	}

	ynabClient := ynab.NewClient(parsedURL, httpClient, ynabAccessToken)

	allBudgets, err := ynabClient.BudgetService.List()
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

	unclearedTxns, err := ctsynab.GetUnclearedTransactions(
		ynabClient,
		budget.Id,
		acountName,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to retrieve uncleared transactions", "error", err)

		return
	}

	if len(unclearedTxns) == 0 {
		slog.InfoContext(ctx, "No uncleared transactions found; nothing to synchronize")

		return
	}

	slog.InfoContext(
		ctx,
		fmt.Sprintf("Retrieved %d uncleared transactions", len(unclearedTxns)),
	)

	for _, unclearedTransaction := range unclearedTxns {
		slog.InfoContext(
			ctx,
			fmt.Sprintf("Uncleared transaction: ID=%s, Date=%s, Amount=%d, Memo=%q",
				unclearedTransaction.Id,
				unclearedTransaction.Date,
				unclearedTransaction.Amount,
				unclearedTransaction.Memo,
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
