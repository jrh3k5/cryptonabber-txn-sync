package transaction

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"strings"
	"time"

	ctsbig "github.com/jrh3k5/cryptonabber-txn-sync/internal/big"
	ctsio "github.com/jrh3k5/cryptonabber-txn-sync/internal/io"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
)

// TransfersFromEtherscanCSV parses the given Etherscan CSV data representing activity for the given token details and returns a slice of Transfers.
// It expects the given reader to contain a CSV with the following columns in no given order:
// - Transaction Hash, which is the hash of the transaction in hex
// - From, which is the address that sent the token in hex
// - To, which is the address that received the token in hex
// - Amount, which is the amount of tokens transferred in the token's base unit
// - DateTime (UTC), which is the time the transaction was executed in UTC
func TransfersFromEtherscanCSV(
	ctx context.Context,
	tokenDetails *token.Details,
	csvReader io.Reader,
) ([]Transfer, error) {
	// wrap the reader to strip a leading UTF-8 BOM (U+FEFF) if present
	r := csv.NewReader(ctsio.StripUTF8BOM(csvReader))
	r.TrimLeadingSpace = true

	// read header
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read the first line of the CSV: %w", err)
	}

	txIdx, fromIdx, toIdx, amountIdx, timeIdx, err := parseHeader(header)
	if err != nil {
		return nil, err
	}

	var transfers []Transfer

	for {
		record, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("read CSV record: %w", err)
		}

		// skip empty records
		if len(record) == 0 {
			slog.DebugContext(ctx, "Row has no values in it; skipping")

			continue
		}

		t, err := parseRecord(record, txIdx, fromIdx, toIdx, amountIdx, timeIdx, tokenDetails)
		if err != nil {
			return nil, err
		}

		transfers = append(transfers, t)
	}

	return transfers, nil
}

func requiredColumn(hdrIdx map[string]int, header []string, name string) (int, error) {
	idx, ok := hdrIdx[name]
	if !ok {
		return 0, fmt.Errorf(
			"CSV is missing required column: %s from available columns: [%s]",
			name,
			strings.Join(header, ", "),
		)
	}

	return idx, nil
}

func parseHeader(header []string) (int, int, int, int, int, error) {
	hdrIdx := make(map[string]int)
	for i, h := range header {
		key := strings.TrimSpace(strings.ToLower(h))
		hdrIdx[key] = i
	}

	txIdx, err := requiredColumn(hdrIdx, header, "transaction hash")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	fromIdx, err := requiredColumn(hdrIdx, header, "from")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	toIdx, err := requiredColumn(hdrIdx, header, "to")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	amountIdx, err := requiredColumn(hdrIdx, header, "amount")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	timeIdx, err := requiredColumn(hdrIdx, header, "datetime (utc)")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	return txIdx, fromIdx, toIdx, amountIdx, timeIdx, nil
}

func parseAmount(amountStr string, decimals int, txHash string) (*big.Int, error) {
	if amountStr == "" {
		return nil, fmt.Errorf("transaction hash %q has empty amount field", txHash)
	}

	totalAmount := new(big.Int)
	wholeTokens, fracTokens, fracTokensLength, err := splitAmountParts(amountStr, txHash)
	if err != nil {
		return nil, err
	}

	// Expand the whole tokens out to base units
	if wholeTokens.Cmp(big.NewInt(0)) == 1 {
		//nolint:mnd
		totalAmount = new(
			big.Int,
		).Mul(wholeTokens, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	}

	if fracTokens.Cmp(big.NewInt(0)) == 1 {
		exponent := decimals - fracTokensLength
		if exponent < 0 {
			return nil, fmt.Errorf(
				"fractional token amount %q for transaction hash %q has more decimal places than token supports",
				amountStr,
				txHash,
			)
		}

		//nolint:mnd
		fracBaseUnits := new(
			big.Int,
		).Mul(fracTokens, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil))
		totalAmount = totalAmount.Add(totalAmount, fracBaseUnits)
	}

	return totalAmount, nil
}

func splitAmountParts(amountStr, txHash string) (*big.Int, *big.Int, int, error) {
	var wholeTokens *big.Int
	fracTokens := new(big.Int)
	fracTokensLength := 0

	if !strings.Contains(amountStr, ".") {
		var err error
		wholeTokens, err = ctsbig.BigIntFromString(amountStr)
		if err != nil {
			return nil, nil, 0, fmt.Errorf(
				"parse whole token amount %q for transaction hash %q: %w",
				amountStr,
				txHash,
				err,
			)
		}

		return wholeTokens, fracTokens, fracTokensLength, nil
	}

	parts := strings.SplitN(amountStr, ".", 2) //nolint:mnd
	var err error
	wholeTokens, err = ctsbig.BigIntFromString(parts[0])
	if err != nil {
		return nil, nil, 0, fmt.Errorf(
			"parse whole token amount %q for transaction hash %q: %w",
			parts[0],
			txHash,
			err,
		)
	}

	fracTokensString := strings.TrimRight(parts[1], "0")
	fracTokensLength = len(fracTokensString)

	if fracTokensLength > 0 {
		var err error
		fracTokens, err = ctsbig.BigIntFromString(fracTokensString)
		if err != nil {
			return nil, nil, 0, fmt.Errorf(
				"parse fractional token amount %q for transaction hash %q: %w",
				parts[1],
				txHash,
				err,
			)
		}
	}

	return wholeTokens, fracTokens, fracTokensLength, nil
}

func parseExecutionTime(timeStr, txHash string) (time.Time, error) {
	executionTime, err := time.Parse("2006-01-02 15:04:05", timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"parse execution time %q for transaction hash %q: %w",
			timeStr,
			txHash,
			err,
		)
	}

	return executionTime, nil
}

func parseRecord(
	record []string,
	txIdx, fromIdx, toIdx, amountIdx, timeIdx int,
	tokenDetails *token.Details,
) (Transfer, error) {
	if txIdx >= len(record) || fromIdx >= len(record) || toIdx >= len(record) ||
		amountIdx >= len(record) ||
		timeIdx >= len(record) {
		return Transfer{}, fmt.Errorf("malformed csv record: %v", record)
	}

	txHash := strings.TrimSpace(record[txIdx])
	from := strings.TrimSpace(record[fromIdx])
	to := strings.TrimSpace(record[toIdx])
	amountStr := strings.TrimSpace(record[amountIdx])
	timeStr := strings.TrimSpace(record[timeIdx])

	totalAmount, err := parseAmount(amountStr, tokenDetails.Decimals, txHash)
	if err != nil {
		return Transfer{}, err
	}

	executionTime, err := parseExecutionTime(timeStr, txHash)
	if err != nil {
		return Transfer{}, err
	}

	return Transfer{
		FromAddress:     from,
		ToAddress:       to,
		Amount:          totalAmount,
		ExecutionTime:   executionTime,
		TransactionHash: txHash,
	}, nil
}
