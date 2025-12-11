package transaction

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"

	ctsio "github.com/jrh3k5/cryptonabber-txn-sync/internal/io"
)

// TransfersFromEtherscanCSV parses the given Etherscan CSV data representing activity for the given token details and returns a slice of Transfers.
// It expects the given reader to contain a CSV with the following columns in no given order:
// - Transaction Hash, which is the hash of the transaction in hex
// - From, which is the address that sent the token in hex
// - To, which is the address that received the token in hex
// - Amount, which is the amount of tokens transferred in the token's base unit
// - DateTime (UTC), which is the time the transaction was executed in UTC
func TransfersFromEtherscanCSV(ctx context.Context, tokenDetails *token.Details, csvReader io.Reader) ([]Transfer, error) {
	// wrap the reader to strip a leading UTF-8 BOM (U+FEFF) if present
	r := csv.NewReader(ctsio.StripUTF8BOM(csvReader))
	r.TrimLeadingSpace = true

	// read header
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read the first line of the CSV: %w", err)
	}

	// map header names (lowercased, trimmed) to indices
	hdrIdx := make(map[string]int)
	for i, h := range header {
		key := strings.TrimSpace(strings.ToLower(h))
		hdrIdx[key] = i
	}

	txIdx, txFound := hdrIdx["transaction hash"]
	if !txFound {
		return nil, fmt.Errorf("CSV is missing required column: Transaction Hash from available columns: [%s]", strings.Join(header, ", "))
	}

	fromIdx, fromFound := hdrIdx["from"]
	if !fromFound {
		return nil, fmt.Errorf("CSV is missing required column: From from available columns: [%s]", strings.Join(header, ", "))
	}

	toIdx, toFound := hdrIdx["to"]
	if !toFound {
		return nil, fmt.Errorf("CSV is missing required column: To from available columns: [%s]", strings.Join(header, ", "))
	}

	amountIdx, amountFound := hdrIdx["amount"]
	if !amountFound {
		return nil, fmt.Errorf("CSV is missing required column: Amount from available columns: [%s]", strings.Join(header, ", "))
	}

	timeIdx, timeFound := hdrIdx["datetime (utc)"]
	if !timeFound {
		return nil, fmt.Errorf("CSV is missing required column: DateTime (UTC) from available columns: [%s]", strings.Join(header, ", "))
	}

	var transfers []Transfer

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV record: %w", err)
		}

		// skip empty records
		if len(record) == 0 {
			continue
		}

		// protect against short records
		if txIdx >= len(record) || fromIdx >= len(record) || toIdx >= len(record) || amountIdx >= len(record) || timeIdx >= len(record) {
			return nil, fmt.Errorf("malformed csv record: %v", record)
		}

		txHash := strings.TrimSpace(record[txIdx])
		from := strings.TrimSpace(record[fromIdx])
		to := strings.TrimSpace(record[toIdx])
		amountStr := strings.TrimSpace(record[amountIdx])
		timeStr := strings.TrimSpace(record[timeIdx])

		// parse amount as integer in base units
		totalAmount := new(big.Int)
		if amountStr == "" {
			return nil, fmt.Errorf("transaction hash %q has empty amount field", txHash)
		} else {
			wholeTokens := new(big.Int)
			fracTokens := new(big.Int)
			fracTokensLength := 0
			if strings.Contains(amountStr, ".") {
				parts := strings.SplitN(amountStr, ".", 2)
				var ok bool
				wholeTokens, ok = wholeTokens.SetString(parts[0], 10)
				if !ok {
					return nil, fmt.Errorf("parse whole token amount %q for transaction hash %q: invalid integer", parts[0], txHash)
				}

				fracTokensString := parts[1]
				// Trim off any trailing zeroes to avoid over-expanding
				fracTokensString = strings.TrimRight(fracTokensString, "0")

				fracTokensLength = len(fracTokensString)
				fracTokens, ok = fracTokens.SetString(fracTokensString, 10)
				if !ok {
					return nil, fmt.Errorf("parse fractional token amount %q for transaction hash %q: invalid integer", parts[1], txHash)
				}
			} else {
				var ok bool
				wholeTokens, ok = wholeTokens.SetString(amountStr, 10)
				if !ok {
					return nil, fmt.Errorf("parse whole token amount %q for transaction hash %q: invalid integer", amountStr, txHash)
				}
			}

			// Expand the whole tokens out to base units
			if wholeTokens.Cmp(big.NewInt(0)) == 1 {
				totalAmount = new(big.Int).Mul(wholeTokens, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tokenDetails.Decimals)), nil))
			}

			if fracTokens.Cmp(big.NewInt(0)) == 1 {
				exponent := tokenDetails.Decimals - fracTokensLength
				if exponent < 0 {
					return nil, fmt.Errorf("fractional token amount %q for transaction hash %q has more decimal places than token supports", amountStr, txHash)
				}

				fracBaseUnits := new(big.Int).Mul(fracTokens, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil))
				totalAmount = totalAmount.Add(totalAmount, fracBaseUnits)
			}
		}

		// parse time using the known layouts
		executionTime, err := time.Parse("2006-01-02 15:04:05", timeStr)
		if err != nil {
			return nil, fmt.Errorf("parse execution time %q for transaction hash %q: %w", timeStr, txHash, err)
		}

		transfers = append(transfers, Transfer{
			FromAddress:     from,
			ToAddress:       to,
			Amount:          totalAmount,
			ExecutionTime:   executionTime,
			TransactionHash: txHash,
		})
	}

	return transfers, nil
}
