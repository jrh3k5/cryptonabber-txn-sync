package etherscan

import (
	"math/big"
	"time"
)

// ERC20TokenTransferTransaction represents a token transfer transaction on the Ethereum blockchain.
type ERC20TokenTransferTransaction struct {
	Amount          *big.Int  // the amount of the token transferred in the transaction
	TransactionHash string    // the hash of the transaction, expressed as a hex-encoded string
	TransferTime    time.Time // the time the transfer occurred
}
