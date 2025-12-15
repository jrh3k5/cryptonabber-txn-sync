# Cryptonabber Transaction Sync

This is a tool used to synchronize transactions onchain with transactions in YNAB. By default, this is intended to be used to synchronize USD transactions in YNAB with USDC (a USD-pegged stablecoin). For transactions that it can match, it will mark them as cleared and store the transaction hash in the memo of the transaction.