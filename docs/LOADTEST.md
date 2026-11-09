# Load test

`internal/terminal/loadtest_test.go` builds a big store and times what people
actually do. Run it with:

```sh
ZEBI_LOADTEST=1 ZEBI_LOAD_REPORT=report.md go test ./internal/terminal -run TestLoad -v -timeout 30m
```

Store: 50,000 products, 1,000,000 sales over 12 months (about 2 million sale
lines), 5 tills. Hub database about 2.5 GB. Measured on an Apple-silicon laptop
(9 Nov 2026); a typical shop PC will be slower, so targets leave room.

## Results

| What | Time | Target | Notes |
|---|---:|---:|---|
| Load 50000 products (setup only) | 1.308s | – |  |
| Load 1000000 sales over a year (setup only) | 1m31.642s | – |  |
| Build report summaries for existing sales (one-time, on upgrade) | 1m8.944s | – |  |
| First sync of 50000 products to 5 tills at once | 12.475s | < 3m0s |  |
| Till: barcode scan (100 lookups) | 13ms | < 500ms | per scan: see total/100 |
| Till: search "yellow rice" (20 searches) | 716ms | < 1s | 30 results |
| Till: search by brand attribute (20 searches) | 820ms | < 1s | 30 results |
| 5 tills × 200 sales, recorded and synced | 1.94s | < 30s | slowest sale at the till 12ms |
| Report: sales today by hour | 1ms | < 2s | 9 rows, 1901 sales |
| Report: last 30 days by day | 2ms | < 2s | 30 rows, 81362 sales |
| Report: last 30 days by item | 311ms | < 2s | 48088 rows, 81362 sales |
| Report: last 30 days by cashier | 3ms | < 2s | 2 rows, 81362 sales |
| Report: last 12 months by day | 23ms | < 2s | 366 rows, 1001000 sales |
| Report: last 12 months by category | 10ms | < 2s | 9 rows, 1001000 sales |
| Report: last 12 months by item | 924ms | < 2s | 50000 rows, 1001000 sales |
| Report: last 12 months by day, uneven times (no summaries) | 20.849s | < 30s | 366 rows, 1001000 sales |
| Report: stock levels (all items) | 110ms | < 2s | 5000 rows |
| Report: stock search "yellow" | 96ms | < 1s | 4001 rows |
| Report: shifts, last 30 days | 0s | < 1s | 5 rows |
| Hub: look up a receipt for a return | 0s | < 200ms |  |
| Hub: dashboard counts | 118ms | < 500ms |  |
| Hub: stock history of one item | 4ms | < 500ms | 37 movements |
| Hub: backup (full copy) | 19.317s | < 1m0s | 2044 MB |

Hub database: 2543 MB.

## What was slow, and the fix

| Before (phase 4) | Time | After |
|---|---:|---:|
| Sales report, last 12 months by day | 24.8 s | 23 ms |
| Sales report, last 12 months by category | 23.0 s | 10 ms |
| Sales report, last 12 months by item | 20.0 s | 0.9 s |
| Sales report, last 30 days by day | 1.4 s | 2 ms |
| Stock levels, all items | 4.8 s | 0.11 s |
| Stock search | 2.3 s | 0.10 s |

Reports used to add up every sale line in the period. The hub now keeps
**daily summaries** (by hour, cashier, till, location, customer; by item; by
category; by payment method), updated in the same transaction as each sale
and return. Reports for whole days (every period the app offers) read the
summaries; a report over arbitrary times still adds up the sale lines, and a
test checks both give identical figures for every grouping. The stock
report's "sold in 30 days" column reads the item summary too.

Upgrading a store builds the summaries once from its existing sales: about
a minute per million sales.

Fine as they are: syncing 50,000 products to 5 new tills at once (12 s, a
one-off), scanning a barcode (under 1 ms), searching at the till (about 40 ms),
5 tills selling at once (12 ms per sale at the till, all synced within 2 s),
and the daily backup of a 2 GB store (about 20 s, in the background).
