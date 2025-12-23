package transaction_test

import (
	"math/big"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transfer", func() {
	Context("FormatAmount", func() {
		DescribeTable("amount formatting", func(amount *big.Int, expectedFormatted string) {
			tr := &transaction.Transfer{
				Amount: amount,
			}
			formatted := tr.FormatAmount(6)
			Expect(formatted).To(Equal(expectedFormatted))
		}, Entry("nil amount", nil, "0"),
			Entry("zero amount", big.NewInt(0), "0"),
			Entry("whole number", big.NewInt(1000000), "1"),
			Entry("fractional amount", big.NewInt(1234567), "1.234567"),
			Entry("trailing zeros", big.NewInt(1200000), "1.2"),
			Entry("no fractional part", big.NewInt(5000000), "5"),
			Entry("small fractional part", big.NewInt(1), "0.000001"),
		)
	})
})
