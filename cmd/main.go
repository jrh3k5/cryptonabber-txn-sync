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
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/transfer"
)

const (
	rpcNodeURLBase  = "https://mainnet.base.org"
	usdcAddressBase = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"

	// YNAB mock values
	acountName = "Base USDC Hot Storage"
)

func main() {
	ctx := context.Background()

	walletAddress, err := getAddress()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get wallet address", "error", err)

		return
	}

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

	if err := runSync(ctx, httpClient, tokenDetails, ynabAccessToken, walletAddress, transfers); err != nil {
		slog.ErrorContext(ctx, "Synchronization failed", "error", err)

		return
	}
}

func runSync(
	ctx context.Context,
	httpClient *http.Client,
	tokenDetails *token.Details,
	ynabAccessToken string,
	walletAddress string,
	transfers []*transaction.Transfer,
) error {
	budget, chosenAccountID, err := selectAccount(ctx, httpClient, ynabAccessToken)
	if err != nil {
		return err
	}

	unclearedTransactions, err := retrieveUnclearedTransactions(
		ctx,
		httpClient,
		ynabAccessToken,
		budget.ID,
		chosenAccountID,
		time.Now().Add(-7*24*time.Hour),
	)
	if err != nil {
		return err
	}

	slog.InfoContext(
		ctx,
		fmt.Sprintf("Retrieved %d uncleared transactions", len(unclearedTransactions)),
	)

	processUnclearedTransactions(
		ctx,
		httpClient,
		ynabAccessToken,
		budget.ID,
		walletAddress,
		tokenDetails,
		transfers,
		unclearedTransactions,
	)

	return nil
}

func filterUncleared(transactions []*client.Transaction) []*client.Transaction {
	var out []*client.Transaction
	for _, txn := range transactions {
		if !txn.Cleared {
			out = append(out, txn)
		}
	}

	return out
}

func chooseBudget(ctx context.Context, budgets []*client.Budget) (*client.Budget, error) {
	switch len(budgets) {
	case 0:
		return nil, errors.New("no YNAB budgets found; at least one budget is required")
	case 1:
		return budgets[0], nil
	default:
		// prefer the first budget and log the selection
		slog.InfoContext(
			ctx,
			fmt.Sprintf(
				"%d budgets returned; using the first ('%s')",
				len(budgets),
				budgets[0].Name,
			),
		)

		return budgets[0], nil
	}
}

func findAccountID(accounts []*client.Account, name string) (string, error) {
	for _, acct := range accounts {
		if acct.Name == name {
			return acct.ID, nil
		}
	}

	return "", fmt.Errorf("account '%s' not found", name)
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

func getAddress() (string, error) {
	var address string
	for _, arg := range os.Args[1:] {
		parsedAddress, hasPrefix := strings.CutPrefix(arg, "--wallet-address=")
		if hasPrefix {
			address = parsedAddress

			break
		}
	}

	if address == "" {
		return "", errors.New("--wallet-address argument is required")
	}

	return address, nil
}

func getTransfers(
	ctx context.Context,
	tokenDetails *token.Details,
) ([]*transaction.Transfer, error) {
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

func selectAccount(
	ctx context.Context,
	httpClient *http.Client,
	ynabAccessToken string,
) (*client.Budget, string, error) {
	allBudgets, err := client.GetBudgets(ctx, httpClient, ynabAccessToken)
	if err != nil {
		return nil, "", fmt.Errorf("failed to retrieve YNAB budgets: %w", err)
	}

	budget, err := chooseBudget(ctx, allBudgets)
	if err != nil {
		return nil, "", err
	}

	accounts, err := client.GetAccounts(ctx, httpClient, ynabAccessToken, budget.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to retrieve YNAB accounts: %w", err)
	}

	chosenAccountID, err := findAccountID(accounts, acountName)
	if err != nil {
		return nil, "", fmt.Errorf("account '%s' not found in budget '%s'", acountName, budget.Name)
	}

	return budget, chosenAccountID, nil
}

func retrieveUnclearedTransactions(
	ctx context.Context,
	httpClient *http.Client,
	ynabAccessToken string,
	budgetID string,
	accountID string,
	since time.Time,
) ([]*client.Transaction, error) {
	transactions, err := client.GetTransactions(
		ctx,
		httpClient,
		ynabAccessToken,
		budgetID,
		accountID,
		since,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	return filterUncleared(transactions), nil
}

func processUnclearedTransactions(
	ctx context.Context,
	httpClient *http.Client,
	accessToken string,
	budgetID string,
	walletAddress string,
	tokenDetails *token.Details,
	transfers []*transaction.Transfer,
	unclearedTransactions []*client.Transaction,
) {
	for _, unclearedTransaction := range unclearedTransactions {
		matchingTransfer := transfer.MatchTransfer(
			unclearedTransaction,
			walletAddress,
			tokenDetails,
			transfers,
		)

		if matchingTransfer == nil {
			slog.InfoContext(
				ctx,
				fmt.Sprintf(
					"No matching transfer found for transaction ID '%s' on date %s with amount %d",
					unclearedTransaction.ID,
					unclearedTransaction.Date,
					unclearedTransaction.Amount,
				),
			)

			continue
		}

		slog.InfoContext(
			ctx,
			fmt.Sprintf(
				"Found matching transfer for transaction ID '%s' on date %s with amount %d: %s",
				unclearedTransaction.ID,
				unclearedTransaction.Date,
				unclearedTransaction.Amount,
				matchingTransfer.TransactionHash,
			),
		)

		if err := client.MarkTransactionClearedAndAppendMemo(
			ctx,
			httpClient,
			accessToken,
			budgetID,
			unclearedTransaction.ID,
			matchingTransfer.TransactionHash,
		); err != nil {
			slog.ErrorContext(
				ctx,
				fmt.Sprintf(
					"Failed to mark transaction ID '%s' as cleared",
					unclearedTransaction.ID,
				),
				"error",
				err,
			)
		}
	}
}
