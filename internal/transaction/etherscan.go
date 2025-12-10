package transaction

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"
)

// TransfersFromEtherscanCSV parses the given Etherscan CSV data and returns a slice of Transfers.
// It expects the given reader to contain a CSV with the following columns in no given order:
// - Transaction Hash, which is the hash of the transaction in hex
// - From, which is the address that sent the token in hex
// - To, which is the address that received the token in hex
// - Amount, which is the amount of tokens transferred in the token's base unit
// - DateTime (UTC), which is the time the transaction was executed in UTC
func TransfersFromEtherscanCSV(ctx context.Context, csvReader io.Reader) ([]Transfer, error) {
	r := csv.NewReader(csvReader)
	r.TrimLeadingSpace = true

	// read header
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}

	// map header names (lowercased, trimmed) to indices
	hdrIdx := make(map[string]int)
	for i, h := range header {
		key := strings.TrimSpace(strings.ToLower(h))
		hdrIdx[key] = i
	}

	// helper to find a header by acceptable names
	find := func(names ...string) (int, bool) {
		for _, n := range names {
			if idx, ok := hdrIdx[strings.ToLower(n)]; ok {
				return idx, true
			}
		}
		return 0, false
	}

	txIdx, txFound := find("transaction hash", "txn hash", "hash")
	fromIdx, fromFound := find("from")
	toIdx, toFound := find("to")
	amountIdx, amountFound := find("amount")
	timeIdx, timeFound := find("datetime (utc)", "date (utc)", "dateutc", "datetime")

	if !(txFound && fromFound && toFound && amountFound && timeFound) {
		return nil, errors.New("csv is missing one or more required columns: Transaction Hash, From, To, Amount, DateTime (UTC)")
	}

	var transfers []Transfer

	// try multiple time layouts
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"1/2/2006 15:04:05",
		"1/2/2006 3:04:05 PM",
	}

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv record: %w", err)
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
		amt := new(big.Int)
		if amountStr == "" {
			amt = nil
		} else {
			// remove commas if present
			cleaned := strings.ReplaceAll(amountStr, ",", "")
			if _, ok := amt.SetString(cleaned, 10); !ok {
				return nil, fmt.Errorf("parse amount %q: invalid integer", amountStr)
			}
		}

		// parse time using the known layouts
		var execTime time.Time
		var tErr error
		for _, lay := range layouts {
			execTime, tErr = time.Parse(lay, timeStr)
			if tErr == nil {
				break
			}
		}
		if tErr != nil {
			return nil, fmt.Errorf("parse time %q: %w", timeStr, tErr)
		}

		transfers = append(transfers, Transfer{
			FromAddress:     from,
			ToAddress:       to,
			Amount:          amt,
			ExecutionTime:   execTime,
			TransactionHash: txHash,
		})
	}

	return transfers, nil
}
