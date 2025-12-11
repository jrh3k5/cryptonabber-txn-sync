package transaction_test

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/csv"
	"fmt"
	"math/big"
	"time"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	transactionpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		transfers, err := transactionpkg.TransfersFromEtherscanCSV(
			context.Background(),
			usdcDetails,
			reader,
		)
		Expect(err).ToNot(HaveOccurred(), "parsing the CSV file should not fail")

		// successful parse: perform basic sanity checks
		Expect(transfers).ToNot(BeEmpty())
		first := transfers[0]
		Expect(
			first.TransactionHash,
		).To(Equal("0x3fe67569dfcce1fe4afca58819da01f423b2cb67d61ee3ba1ed413d2612717c7"))
		Expect(first.FromAddress).To(Equal("0x9134fc7112b478e97eE6F0E6A7bf81EcAfef19ED"))
		Expect(first.ToAddress).To(Equal("0xC8B0C609712aa852B1E390deD058276fa9bc36f1"))
		Expect(first.Amount).To(Equal(big.NewInt(101500000))) // 101.5 * 10^6

		expectedExecutionTime, err := time.Parse(time.RFC3339, "2025-12-10T11:53:23Z")
		Expect(err).ToNot(HaveOccurred(), "parsing expected transfer time should not fail")
		Expect(
			first.ExecutionTime,
		).To(Equal(expectedExecutionTime), "transfer execution time should match expected value")
	})

	DescribeTable("missing required column", func(missingColumn string) {
		allColumns := []string{
			"Transaction Hash",
			"From",
			"To",
			"Amount",
			"DateTime (UTC)",
		}

		row := []string{
			"0xhash",
			"0xfrom",
			"0xto",
			"1000000",
			"2025-12-10 11:53:23",
		}

		toRemoveIndex := -1
		for i, col := range allColumns {
			if col == missingColumn {
				toRemoveIndex = i
				break
			}
		}
		Expect(
			toRemoveIndex,
		).ToNot(Equal(-1), "column '%s' to remove should be found", missingColumn)

		// build CSV data with the specified column removed
		var buffer bytes.Buffer
		writer := csv.NewWriter(&buffer)
		var header []string
		var data []string
		for i, col := range allColumns {
			if i != toRemoveIndex {
				header = append(header, col)
				data = append(data, row[i])
			}
		}
		Expect(writer.Write(header)).To(Succeed(), "writing CSV header should succeed")
		Expect(writer.Write(data)).To(Succeed(), "writing CSV data should succeed")
		writer.Flush()
		Expect(writer.Error()).ToNot(HaveOccurred(), "flushing CSV writer should not fail")

		_, err := transactionpkg.TransfersFromEtherscanCSV(
			context.Background(),
			usdcDetails,
			&buffer,
		)
		Expect(
			err,
		).To(MatchError(ContainSubstring(fmt.Sprintf("CSV is missing required column: %s", missingColumn))), "parsing CSV with missing column '%s' should fail", missingColumn)
	},
		Entry("Transaction Hash", "Transaction Hash"),
		Entry("From", "From"),
		Entry("To", "To"),
		Entry("Amount", "Amount"),
		Entry("DateTime (UTC)", "DateTime (UTC)"),
	)
})
