# Roadmap

Progress of the rebuild into an offline-first, multi-till POS. Every piece of
work that should be done is listed under a phase here and ticked off when it
lands. Things only the shop owner can do (hardware, certificates) are listed
under **Needs you**.

Status: **phase 3.5 done** · next: **phase 4**

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

## Phase 4 — Setup, safety, updates

- [ ] Shop templates per business type (bookshop, grocery, pharmacy, distributor, general): categories, units, product details, payment methods, tax defaults
- [ ] Scheduled backups to a second drive, USB or network folder, with retention and a warning when backups stop
- [ ] Restore from a backup in the app (with a check before replacing data)
- [ ] Auto-update: the hub keeps the latest version and tills update from it over the LAN, so only one PC needs internet
- [ ] Version check between hub and tills, with a clear message when one needs updating
- [ ] Email documents straight from the app (SMTP settings), as well as print / PDF

## Phase 5 — Hardening

- [ ] Encrypted LAN sync (TLS with the hub's certificate pinned at pairing)
- [ ] Returns without a receipt (manager approval, refund or store credit)
- [ ] Load test: 50,000 products, 1 million sales, 5 tills syncing at once; fix anything slow
- [ ] Accessibility and keyboard-only pass over every screen

## Needs you

- [ ] **Test on a real Windows till with the thermal printer.** Build the installer
  (`./scripts/build-windows.sh`), install it, then in Settings → Printer print the
  test page (the ruler must fill exactly one line) and try **Open drawer**. Make
  a sale and print its receipt. If anything is wrong, Settings → Connection →
  **Save support file** and send it over.
- [ ] **Code-signing certificate** (optional, recommended): without one, Windows
  SmartScreen warns when the installer is first run. Signing needs a certificate
  bought in the business's name.
