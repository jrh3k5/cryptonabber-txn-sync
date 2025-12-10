package transaction

import (
	"math/big"
	"time"
)

type Transfer struct {
	FromAddress     string    // the address that sent the token, encoded in hex
	ToAddress       string    // the address that received the token, encoded in hex
	Amount          *big.Int  // the amount of tokens transferred, in the token's base unit
	ExecutionTime   time.Time // the time the transaction was executed
	TransactionHash string    // the hash of the transaction, encoded in hex
}
