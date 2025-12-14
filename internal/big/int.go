package big

import (
	"fmt"
	"math/big"
	"strings"
)

const (
	base10 = 10
)

// BigIntFromString converts a string to a *big.Int.
func BigIntFromString(s string) (*big.Int, error) {
	// allow common thousands separators (commas, underscores and spaces)
	sanitized := strings.ReplaceAll(s, ",", "")
	sanitized = strings.ReplaceAll(sanitized, "_", "")
	sanitized = strings.ReplaceAll(sanitized, " ", "")

	bigInt, isValid := new(big.Int).SetString(sanitized, base10)
	if !isValid {
		return nil, fmt.Errorf("invalid integer string: %s", s)
	}

	return bigInt, nil
}
