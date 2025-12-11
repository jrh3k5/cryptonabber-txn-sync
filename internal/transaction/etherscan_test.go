package transaction_test

import (
	"bytes"
	"context"
	"math/big"
	"time"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	transactionpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	_ "embed"
)

//go:embed test_etherscan_usdc_export.csv
var etherscanUSDCExportCSV string

var _ = Describe("TransfersFromEtherscanCSV", func() {
	var usdcDetails *token.Details

	BeforeEach(func() {
		usdcDetails = &token.Details{
			Decimals: 6,
		}
	})

	It("parses the provided Etherscan CSV", func() {
		reader := bytes.NewBufferString(etherscanUSDCExportCSV)
		transfers, err := transactionpkg.TransfersFromEtherscanCSV(context.Background(), usdcDetails, reader)
		Expect(err).ToNot(HaveOccurred(), "parsing the CSV file should not fail")

		// successful parse: perform basic sanity checks
		Expect(transfers).ToNot(BeEmpty())
		first := transfers[0]
		Expect(first.TransactionHash).To(Equal("0x3fe67569dfcce1fe4afca58819da01f423b2cb67d61ee3ba1ed413d2612717c7"))
		Expect(first.FromAddress).To(Equal("0x9134fc7112b478e97eE6F0E6A7bf81EcAfef19ED"))
		Expect(first.ToAddress).To(Equal("0xC8B0C609712aa852B1E390deD058276fa9bc36f1"))
		Expect(first.Amount).To(Equal(big.NewInt(10150000))) // 101.5 * 10^6

		expectedTransferTime, err := time.Parse(time.RFC3339, "2025-12-10T11:53:23Z")
		Expect(err).ToNot(HaveOccurred(), "parsing expected transfer time should not fail")
		Expect(first.ExecutionTime).To(Equal(expectedTransferTime), "transfer execution time should match expected value")
	})
})
