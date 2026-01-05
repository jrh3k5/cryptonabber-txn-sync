# Cryptonabber Transaction Sync

This is a tool used to synchronize transactions onchain with transactions in YNAB. By default, this is intended to be used to synchronize USD transactions in YNAB with USDC (a USD-pegged stablecoin). For transactions that it can match, it will mark them as cleared and store the transaction hash in the memo of the transaction.

## Usage

This tool requires an export of data from an Etherscan explorer (e.g., https://basescan.org) of the transactions for the account and token.

### Command

An example usage of this is:

```
./cryptonabber-txn-sync \
  --ynab-access-token=<access token> \
  --csv-file=/path/to/etherscan_export.csv \
  --wallet-address=0x9134fc7112b478e97eE6F0E6A7bf81EcAfef19ED \
  --ynab-account-name="<name of account in YNAB>"
```

The tool will read the CSV, fetch token details from the RPC endpoint, find uncleared transactions in YNAB, and attempt to match and clear them while appending the matching transaction hash to the memo.

#### Command-line Arguments

- **--ynab-access-token**: (required) YNAB Personal Access Token used to authenticate requests to the YNAB API.
- **--csv-file**: (required) Path to an Etherscan CSV file containing token transfers (used to find matching on-chain transfers).
- **--wallet-address**: (required) The wallet address to match transfers against (case-insensitive).
- **--ynab-account-name**: (required) The name of the account as it appears in YNAB to which transactions are to be synchronized.
- **--rpc-url**: (optional) The JSON-RPC endpoint to use for token metadata lookups. Defaults to `https://mainnet.base.org`.
- **--token-address**: (optional) The token contract address to sync. Defaults to the USDC address configured in the project.
- **--match-since**: (optional) The date from which to read transactions from YNAB; takes the form of YYYY-MM-DD. If not provided, then the application defaults to seven days prior to the current date.
