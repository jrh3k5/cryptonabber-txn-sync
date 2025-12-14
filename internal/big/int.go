package big

import (
	"fmt"
	"math/big"
)

const (
	base10 = 10
)

// BigIntFromString converts a string to a *big.Int.
func BigIntFromString(s string) (*big.Int, error) {
	bigInt, isValid := new(big.Int).SetString(s, base10)
	if !isValid {
		return nil, fmt.Errorf("invalid integer string: %s", s)
	}

	return bigInt, nil
}
