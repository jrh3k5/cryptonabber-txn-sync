package main

// TODO: add all processed hashes to the ignorelist file
// with a reason that it was processed for transaction ID XXXX on MM/DD/YYYY
// Also allow additions to the ignore list when prompting for import

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	ctsslog "github.com/jrh3k5/cryptonabber-txn-sync/internal/logging/slog"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/transfer"
	"github.com/manifoldco/promptui"

	ctsio "github.com/jrh3k5/cryptonabber-txn-sync/internal/io"
)

const (
	rpcNodeURLBase  = "https://mainnet.base.org"
	usdcAddressBase = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"

	// YNAB mock values
	acountName = "Base USDC Hot Storage"

	ignoreListFilename = "transaction_hash.ignorelist"
)

func main() {
	ctx := context.Background()

	debugMode := isDebug()
	if debugMode {
		debugTextHandler := ctsslog.NewHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		slog.SetDefault(slog.New(debugTextHandler))

		slog.DebugContext(ctx, "Running in debug mode; more detailed logging will be provided")
	}

	dryRun := isDryRun()
	if dryRun {
		slog.InfoContext(ctx, "Running in dry-run mode; no changes will be made to YNAB")
	}

	walletAddress, tokenAddress, httpClient, tokenDetails, transfers, err := initRun(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Initialization failed", "error", err)

		return
	}

	slog.InfoContext(ctx, fmt.Sprintf("Parsed %d transfers", len(transfers)))

	slog.InfoContext(
		ctx,
		fmt.Sprintf(
			"Synchronizing transactions for contract '%s' for wallet '%s'",
			tokenAddress,
			walletAddress,
		),
	)

	ignoreFileExists, err := ctsio.FileExists(ignoreListFilename)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to check for ignore list file", "error", err)

		return
	}

	var ignoreList *transaction.IgnoreList
	if ignoreFileExists {
		readHandle, err := os.Open(ignoreListFilename)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to open ignore list file", "error", err)

			return
		}
		defer func() { _ = readHandle.Close() }()

		ignoreList, err = transaction.FromYAML(readHandle)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to parse ignore list file", "error", err)

			return
		}

		slog.InfoContext(ctx, fmt.Sprintf("Loaded %d entries from ignore list", ignoreList.GetHashCount()))
	} else {
		ignoreList = transaction.NewIgnoreList()
	}

	// Schedule the writing of all ignored entries
	var writeHandle *os.File
	if !ignoreFileExists {
		writeHandle, err = os.Create(ignoreListFilename)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to create ignore list file", "err", err)

			return
		}
	} else {
		writeHandle, err = os.Open(ignoreListFilename)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to open file for writing ignore list to file", "err", err)

			return
		}
	}
	defer func() {
		_ = writeHandle.Close()
	}()

	defer func(writer io.Writer) {
		err := transaction.ToYAML(ignoreList, writer)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to write ignore list to YAML", "err", err)
		}
	}(writeHandle)

	ynabAccessToken, err := getAccessToken()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get YNAB access token", "error", err)

		return
	}

	if err := runSync(ctx, httpClient, tokenDetails, ynabAccessToken, walletAddress, transfers, dryRun, ignoreList); err != nil {
		slog.ErrorContext(ctx, "Synchronization failed", "error", err)

		return
	}
}

func initRun(
	ctx context.Context,
) (
	string,
	string,
	*http.Client,
	*token.Details,
	[]*transaction.Transfer,
	error,
) {
	walletAddress, err := getAddress()
	if err != nil {
		return "", "", nil, nil, nil, fmt.Errorf("failed to get wallet address: %w", err)
	}

	tokenAddress := getTokenAddress()
	slog.InfoContext(ctx, "Using token contract address: "+tokenAddress)

	rpcURL := getRPCURL()

	httpClient := http.DefaultClient

	slog.InfoContext(ctx, fmt.Sprintf("Retrieving token details for contract '%s'", tokenAddress))

	tokenDetailsService := token.NewRPCDetailsService(httpClient, rpcURL)

	tokenDetails, err := tokenDetailsService.GetTokenDetails(ctx, tokenAddress)
	if err != nil {
		return "", "", nil, nil, nil, fmt.Errorf("failed to retrieve token details: %w", err)
	}

	transfers, err := getTransfers(ctx, tokenDetails)
	if err != nil {
		return "", "", nil, nil, nil, fmt.Errorf("failed to get transfers: %w", err)
	}

	return walletAddress, tokenAddress, httpClient, tokenDetails, transfers, nil
}

func runSync(
	ctx context.Context,
	httpClient *http.Client,
	tokenDetails *token.Details,
	ynabAccessToken string,
	walletAddress string,
	transfers []*transaction.Transfer,
	dryRun bool,
	ignoreList *transaction.IgnoreList,
) error {
	budget, chosenAccountID, err := selectAccount(ctx, httpClient, ynabAccessToken)
	if err != nil {
		return fmt.Errorf("failed to select an account: %w", err)
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
		return fmt.Errorf("failed to retrieve uncleared transactions: %w", err)
	}

	slog.DebugContext(
		ctx,
		fmt.Sprintf("Retrieved %d uncleared transactions", len(unclearedTransactions)),
	)

	for _, unclearedTransaction := range unclearedTransactions {
		slog.DebugContext(
			ctx,
			fmt.Sprintf(
				"  - %s %s %s with description '%s'",
				unclearedTransaction.GetFormattedAmount(),
				transaction.ResolveDirection(unclearedTransaction.IsOutbound()),
				unclearedTransaction.Payee,
				unclearedTransaction.Description,
			),
		)
	}

	remainingTransfers, err := processUnclearedTransactions(
		ctx,
		httpClient,
		ynabAccessToken,
		budget.ID,
		walletAddress,
		tokenDetails,
		transfers,
		unclearedTransactions,
		dryRun,
		ignoreList,
	)
	if err != nil {
		return fmt.Errorf("failed to process uncleared transactions: %w", err)
	}

	return transaction.ImportRemainingTransfers(
		ctx,
		httpClient,
		ynabAccessToken,
		budget.ID,
		chosenAccountID,
		remainingTransfers,
		tokenDetails,
		walletAddress,
	)
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
		// If multiple budgets are available, prompt the user to select one.
		var items []string
		for _, b := range budgets {
			items = append(items, fmt.Sprintf("%s (%s)", b.Name, b.ID))
		}

		prompt := promptui.Select{
			Label: "Select a YNAB budget",
			Items: items,
		}

		i, _, err := prompt.Run()
		if err != nil {
			// If the user canceled the prompt (Ctrl-C/Ctrl-D), exit with an error so the program stops.
			if errors.Is(err, promptui.ErrInterrupt) || errors.Is(err, promptui.ErrEOF) {
				return nil, errors.New("budget selection canceled")
			}

			// Otherwise, if the prompt fails for a non-interactive reason, log a warning and fall back to the first budget.
			slog.WarnContext(
				ctx,
				"Budget selection prompt failed; defaulting to first budget",
				"error",
				err,
			)

			return budgets[0], nil
		}

		selected := budgets[i]
		slog.InfoContext(
			ctx,
			"Selected budget",
			"budgetName",
			selected.Name,
			"budgetID",
			selected.ID,
		)

		return selected, nil
	}
}

func chooseTransfer(
	ctx context.Context,
	tokenDetails *token.Details,
	transfers []*transaction.Transfer,
	walletAddress string,
	promptText string,
) (*transaction.Transfer, error) {
	sortedTransfers := make([]*transaction.Transfer, len(transfers))
	copy(sortedTransfers, transfers)
	// Sort transfers by amount and then execution time for easier selection.
	// Earlier transfers will appear first in the list.
	sort.Slice(sortedTransfers, func(i, j int) bool {
		byAmount := sortedTransfers[i].Amount.Cmp(sortedTransfers[j].Amount)
		if byAmount != 0 {
			return byAmount < 0
		}

		return sortedTransfers[i].ExecutionTime.Before(sortedTransfers[j].ExecutionTime)
	})

	// If multiple budgets are available, prompt the user to select one.
	items := make([]string, 0, len(sortedTransfers)+1)
	items = append(items, "Skip match")

	for _, xfr := range sortedTransfers {
		amountSign := ""
		if strings.EqualFold(xfr.FromAddress, walletAddress) {
			amountSign = "-"
		}

		items = append(
			items,
			fmt.Sprintf(
				"%s%s %s on %s (%s)",
				amountSign,
				xfr.FormatAmount(tokenDetails.Decimals),
				tokenDetails.Name,
				xfr.ExecutionTime.Format(time.RFC3339),
				xfr.TransactionHash,
			),
		)
	}

	prompt := promptui.Select{
		Label: promptText,
		Items: items,
	}

	i, _, err := prompt.Run()
	if err != nil {
		// If the user canceled the prompt (Ctrl-C/Ctrl-D), exit with an error so the program stops.
		if errors.Is(err, promptui.ErrInterrupt) || errors.Is(err, promptui.ErrEOF) {
			return nil, errors.New("transfer selection canceled")
		}

		return nil, fmt.Errorf("transfer selection prompt failed: %w", err)
	}

	if i == 0 {
		slog.DebugContext(ctx, "User opted to skip matching")

		return nil, nil
	}

	return sortedTransfers[i-1], nil
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

func getRPCURL() string {
	var rpcURL string
	for _, arg := range os.Args[1:] {
		parsedURL, hasPrefix := strings.CutPrefix(arg, "--rpc-url=")
		if hasPrefix {
			rpcURL = parsedURL

			break
		}
	}

	if rpcURL == "" {
		return rpcNodeURLBase
	}

	return rpcURL
}

func getTokenAddress() string {
	var tokenAddress string
	for _, arg := range os.Args[1:] {
		parsedAddress, hasPrefix := strings.CutPrefix(arg, "--token-address=")
		if hasPrefix {
			tokenAddress = parsedAddress

			break
		}
	}

	if tokenAddress == "" {
		return usdcAddressBase
	}

	return tokenAddress
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

func isDebug() bool {
	return slices.Contains(os.Args[1:], "--debug")
}

func isDryRun() bool {
	return slices.Contains(os.Args[1:], "--dry-run")
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

// processUnclearedTransactions attempts to match each uncleared transaction with a transfer.
// It returns any remaining unconsumed transfers after processing.
func processUnclearedTransactions(
	ctx context.Context,
	httpClient *http.Client,
	accessToken string,
	budgetID string,
	walletAddress string,
	tokenDetails *token.Details,
	transfers []*transaction.Transfer,
	unclearedTransactions []*client.Transaction,
	dryRun bool,
	ignoreList *transaction.IgnoreList,
) ([]*transaction.Transfer, error) {
	matchedCount := 0
	unmatchedCount := 0

	remainingTransfers := make([]*transaction.Transfer, len(transfers))
	copy(remainingTransfers, transfers)

	for _, unclearedTransaction := range unclearedTransactions {
		matchingTransfer, err := resolveMatchingTransfer(
			ctx,
			unclearedTransaction,
			walletAddress,
			tokenDetails,
			remainingTransfers,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve matching transfer: %w", err)
		}

		if matchingTransfer == nil {
			unmatchedCount++

			continue
		}

		matchedCount++

		slog.DebugContext(
			ctx,
			fmt.Sprintf(
				"Matched transfer of %s %s %s to transaction hash %s",
				unclearedTransaction.GetFormattedAmount(),
				transaction.ResolveDirection(unclearedTransaction.IsOutbound()),
				unclearedTransaction.Payee,
				matchingTransfer.TransactionHash,
			),
		)

		if !dryRun {
			if err := handleMatchedTransaction(ctx, httpClient, accessToken, budgetID, unclearedTransaction.ID, matchingTransfer.TransactionHash); err != nil {
				slog.ErrorContext(
					ctx,
					fmt.Sprintf(
						"Failed to mark transaction ID %s as cleared",
						unclearedTransaction.ID,
					),
					"error",
					err,
				)
			}

			ignoreList.AddProcessedHash(matchingTransfer.TransactionHash, unclearedTransaction.ID)
		}

		// Remove the matched transfer from remainingTransfers to prevent duplicate matches.
		for i := len(remainingTransfers) - 1; i >= 0; i-- {
			if remainingTransfers[i] == matchingTransfer {
				remainingTransfers = append(remainingTransfers[:i], remainingTransfers[i+1:]...)
			}
		}
	}

	slog.InfoContext(
		ctx,
		fmt.Sprintf("Matched %d transactions", matchedCount),
	)

	if unmatchedCount > 0 {
		slog.InfoContext(
			ctx,
			fmt.Sprintf(
				"Unable to match %d transactions; these may need to be manually matched or your CSV import may be out-of-date",
				unmatchedCount,
			),
		)
	}

	return remainingTransfers, nil
}

// resolveMatchingTransfer finds a matching transfer for the given uncleared transaction.
// If multiple matching transfers are found, it prompts the user to select one.
// If no matching transfers are found, it logs the absence and returns nil.
func resolveMatchingTransfer(
	ctx context.Context,
	unclearedTransaction *client.Transaction,
	walletAddress string,
	tokenDetails *token.Details,
	transfers []*transaction.Transfer,
) (*transaction.Transfer, error) {
	matchingTransfers := transfer.MatchTransfers(
		unclearedTransaction,
		walletAddress,
		tokenDetails,
		transfers,
	)

	if len(matchingTransfers) == 0 {
		slog.InfoContext(
			ctx,
			fmt.Sprintf(
				"No matching transfer of %s %s %s found",
				unclearedTransaction.GetFormattedAmount(),
				transaction.ResolveDirection(unclearedTransaction.IsOutbound()),
				unclearedTransaction.Payee,
			),
		)

		return nil, nil
	}

	if len(matchingTransfers) == 1 {
		return matchingTransfers[0], nil
	}

	var matchingTransfer *transaction.Transfer
	if len(matchingTransfers) > 1 {
		promptText := fmt.Sprintf(
			"Multiple transfers matched the transfer of %s %s %s with memo '%s' on %s; please select the correct one",
			unclearedTransaction.GetFormattedAmount(),
			transaction.ResolveDirection(unclearedTransaction.IsOutbound()),
			unclearedTransaction.Payee,
			unclearedTransaction.Description,
			unclearedTransaction.Date.Format(time.DateOnly),
		)

		var err error
		matchingTransfer, err = chooseTransfer(
			ctx,
			tokenDetails,
			matchingTransfers,
			walletAddress,
			promptText,
		)
		if err != nil {
			return nil, fmt.Errorf("transfer selection failed: %w", err)
		}

		return matchingTransfer, nil
	}

	promptText := fmt.Sprintf(
		"No transfers matched the transfer of %s %s %s with memo '%s' on %s; please select one from the list of imported transfers",
		unclearedTransaction.GetFormattedAmount(),
		transaction.ResolveDirection(unclearedTransaction.IsOutbound()),
		unclearedTransaction.Payee,
		unclearedTransaction.Description,
		unclearedTransaction.Date.Format(time.DateOnly),
	)

	var err error
	matchingTransfer, err = chooseTransfer(
		ctx,
		tokenDetails,
		transfers,
		walletAddress,
		promptText,
	)
	if err != nil {
		return nil, fmt.Errorf("transfer selection failed: %w", err)
	}

	return matchingTransfer, nil
}

func handleMatchedTransaction(
	ctx context.Context,
	httpClient *http.Client,
	accessToken, budgetID, transactionID, txHash string,
) error {
	if err := client.MarkTransactionClearedAndAppendMemo(ctx, httpClient, accessToken, budgetID, transactionID, txHash); err != nil {
		return fmt.Errorf("failed to update transaction %s: %w", transactionID, err)
	}

	return nil
}
