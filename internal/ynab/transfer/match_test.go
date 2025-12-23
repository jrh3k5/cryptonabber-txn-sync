package transfer_test

import (
	"math/big"
	"strings"
	"time"

	"github.com/jrh3k5/cryptonabber-txn-sync/internal/token"
	ttx "github.com/jrh3k5/cryptonabber-txn-sync/internal/transaction"
	clientpkg "github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/client"
	"github.com/jrh3k5/cryptonabber-txn-sync/internal/ynab/transfer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MatchTransfers", func() {
	When("there is a matching transfer", func() {
		It("matches outbound transfer by date, address, and amount", func() {
			date := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
			ynabTxn := &clientpkg.Transaction{
				ID:     "test-txn",
				Amount: -1000,
				Date:   date,
			}

			tr := &ttx.Transfer{
				FromAddress:     strings.ToLower("0xAbc"),
				ToAddress:       "0xother",
				Amount:          big.NewInt(1000000), // decimals 6 -> $1 == 1_000_000
				ExecutionTime:   date.Add(3 * time.Hour),
				TransactionHash: "0xhash",
			}

			tokenDetails := &token.Details{Decimals: 6}

			matches := transfer.MatchTransfers(ynabTxn, "0xABC", tokenDetails, []*ttx.Transfer{tr})
			Expect(matches).To(And(
				HaveLen(1),
				ContainElement(tr),
			))
		})

		When("it is an inbound transfer", func() {
			It("matches inbound transfer by date, address, and amount", func() {
				date := time.Date(2025, 12, 2, 0, 0, 0, 0, time.UTC)
				ynabTxn := &clientpkg.Transaction{
					ID:     "test-txn",
					Amount: 2000,
					Date:   date,
				}

				tr := &ttx.Transfer{
					FromAddress:     "0xother",
					ToAddress:       strings.ToLower("0xAbc"),
					Amount:          big.NewInt(2000000), // $2 -> 2 * 10^6
					ExecutionTime:   date.Add(5 * time.Hour),
					TransactionHash: "0xhash2",
				}

				tokenDetails := &token.Details{Decimals: 6}

				matches := transfer.MatchTransfers(
					ynabTxn,
					"0xabc",
					tokenDetails,
					[]*ttx.Transfer{tr},
				)
				Expect(matches).To(And(
					HaveLen(1),
					ContainElement(tr),
				))
			})
		})

		When("multiple transfers match", func() {
			It("returns all matching transfers", func() {
				date := time.Date(2025, 12, 2, 0, 0, 0, 0, time.UTC)
				ynabTxn := &clientpkg.Transaction{
					ID:     "test-txn",
					Amount: 2000,
					Date:   date,
				}

				match0 := &ttx.Transfer{
					FromAddress:     "0xother",
					ToAddress:       strings.ToLower("0xAbc"),
					Amount:          big.NewInt(2000000), // $2 -> 2 * 10^6
					ExecutionTime:   date.Add(5 * time.Hour),
					TransactionHash: "0xhash2",
				}

				match1 := &ttx.Transfer{
					FromAddress:     "0xother",
					ToAddress:       strings.ToLower("0xAbc"),
					Amount:          big.NewInt(2000000), // $2 -> 2 * 10^6
					ExecutionTime:   date.Add(10 * time.Hour),
					TransactionHash: "0xhash2-1",
				}

				nonMatch := &ttx.Transfer{
					FromAddress:     "0xother",
					ToAddress:       strings.ToLower("0xAbc"),
					Amount:          big.NewInt(1999999), // non-matching amount
					ExecutionTime:   date.Add(5 * time.Hour),
					TransactionHash: "0xhash2-nonmatch",
				}

				tokenDetails := &token.Details{Decimals: 6}

				matches := transfer.MatchTransfers(
					ynabTxn,
					"0xabc",
					tokenDetails,
					[]*ttx.Transfer{match0, nonMatch, match1},
				)
				Expect(matches).To(And(
					HaveLen(2),
					ContainElement(match0),
					ContainElement(match1),
				))
			})
		})
	})

	When("there is no matching transfer", func() {
		It("returns an empty slice", func() {
			date := time.Date(2025, 12, 3, 0, 0, 0, 0, time.UTC)
			ynabTxn := &clientpkg.Transaction{
				ID:     "test-txn",
				Amount: 1000,
				Date:   date,
			}

			tr := &ttx.Transfer{
				FromAddress:     "0xother",
				ToAddress:       strings.ToLower("0xAbc"),
				Amount:          big.NewInt(999999),
				ExecutionTime:   date,
				TransactionHash: "0xhash3",
			}

			tokenDetails := &token.Details{Decimals: 6}

			Expect(
				transfer.MatchTransfers(ynabTxn, "0xabc", tokenDetails, []*ttx.Transfer{tr}),
			).To(BeEmpty())
		})
	})

	Context("marginal date matching", func() {
		When("the date mismatches by more than 1 day", func() {
			It("returns no results", func() {
				ynabDate := time.Date(2025, 12, 4, 0, 0, 0, 0, time.UTC)
				trDate := ynabDate.Add(48 * time.Hour) // 2 days later

				ynabTxn := &clientpkg.Transaction{Amount: 1000, Date: ynabDate}
				tr := &ttx.Transfer{
					FromAddress:     strings.ToLower("0xAbc"),
					ToAddress:       "0xother",
					Amount:          big.NewInt(1000000),
					ExecutionTime:   trDate,
					TransactionHash: "0xhash4",
				}

				tokenDetails := &token.Details{Decimals: 6}
				Expect(
					transfer.MatchTransfers(ynabTxn, "0xabc", tokenDetails, []*ttx.Transfer{tr}),
				).To(BeEmpty())
			})
		})

		When("the transfer date is within ±1 day of the YNAB date", func() {
			When("the transfer is 1 day after YNAB date", func() {
				It("matches", func() {
					// Simulates scenario: user in PST records txn on 12/14,
					// but actual UTC time is early hours of 12/15
					ynabDate := time.Date(2025, 12, 14, 0, 0, 0, 0, time.UTC)
					trDate := time.Date(
						2025,
						12,
						15,
						7,
						0,
						0,
						0,
						time.UTC,
					) // next day, early morning UTC

					ynabTxn := &clientpkg.Transaction{
						ID:     "test-txn-tolerance",
						Amount: -1500,
						Date:   ynabDate,
					}

					tr := &ttx.Transfer{
						FromAddress:     strings.ToLower("0xAbc"),
						ToAddress:       "0xother",
						Amount:          big.NewInt(1500000),
						ExecutionTime:   trDate,
						TransactionHash: "0xhash-tolerance",
					}

					tokenDetails := &token.Details{Decimals: 6}
					matches := transfer.MatchTransfers(
						ynabTxn,
						"0xabc",
						tokenDetails,
						[]*ttx.Transfer{tr},
					)
					Expect(matches).To(And(
						HaveLen(1),
						ContainElement(tr),
					))
				})
			})

			When("the transfer is 1 day before YNAB date", func() {
				It("matches", func() {
					// Simulates reverse scenario: YNAB records on 12/15,
					// but actual UTC time was late hours of 12/14
					ynabDate := time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)
					trDate := time.Date(
						2025,
						12,
						14,
						23,
						30,
						0,
						0,
						time.UTC,
					) // previous day, late evening UTC

					ynabTxn := &clientpkg.Transaction{
						ID:     "test-txn-tolerance-before",
						Amount: 2500,
						Date:   ynabDate,
					}

					tr := &ttx.Transfer{
						FromAddress:     "0xother",
						ToAddress:       strings.ToLower("0xAbc"),
						Amount:          big.NewInt(2500000),
						ExecutionTime:   trDate,
						TransactionHash: "0xhash-tolerance-before",
					}

					tokenDetails := &token.Details{Decimals: 6}
					matches := transfer.MatchTransfers(
						ynabTxn,
						"0xabc",
						tokenDetails,
						[]*ttx.Transfer{tr},
					)
					Expect(matches).To(And(
						HaveLen(1),
						ContainElement(tr),
					))
				})
			})
		})

		When("the dates differ by more than 1 day", func() {
			It("returns no matches", func() {
				ynabDate := time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC)
				trDate := time.Date(2025, 12, 12, 12, 0, 0, 0, time.UTC) // 2 days later

				ynabTxn := &clientpkg.Transaction{
					ID:     "test-txn-2days",
					Amount: 1000,
					Date:   ynabDate,
				}

				tr := &ttx.Transfer{
					FromAddress:     "0xother",
					ToAddress:       strings.ToLower("0xAbc"),
					Amount:          big.NewInt(1000000),
					ExecutionTime:   trDate,
					TransactionHash: "0xhash-2days",
				}

				tokenDetails := &token.Details{Decimals: 6}
				matches := transfer.MatchTransfers(
					ynabTxn,
					"0xabc",
					tokenDetails,
					[]*ttx.Transfer{tr},
				)
				Expect(matches).To(BeEmpty())
			})
		})
	})
})
