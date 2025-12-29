package transaction_test

import (
	"bytes"
	"fmt"
	"time"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IgnoreList", func() {
	Context("AddProcessedHash", func() {
		It("adds a processed hash with reason, date, and added_on field", func() {
			ignoreList := transaction.NewIgnoreList()
			hash := "0xabc"
			txnID := "tx-123"
			today := time.Now().Format(time.DateOnly)

			ignoreList.AddProcessedHash(hash, txnID)

			hashes := ignoreList.GetHashes()
			Expect(hashes).To(HaveLen(1))
			Expect(hashes[0].Hash).To(Equal(hash))
			Expect(hashes[0].Reason).To(ContainSubstring(txnID))
			Expect(hashes[0].Reason).To(ContainSubstring(today))

			var buf bytes.Buffer
			err := transaction.ToYAML(ignoreList, &buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring(fmt.Sprintf("added_on: \"%s\"", today)))
		})

		It("does not add a duplicate processed hash", func() {
			ignoreList := transaction.NewIgnoreList()
			hash := "0xabc"

			ignoreList.AddProcessedHash(hash, "tx-1")
			ignoreList.AddProcessedHash(hash, "tx-2")

			Expect(ignoreList.GetHashCount()).To(Equal(1))
		})
	})

	Context("AddIgnoredHash", func() {
		It("adds an ignored hash with reason, date, and added_on field", func() {
			ignoreList := transaction.NewIgnoreList()
			hash := "0xdef"
			today := time.Now().Format(time.DateOnly)

			ignoreList.AddIgnoredHash(hash)

			hashes := ignoreList.GetHashes()
			Expect(hashes).To(HaveLen(1))
			Expect(hashes[0].Hash).To(Equal(hash))
			Expect(hashes[0].Reason).To(ContainSubstring("Marked as ignored on "))
			Expect(hashes[0].Reason).To(ContainSubstring(today))

			var buf bytes.Buffer
			err := transaction.ToYAML(ignoreList, &buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring(fmt.Sprintf("added_on: \"%s\"", today)))
		})

		It("does not add a duplicate ignored hash", func() {
			ignoreList := transaction.NewIgnoreList()
			hash := "0xdef"

			ignoreList.AddIgnoredHash(hash)
			ignoreList.AddIgnoredHash(hash)

			Expect(ignoreList.GetHashCount()).To(Equal(1))
		})
	})

	Context("FromYAML", func() {
		It("parses a valid YAML ignore list", func() {
			yaml := `ignored_hashes:
  - hash: "0x1234567890abcdef"
    reason: "test transaction"
  - hash: "0xfedcba0987654321"
    reason: "duplicate"`
			reader := bytes.NewReader([]byte(yaml))
			ignoreList, err := transaction.FromYAML(reader)

			Expect(err).NotTo(HaveOccurred())
			Expect(ignoreList).NotTo(BeNil())
		})

		It("returns an error for invalid YAML", func() {
			yaml := `ignored_hashes:
  - hash: "0x1234"
    reason: test
  invalid yaml structure [`
			reader := bytes.NewReader([]byte(yaml))
			ignoreList, err := transaction.FromYAML(reader)

			Expect(err).To(HaveOccurred())
			Expect(ignoreList).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to decode ignore list from YAML"))
		})

		It("handles an empty ignore list", func() {
			yaml := `ignored_hashes: []`
			reader := bytes.NewReader([]byte(yaml))
			ignoreList, err := transaction.FromYAML(reader)

			Expect(err).NotTo(HaveOccurred())
			Expect(ignoreList).NotTo(BeNil())
		})

		It("handles YAML with no ignored_hashes field", func() {
			yaml := `some_other_field: value`
			reader := bytes.NewReader([]byte(yaml))
			ignoreList, err := transaction.FromYAML(reader)

			Expect(err).NotTo(HaveOccurred())
			Expect(ignoreList).NotTo(BeNil())
		})
	})

	Context("ToYAML", func() {
		It("writes a valid YAML ignore list", func() {
			ignoreList := &transaction.IgnoreList{}
			var buf bytes.Buffer

			err := transaction.ToYAML(ignoreList, &buf)

			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("ignored_hashes"))
		})

		It("round-trips data from YAML to object and back to YAML", func() {
			originalYAML := `ignored_hashes:
  - hash: "0x1234567890abcdef"
    reason: "test transaction"
  - hash: "0xfedcba0987654321"
    reason: "duplicate"`
			// Read from YAML
			reader := bytes.NewReader([]byte(originalYAML))
			ignoreList, err := transaction.FromYAML(reader)
			Expect(err).NotTo(HaveOccurred())

			// Write back to YAML
			var buf bytes.Buffer
			err = transaction.ToYAML(ignoreList, &buf)
			Expect(err).NotTo(HaveOccurred())

			// Read again to verify
			reader2 := bytes.NewReader(buf.Bytes())
			ignoreList2, err := transaction.FromYAML(reader2)
			Expect(err).NotTo(HaveOccurred())
			Expect(ignoreList2).NotTo(BeNil())
		})
	})

	Context("Round-trip", func() {
		It("preserves data through YAML serialization and deserialization", func() {
			yaml := `ignored_hashes:
  - hash: "0xaabbccdd"
    reason: "first"
  - hash: "0xddeeffaa"
    reason: "second"`
			// Deserialize
			reader := bytes.NewReader([]byte(yaml))
			ignoreList, err := transaction.FromYAML(reader)
			Expect(err).NotTo(HaveOccurred())

			// Serialize back
			var buf bytes.Buffer
			err = transaction.ToYAML(ignoreList, &buf)
			Expect(err).NotTo(HaveOccurred())

			// Verify the output contains expected content
			output := buf.String()
			Expect(output).To(ContainSubstring("0xaabbccdd"))
			Expect(output).To(ContainSubstring("0xddeeffaa"))
			Expect(output).To(ContainSubstring("first"))
			Expect(output).To(ContainSubstring("second"))
		})
	})
})
