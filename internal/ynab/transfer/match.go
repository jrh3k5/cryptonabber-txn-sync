package transfer

import (
	"math/big"
	"strings"
	"time"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
)

func MatchTransfer(
	ynabTransaction *client.Transaction,
	address string,
	tokenDetails *token.Details,
	transfers []*transaction.Transfer,
) *transaction.Transfer {
	if tokenDetails == nil {
		return nil
	}

	addr := strings.ToLower(address)

	// ynabTransaction.Amount is in tenths of cents (1000 == $1)
	absAmt := ynabTransaction.Amount
	if absAmt < 0 {
		absAmt = -absAmt
	}

	// scale = 10^decimals
	//nolint:mnd
	scale := new(
		big.Int,
	).Exp(big.NewInt(10), big.NewInt(int64(tokenDetails.Decimals)), nil)
	tmp := new(big.Int).Mul(big.NewInt(absAmt), scale)
	expected := new(big.Int).Div(tmp, big.NewInt(1000)) //nolint:mnd

	for _, tr := range transfers {
		if !sameDate(tr.ExecutionTime, ynabTransaction.Date) {
			continue
		}

		if ynabTransaction.Amount < 0 {
			// outbound: match from address
			if strings.ToLower(tr.FromAddress) != addr {
				continue
			}
		} else {
			// inbound: match to address
			if strings.ToLower(tr.ToAddress) != addr {
				continue
			}
		}

		if tr.Amount == nil {
			continue
		}

		if expected.Cmp(tr.Amount) == 0 {
			return tr
		}
	}

	return nil
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}
