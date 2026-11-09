# Zebiventor

Progress and what comes next: [ROADMAP.md](ROADMAP.md).

Offline-first point of sale and inventory for shops with several tills on a
local network: bookshops, groceries, pharmacies, distributors.

- **Every till sells from its own SQLite database.** A sale never waits on the network.
- **One hub per shop** holds the store's records. Tills sync with it over the LAN
  and find it automatically (mDNS); if the network drops, they keep selling and
  catch up when it returns.
- **80mm/58mm thermal receipts** rendered at the print head's exact width, with
  a height that follows the content, sent as raw ESC/POS (₦ prints correctly).

## Topology

```
          ┌──────────────── Hub (one per shop) ────────────────┐
          │ hub.db (SQLite, WAL) ← single queued writer         │
          │ HTTP API :7420 · mDNS "_zebiventor._tcp"            │
          └──────▲──────────────────▲──────────────────▲────────┘
                 │ push sales        │ pull catalogue,  │
                 │ (outbox)          │ prices, users,   │
          ┌──────┴──────┐     ┌──────┴──────┐   stock (long-poll)
          │ Till T01    │     │ Till T02    │    …
          │ terminal.db │     │ terminal.db │
          └─────────────┘     └─────────────┘
```

The hub can run in any of three places:

| Where | How |
|---|---|
| Inside one of the tills | First-run setup → **Start a new store** (the app hosts the hub while it is open) |
| Headless on a back-office PC | `zebihub init …` then `zebihub service install && zebihub service start` |
| Headless on a till, in the background | Same as above on that till; the till app then **joins** `127.0.0.1` |

## How the important parts work

**Write queue (`internal/store`).** Each database has exactly one writer
connection, owned by one goroutine. All writes are queued and group-committed:
up to 64 jobs share a `BEGIN IMMEDIATE` transaction, each in its own
`SAVEPOINT`, so one failing job never affects the others. Reads use a separate
read-only pool and never block under WAL. Any number of tills can submit at once
without `SQLITE_BUSY`; the benchmark sustains ~250k small writes/sec on an M-series Mac.

**Sync (`internal/hub`, `internal/terminal`).**
- *Up:* a sale is committed locally together with an outbox op (ULID = idempotency
  key). The pusher sends pending ops; the hub applies each at most once
  (`inbox` table) and appends stock movements. The hub never refuses a completed sale
  for business reasons (it already happened); it flags anomalies in `audit_log`.
- *Down:* every master-data change bumps a revision in `changelog` (one row per
  entity). Tills long-poll `GET /api/sync/pull?since=REV`, so price changes
  reach every till in well under a second.
- *Stock* is an append-only ledger (`stock_movements`) plus running levels,
  so concurrent offline sales from many tills merge without conflicts.

**Point of sale (phase 2).**
- *Shifts:* every sale and refund belongs to a shift opened with a cash float.
  Closing is a blind count for cashiers (NGN note counter); the Z report shows
  expected vs counted cash and the variance, which is also flagged at the hub.
  Paid in / paid out are recorded per shift.
- *Manager approval:* discounts, price overrides, refunds, paid-outs and
  "no sale" drawer opens need a manager. A manager signs in on the cashier's
  till without ending their session, which issues a single-use, 2-minute
  approval that is consumed only when the action succeeds. The approver is stored on the record.
- *Tax:* none / inclusive / exclusive, with a rate per product. Cart discounts
  are spread across lines (largest remainder) before tax. `frontend/src/lib/pricing.ts`
  mirrors the Go maths exactly; a fixture test cross-checks 2,000 random carts.
- *Returns:* by receipt number or the receipt's QR code. Sales from this till
  work offline; other tills' sales are looked up at the hub, which also
  reports what was already returned, so the same item can't be refunded twice. Refunds are
  proportional to what was actually paid, optionally restocked, and can go
  back to a customer's account.
- *Customers:* created on any till (even offline) and shared with all tills
  (last write wins). Sales "on account" increase the balance, which the hub keeps.
- *Hold / recall:* park a cart on the till and recall it later (F8).

**Back office (phase 3).** Screens for managers and stock staff that work on
the hub's records, so they need the hub to be reachable. Selling stays offline-capable.
- *Purchasing:* suppliers, purchase orders (draft, then ordered, then part
  received or received), and goods received against an order or without one.
  Receiving updates stock straight away. Each item's cost becomes the weighted
  average of stock on hand and the delivery, and the selling price can change
  in the same step. "Add low-stock items" fills an order up to twice each item's reorder level.
- *Stock counts:* count a location or one category while still trading.
  Every counted item is compared with the stock ledger *as of the moment it
  was counted*, and the correction is dated then. Sales before and after the
  count are therefore both accounted for. Several people can count at once
  (scan +1 or type a total). Optionally, items not counted are set to zero.
  If a till hasn't synced since counting began, you get a warning.
- *Locations and transfers:* shop floor, store room, warehouse. Goods leave
  the source when a transfer is sent and stay in transit until someone at the
  destination receives them; shortfalls are recorded, and cancelling returns
  the stock. "Arrives now" does it in one step for rooms in the same building.
- *Stock levels:* on hand, reorder level, 30-day sales, value at cost and at
  retail, low/out filters, and a per-item ledger with running balance. Export to CSV.
- *Customer accounts:* any till can take a payment on account, even offline.
  The payment belongs to the shift (cash counts towards the drawer) and prints
  a receipt. The hub keeps a full statement (sales on account, refunds,
  payments, adjustments). Admins can enter opening balances or write-offs.
- *Reports:* sales and gross profit by day, hour, weekday, item, category,
  cashier, till, customer or location, net of returns. A return that isn't
  restocked keeps its cost as a loss. Also: money in by payment method, shifts
  with cash variances, and an activity log. Everything exports to CSV.
- *Money owed to suppliers:* bills (the supplier's invoice, usually from a
  delivery) with due dates from the supplier's payment terms. Payments go to
  the bills you tick, then the oldest; overpayments stay as credit for the next
  bill. Supplier statement and an ageing report (not due, 1–30 … over 90 days).
- *A4 documents:* purchase orders, transfer notes, supplier statements and
  payment advice open in the browser to print or save as PDF.
- *Catalogue import/export:* CSV with flexible headers (name/title, price,
  cost, barcode/ISBN, category, any extra column becomes a product detail). It
  previews (dry run) before applying; rows with problems are listed and skipped.
- *Approval rules:* each manager-approval requirement (discount, price
  change, refund, paid out, no sale) can be switched off in Settings.

**Money and quantities** are integers: kobo (minor units) and milli-units
(1.250 kg = 1250).

**Receipt numbers** are `<till prefix>-<sequence>` (e.g. `T02-000451`), unique
even when tills are offline.

## Development

Requirements: Go 1.24+, Node 20+, Wails CLI v2.17+.

```sh
wails dev                         # run the app with hot reload
go test -race ./internal/...      # unit + end-to-end tests (5 tills, offline, reconnect)
wails build                       # desktop app for this OS
wails build -platform windows/amd64
./scripts/build-windows.sh        # Windows installer (needs NSIS: brew install makensis)
go build -o zebihub ./cmd/zebihub # headless hub
```

Run several tills on one machine by giving each its own data folder:
`ZEBIVENTOR_DATA=/tmp/till2 wails dev`.

Logs are in `<data folder>/logs` (rotated at 5 MB). Settings → Connection →
**Save support file** zips the logs and settings (no sales or customer data).

Receipt previews can be written to disk for inspection:
`RECEIPT_PREVIEW_DIR=/tmp go test ./internal/printing -run Previews`.

## Layout

```
app.go, main.go        Wails desktop app (bindings for the React UI)
cmd/zebihub            headless hub CLI + OS service
internal/store         SQLite with queued, group-committed writer
internal/hub           hub: schema, pairing, auth, sync, catalogue, backups
internal/terminal      till: local DB, outbox, sync client, sales
internal/printing      receipt layout, raster rendering, ESC/POS, printer transports
internal/documents     A4 documents (purchase order, statements…) as printable HTML
internal/discovery     mDNS advertise/browse
internal/protocol      wire types shared by hub and tills
frontend/              React + TypeScript + Tailwind v4 + shadcn/ui
```

Fonts: DejaVu Sans Mono (see `internal/printing/fonts/LICENSE-DejaVu.txt`).
