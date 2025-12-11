package transaction_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEtherscanCSV(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etherscan CSV Suite")
}
