# Roadmap

Progress of the rebuild into an offline-first, multi-till POS. Every piece of
work that should be done is listed under a phase here and ticked off when it
lands. Things only the shop owner can do (hardware, certificates) are listed
under **Needs you**.

Status: **phase 6 done** · all planned phases complete and on `main`; remaining items need the owner (below)

## Phase 1 — Hub, tills, sync, printing ✅ `914cc8d`

- [x] Go + Wails desktop app, React + shadcn + Tailwind UI
- [x] SQLite in WAL mode with a single queued, group-committed writer
- [x] Hub runs inside a till or headless (`zebihub`, as a Windows service / systemd unit)
- [x] Pairing, LAN discovery (mDNS), changelog sync, idempotent outbox
- [x] Tills sell offline and catch up when the hub returns
- [x] Generic catalogue: variants, barcodes, units, fractional quantities, attributes
- [x] 80 mm / 58 mm ESC/POS receipts with height that follows the content

## Phase 2 — POS operations ✅ `777abe5`

- [x] Shifts with float, paid in/out, X/Z reports
- [x] Returns and refunds against a receipt, restock or write off
- [x] Manager approvals (discount, price change, refund, paid out, no sale)
- [x] Inclusive / exclusive tax
- [x] Customers and credit sales
- [x] Hold and recall sales

## Phase 3 — Back office ✅ `4f6fee8`

- [x] Suppliers, purchase orders, goods received, weighted-average cost
- [x] Stock counts while trading, locations, transfers, stock ledger
- [x] Payments on account (offline at the till), customer statements
- [x] Sales & profit, payments, shifts, activity log; CSV export
- [x] CSV catalogue import / export
- [x] Approval rules switchable in Settings

## Phase 3.5 — Follow-ups from phase 3 ✅ branch `rebuild/phase-3.5`

- [x] Two-step transfers: sent → in transit → received, shortfalls recorded, cancel returns stock
- [x] Money owed to suppliers: bills (from deliveries or entered), payment terms, payments allocated to bills, credit carried forward, supplier statement, ageing report
- [x] A4 documents (print or save as PDF): purchase order, transfer note, supplier statement, payment advice
- [x] Hardware-test aids: log file with rotation (app and hub), every print logged, support file (logs + settings, no sales data), printer test page with width ruler and character check, cash drawer test
- [x] Windows installer (`scripts/build-windows.sh`): app + headless hub, firewall rules for the LAN, hub service restarted on upgrade, store data kept on uninstall
- [x] Fix: opening a single delivery failed (ambiguous column in the query)

## Phase 4 — Setup, safety, updates ✅ `aa5aad4`

- [x] Shop templates per business type (bookshop, grocery, pharmacy, distributor, fashion, hardware, general): categories, units, product details, payment methods, tax defaults; editable product details and units
- [x] Daily backups copied to a second drive, USB or network folder, with retention and a warning when backups stop or fail
- [x] Restore from a backup in the app or `zebihub restore` (checks the file, keeps current data first); tills detect it and re-send what the backup missed
- [x] Tills also catch up if the hub's data is put back by hand from an older copy
- [x] Auto-update: the hub keeps the latest signed version (from an update address or a file on USB) and tills update from it over the LAN; `cmd/zebirelease` signs releases
- [x] Version check between hub and tills, with a clear message saying which one to update; till versions shown in Tills
- [x] Email documents from the hub's mail server (SMTP settings, test email)
- [x] Branches: tills receive transfers and see what's on its way while offline (queued like sales); branch set-up over a private network documented
- [x] Fix: long polls now end when the hub closes, so tills reconnect at once after a hub restart

## Phase 5 — Hardening ✅ `c10e6ff`

- [x] Encrypted LAN sync: the hub serves TLS with its own certificate (kept in its database, so restores keep it); tills pin it at pairing and refuse any other; security code shown on both screens when pairing; tills paired earlier upgrade themselves; admins can refuse unencrypted connections once every till is updated (Tills)
- [x] Returns without a receipt: manager approval always, refund up to today's price, money back or store credit
- [x] Store credit: kept on the customer's account and spent at the till (works offline)
- [x] Load test: 50,000 products, 1 million sales, 5 tills (`docs/LOADTEST.md`); daily sales summaries make year-long reports ~1000× faster, stock report 40× faster
- [x] Accessibility and keyboard pass: every icon button named, clickable rows reachable and opened with Enter, labels tied to their fields, visible focus, pressed states, reduced motion; sale, payment, returns and receipts work keyboard-only
- [x] PDF attachments for emailed documents
- [x] Tills retire products the hub no longer has after its data is put back from an older copy
- [x] Branch work offline: send transfers and receive purchase-order deliveries at a till without the hub

## Phase 6 — Hand-over ✅

- [x] Staff guide inside the app (Help): how-to topics for selling, returns, shifts, stock, purchasing, settings, backups, updates and troubleshooting, shown by role and searchable
- [x] Rebuild merged into `main` and pushed

## Needs you

- [ ] **Test on a real Windows till with the thermal printer.** Build the installer
  (`./scripts/build-windows.sh`), install it, then in Settings → Printer print the
  test page (the ruler must fill exactly one line) and try **Open drawer**. Make
  a sale and print its receipt. If anything is wrong, Settings → Connection →
  **Save support file** and send it over.

## Later (parked by the owner)

- [ ] **Release signing key:** run `go run ./cmd/zebirelease keygen` once, keep
  `release.key` safe and offline, and build releases with `ZEBI_UPDATE_KEY` set to
  the printed public key (`./scripts/release.sh`). Decide where releases are
  published (the update address hubs check), e.g. GitHub Releases.
- [ ] **Branches (if you have them):** install Tailscale or ZeroTier on the hub PC
  and branch PCs, then pair branch tills using the hub's private address.
- [ ] **Code-signing certificate** (optional, recommended): without one, Windows
  SmartScreen warns when the installer is first run. Signing needs a certificate
  bought in the business's name.
