package transfer_test

import (
	"math/big"
	"strings"
	"time"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	ttx "github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
	clientpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/transfer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MatchTransfer", func() {
	It("matches outbound transfer by date, address, and amount", func() {
		date := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
		ynabTxn := &clientpkg.Transaction{
			ID:     "test-txn",
			Amount: -1000,
			Date:   date,
		}

		tr := &ttx.Transfer{
			FromAddress:     strings.ToLower("0xAbc"),
			ToAddress:       "0xother",
			Amount:          big.NewInt(1000000), // decimals 6 -> $1 == 1_000_000
			ExecutionTime:   date.Add(3 * time.Hour),
			TransactionHash: "0xhash",
		}

		tokenDetails := &token.Details{Decimals: 6}

		got := transfer.MatchTransfer(ynabTxn, "0xABC", tokenDetails, []*ttx.Transfer{tr})
		Expect(got).To(Equal(tr))
	})

	It("matches inbound transfer by date, address, and amount", func() {
		date := time.Date(2025, 12, 2, 0, 0, 0, 0, time.UTC)
		ynabTxn := &clientpkg.Transaction{
			ID:     "test-txn",
			Amount: 2000,
			Date:   date,
		}

		tr := &ttx.Transfer{
			FromAddress:     "0xother",
			ToAddress:       strings.ToLower("0xAbc"),
			Amount:          big.NewInt(2000000), // $2 -> 2 * 10^6
			ExecutionTime:   date.Add(5 * time.Hour),
			TransactionHash: "0xhash2",
		}

		tokenDetails := &token.Details{Decimals: 6}

		got := transfer.MatchTransfer(ynabTxn, "0xabc", tokenDetails, []*ttx.Transfer{tr})
		Expect(got).To(Equal(tr))
	})

	It("returns nil when no matching amount", func() {
		date := time.Date(2025, 12, 3, 0, 0, 0, 0, time.UTC)
		ynabTxn := &clientpkg.Transaction{
			ID:     "test-txn",
			Amount: 1000,
			Date:   date,
		}

		tr := &ttx.Transfer{
			FromAddress:     "0xother",
			ToAddress:       strings.ToLower("0xAbc"),
			Amount:          big.NewInt(999999),
			ExecutionTime:   date,
			TransactionHash: "0xhash3",
		}

		tokenDetails := &token.Details{Decimals: 6}

		got := transfer.MatchTransfer(ynabTxn, "0xabc", tokenDetails, []*ttx.Transfer{tr})
		Expect(got).To(BeNil())
	})

	It("returns nil when date mismatches", func() {
		ynabDate := time.Date(2025, 12, 4, 0, 0, 0, 0, time.UTC)
		trDate := ynabDate.Add(24 * time.Hour)

		ynabTxn := &clientpkg.Transaction{Amount: 1000, Date: ynabDate}
		tr := &ttx.Transfer{
			FromAddress:     strings.ToLower("0xAbc"),
			ToAddress:       "0xother",
			Amount:          big.NewInt(1000000),
			ExecutionTime:   trDate,
			TransactionHash: "0xhash4",
		}

		tokenDetails := &token.Details{Decimals: 6}
		got := transfer.MatchTransfer(ynabTxn, "0xabc", tokenDetails, []*ttx.Transfer{tr})
		Expect(got).To(BeNil())
	})
})
