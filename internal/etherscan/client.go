package etherscan

import "context"

// Client defines the interface for interacting with the Etherscan API.
// An individual client instance is bound to a particular chain.
type Client interface {
	// GetTransactions retrieves token transfer transactions for the specified account address and contract address.
	// The offset is the number of transactions to show in each page; the page is a zero-based index of the page to retrieve.
	GetERC20TokenTransferTransactions(ctx context.Context, accountAddress string, contractAddress string, page int, offset int) ([]ERC20TokenTransferTransaction, error)
}
