package big_test

import (
	ctsbig "github.com/jrh3k5/cryptonabber-txn-sync/internal/big"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BigIntFromString", func() {
	It("parses numbers with commas, spaces and underscores", func() {
		cases := []struct {
			in  string
			exp string
		}{
			{"4,157", "4157"},
			{"-1,234", "-1234"},
			{"1 234", "1234"},
			{"1_234", "1234"},
		}

		for _, c := range cases {
			b, err := ctsbig.BigIntFromString(c.in)
			Expect(err).ToNot(HaveOccurred())
			Expect(b.String()).To(Equal(c.exp))
		}
	})
})
