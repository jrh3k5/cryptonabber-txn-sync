package ynab

import (
	"fmt"
	"strings"

	"github.com/davidsteinsland/ynab-go/ynab"
)

const (
	transactionStatusCleared = "cleared"
)

// GetUnclearedTransactions retrieves all transactions for the specified budget ID
// and account name that are not marked as "cleared".
func GetUnclearedTransactions(
	client *ynab.Client,
	budgetID string,
	accountName string,
) ([]ynab.TransactionDetail, error) {
	budget, err := client.BudgetService.Get(budgetID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve budget for ID '%s'", budgetID)
	}

	var account *ynab.Account
	for _, acc := range budget.Accounts {
		if acc.Name == accountName {
			account = &acc

			break
		}
	}
	if account == nil {
		return nil, fmt.Errorf("account '%q' not found", accountName)
	}

	transactions, err := client.TransactionsService.GetByAccount(budgetID, account.Id)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get transactions for account ID '%s' under budget ID '%s'",
			account.Id,
			budgetID,
		)
	}

	var uncleared []ynab.TransactionDetail
	for _, txn := range transactions {
		if !strings.EqualFold(txn.Cleared, transactionStatusCleared) {
			uncleared = append(uncleared, txn)
		}
	}

	return uncleared, nil
}
