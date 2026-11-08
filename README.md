# Zebiventor

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
go build -o zebihub ./cmd/zebihub # headless hub
```

Run several tills on one machine by giving each its own data folder:
`ZEBIVENTOR_DATA=/tmp/till2 wails dev`.

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
internal/discovery     mDNS advertise/browse
internal/protocol      wire types shared by hub and tills
frontend/              React + TypeScript + Tailwind v4 + shadcn/ui
```

Fonts: DejaVu Sans Mono (see `internal/printing/fonts/LICENSE-DejaVu.txt`).
