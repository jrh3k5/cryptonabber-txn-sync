package transaction

import (
	"math/big"
	"time"
)

// Transfer represents a transfer of tokens from one address to another.
type Transfer struct {
	FromAddress     string    // the address that sent the token, encoded in hex
	ToAddress       string    // the address that received the token, encoded in hex
	Amount          *big.Int  // the amount of tokens transferred, in the token's base unit
	ExecutionTime   time.Time // the time the transaction was executed
	TransactionHash string    // the hash of the transaction, encoded in hex
}

func (t *Transfer) FormatAmount(decimals int) string {
	if t.Amount == nil {
		return "0"
	}
	amount := new(big.Float).SetInt(t.Amount)
	denom := new(big.Float).SetFloat64(1)
	for range decimals {
		denom.Mul(denom, big.NewFloat(10)) //nolint:mnd
	}
	amount.Quo(amount, denom)
	s := amount.Text('f', decimals)
	// Trim trailing zeros and dot if needed
	for len(s) > 1 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	if len(s) > 1 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}

	return s
}
