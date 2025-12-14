package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
)

const (
	rpcNodeURLBase  = "https://mainnet.base.org"
	usdcAddressBase = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	walletAddress   = "0x9134fc7112b478e97eE6F0E6A7bf81EcAfef19ED"
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
