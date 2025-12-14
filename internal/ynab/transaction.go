package ynab

import (
	"fmt"
	"strings"
	"time"

	"github.com/seanag0234/go-ynab"
	"github.com/seanag0234/go-ynab/api"
	"github.com/seanag0234/go-ynab/api/account"
	"github.com/seanag0234/go-ynab/api/transaction"
)

const (
	transactionStatusCleared = "cleared"
)

// GetUnclearedTransactions retrieves all transactions for the specified budget ID
// and account name that are not marked as "cleared".
func GetUnclearedTransactions(
	client ynab.ClientServicer,
	budgetID string,
	accountName string,
) ([]*transaction.Transaction, error) {
	budget, err := client.Budget().GetBudget(budgetID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve budget for ID '%s'", budgetID)
	}

	var account *account.Account
	for _, budgetAccount := range budget.Budget.Accounts {
		if budgetAccount.Name == accountName {
			account = budgetAccount

			break
		}
	}
	if account == nil {
		return nil, fmt.Errorf("account '%q' not found in budget ID '%s'", accountName, budgetID)
	}

	transactions, err := client.Transaction().GetTransactionsByAccount(budgetID, account.ID, &transaction.Filter{
		Since: &api.Date{
			Time: time.Now().Add(-7 * 24 * time.Hour),
		},
	})
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get transactions for account ID '%s' under budget ID '%s'",
			account.ID,
			budgetID,
		)
	}

	var uncleared []*transaction.Transaction
	for _, txn := range transactions {
		if !strings.EqualFold(txn., transactionStatusCleared) {
			uncleared = append(uncleared, txn)
		}
	}

	return uncleared, nil
}
