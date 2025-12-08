package transaction

import "context"

type Service interface {
	GetTransactions(ctx context.Context, accountAddress string, contractAddress string, offset int, max int) ([]Transaction, error)
}
