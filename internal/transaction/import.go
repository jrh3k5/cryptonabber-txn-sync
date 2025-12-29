package transaction

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
	"github.com/manifoldco/promptui"
)

type importTransferAction string

const (
	importTransferActionCreate importTransferAction = "create"
	importTransferActionSkip   importTransferAction = "skip"
	importTransferActionIgnore importTransferAction = "ignore"
)

type transferImporter struct {
	httpClient      *http.Client
	ynabAccessToken string
	budgetID        string
	accountID       string
	tokenDetails    *token.Details
	walletAddress   string
	ignoreList      *IgnoreList
	minimumAmount   *big.Int
}

func newTransferImporter(
	httpClient *http.Client,
	ynabAccessToken string,
	budgetID string,
	accountID string,
	tokenDetails *token.Details,
	walletAddress string,
	ignoreList *IgnoreList,
) (*transferImporter, error) {
	decimalPrecision := 2

	tokenDecimals := tokenDetails.Decimals
	if tokenDecimals < decimalPrecision {
		return nil, fmt.Errorf(
			"tokens with fewer than 2 decimals (%d) are not supported",
			tokenDecimals,
		)
	}

	// Minimum amount threshold in base units: 10^(decimals-2) => 0.01 token
	minimumAmount := big.NewInt(1)
	//nolint:mnd
	minimumAmount.Exp(
		big.NewInt(10),
		big.NewInt(int64(tokenDecimals-decimalPrecision)),
		nil,
	)

	return &transferImporter{
		httpClient:      httpClient,
		ynabAccessToken: ynabAccessToken,
		budgetID:        budgetID,
		accountID:       accountID,
		tokenDetails:    tokenDetails,
		walletAddress:   walletAddress,
		ignoreList:      ignoreList,
		minimumAmount:   minimumAmount,
	}, nil
}

func (p *transferImporter) processTransfers(
	ctx context.Context,
	transfers []*Transfer,
) error {
	for _, xfr := range transfers {
		if err := p.processTransfer(ctx, xfr); err != nil {
			if errors.Is(err, errUserCanceled) {
				return err
			}
			// Log error and continue with next transfer
			slog.ErrorContext(ctx, "Failed to process transfer", "error", err)

			continue
		}
	}

	return nil
}

var errUserCanceled = errors.New("user canceled operation")

func (p *transferImporter) processTransfer(
	ctx context.Context,
	xfr *Transfer,
) error {
	isOutbound, counterparty, ok := p.determineDirection(xfr)
	if !ok {
		// Not related to the wallet; skip
		return nil
	}

	if p.isBelowMinimum(ctx, xfr) {
		return nil
	}

	// Ask user if they want to create a transaction
	importAction, err := p.promptCreateTransaction(xfr, isOutbound, counterparty)
	if err != nil {
		return err
	}

	switch importAction {
	case importTransferActionSkip:
		// User chose to skip; do nothing
		return nil
	case importTransferActionIgnore:
		// User chose to ignore; add to ignore list
		slog.DebugContext(
			ctx,
			"Ignoring transfer permanently",
			"transaction_hash",
			xfr.TransactionHash,
		)

		p.ignoreList.AddIgnoredHash(xfr.TransactionHash)

		return nil
	case importTransferActionCreate:
		// Proceed to create the transaction
	default:
		return fmt.Errorf("unknown import action: %s", importAction)
	}

	// Get transaction details from user
	payeeName, memoText, err := p.promptTransactionDetails(xfr, counterparty)
	if err != nil {
		return err
	}

	// Create the YNAB transaction
	return p.createYNABTransaction(ctx, xfr, isOutbound, payeeName, memoText)
}

func (p *transferImporter) determineDirection(
	xfr *Transfer,
) (bool, string, bool) {
	switch {
	case strings.EqualFold(xfr.FromAddress, p.walletAddress):
		return true, xfr.ToAddress, true
	case strings.EqualFold(xfr.ToAddress, p.walletAddress):
		return false, xfr.FromAddress, true
	default:
		return false, "", false
	}
}

func (p *transferImporter) isBelowMinimum(
	ctx context.Context,
	xfr *Transfer,
) bool {
	if xfr.Amount.Cmp(p.minimumAmount) < 0 {
		slog.DebugContext(
			ctx,
			fmt.Sprintf(
				"transaction with hash '%s' and amount %s is less than the minimum (%s)",
				xfr.TransactionHash,
				xfr.Amount.Text(10),      //nolint:mnd
				p.minimumAmount.Text(10), //nolint:mnd
			),
		)

		return true
	}

	return false
}

// promptCreateTransaction prompts the user to decide whether to create a YNAB transaction for the given transfer.
func (p *transferImporter) promptCreateTransaction(
	xfr *Transfer,
	isOutbound bool,
	counterparty string,
) (importTransferAction, error) {
	details := p.formatTransferDetails(xfr, isOutbound, counterparty)

	createOption := "Create"
	skipOption := "Skip (for now)"
	ignoreOption := "Ignore (skip permanently)"

	selector := promptui.Select{
		Label: "Create YNAB transaction for " + details + "?",
		Items: []string{createOption, skipOption, ignoreOption},
	}

	selIdx, _, err := selector.Run()
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) || errors.Is(err, promptui.ErrEOF) {
			return importTransferActionSkip, errUserCanceled
		}

		return importTransferActionSkip, fmt.Errorf("transaction creation prompt failed: %w", err)
	}

	switch selIdx {
	case 0:
		return importTransferActionCreate, nil
	case 1:
		return importTransferActionSkip, nil
	case 2: //nolint:mnd
		return importTransferActionIgnore, nil
	default:
		return importTransferActionSkip, fmt.Errorf("invalid selection index: %d", selIdx)
	}
}

func (p *transferImporter) formatTransferDetails(
	xfr *Transfer,
	isOutbound bool,
	counterparty string,
) string {
	sign := "+"
	if isOutbound {
		sign = "-"
	}

	return fmt.Sprintf(
		"%s %s %s on %s %s %s",
		sign,
		xfr.FormatAmount(p.tokenDetails.Decimals),
		p.tokenDetails.Name,
		xfr.ExecutionTime.Format(time.RFC3339),
		ResolveDirection(isOutbound),
		counterparty,
	)
}

func (p *transferImporter) promptTransactionDetails(
	xfr *Transfer,
	counterparty string,
) (string, string, error) {
	payeeName, err := p.promptPayeeName(counterparty)
	if err != nil {
		return "", "", err
	}

	memoText, err := p.promptMemo(xfr)
	if err != nil {
		return "", "", err
	}

	return payeeName, memoText, nil
}

func (p *transferImporter) promptPayeeName(defaultPayee string) (string, error) {
	payeePrompt := promptui.Prompt{
		Label:   "Payee name",
		Default: defaultPayee,
	}

	payeeName, err := payeePrompt.Run()
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) || errors.Is(err, promptui.ErrEOF) {
			return "", errUserCanceled
		}

		return "", fmt.Errorf("payee prompt failed: %w", err)
	}

	return payeeName, nil
}

func (p *transferImporter) promptMemo(xfr *Transfer) (string, error) {
	memoPrompt := promptui.Prompt{
		Label:   "Memo (will auto-append transaction hash)",
		Default: xfr.TransactionHash,
	}

	memoText, err := memoPrompt.Run()
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) || errors.Is(err, promptui.ErrEOF) {
			return "", errUserCanceled
		}

		return "", fmt.Errorf("memo prompt failed: %w", err)
	}

	if !strings.Contains(memoText, xfr.TransactionHash) {
		memoText += "; transaction hash: " + xfr.TransactionHash
	}

	return memoText, nil
}

func (p *transferImporter) createYNABTransaction(
	ctx context.Context,
	xfr *Transfer,
	isOutbound bool,
	payeeName string,
	memoText string,
) error {
	amountInt64, err := p.convertToYNABAmount(xfr.Amount, isOutbound)
	if err != nil {
		return err
	}

	cleared := "uncleared"
	req := client.CreateTransactionRequest{
		AccountID: p.accountID,
		Date:      xfr.ExecutionTime,
		Amount:    amountInt64,
		PayeeName: &payeeName,
		Memo:      &memoText,
		Cleared:   &cleared,
	}

	created, err := client.CreateTransaction(ctx, p.httpClient, p.ynabAccessToken, p.budgetID, req)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	slog.InfoContext(
		ctx,
		"Created YNAB transaction",
		"id",
		created.ID,
		"amount",
		created.GetFormattedAmount(),
		"payee",
		created.Payee,
	)

	return nil
}

func (p *transferImporter) convertToYNABAmount(amount *big.Int, isOutbound bool) (int64, error) {
	// Convert token amount (base units) to YNAB milliunits
	// milliunits = amount_base_units * 1000 / 10^decimals
	//nolint:mnd
	num := new(big.Int).Mul(amount, big.NewInt(1000))
	//nolint:mnd
	denom := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(p.tokenDetails.Decimals)), nil)
	ynabMilli := new(big.Int).Quo(num, denom)

	if isOutbound {
		ynabMilli.Neg(ynabMilli)
	}

	// Sanity check: ensure ynabMilli fits in int64
	//nolint:mnd
	if ynabMilli.BitLen() > 63 {
		return 0, fmt.Errorf("computed amount exceeds int64: %s", ynabMilli.String())
	}

	return ynabMilli.Int64(), nil
}

func ImportRemainingTransfers(
	ctx context.Context,
	httpClient *http.Client,
	ynabAccessToken string,
	budgetID string,
	accountID string,
	transfers []*Transfer,
	tokenDetails *token.Details,
	walletAddress string,
	ignoreList *IgnoreList,
) error {
	processor, err := newTransferImporter(
		httpClient,
		ynabAccessToken,
		budgetID,
		accountID,
		tokenDetails,
		walletAddress,
		ignoreList,
	)
	if err != nil {
		return err
	}

	return processor.processTransfers(ctx, transfers)
}
