# TMIT Bookstore demo data

A full store for testing: about four years of trading (2 January 2023 to the
day before it was generated) at **TMIT Bookstore**, Ikeja, Lagos.

## What's in it

- **Catalogue:** 935 real books (titles, authors, ISBNs and publishers from
  Open Library) in ten categories, plus 36 stationery lines. Covers are the real
  ones where Open Library has them; the rest have a drawn cover.
- **Three sales points:** *Front Counter* and *Children's Corner* at TMIT Main
  Store, and *Campus Kiosk* at a second location.
- **Trading:** a shift per till per day, with busier Saturdays, lunchtimes,
  January and September (back to school) and December. Prices rise with
  inflation (January 2024, July 2024, January 2025, January 2026), and early
  2023 shows the naira-note shortage (few cash sales).
- **Customers:** walk-in regulars, plus twelve schools buying book lists on
  account and paying monthly.
- **Day-to-day operations:** returns (some without a receipt, as store credit),
  discounts approved by managers (book weeks in April and November), and diesel
  for the generator and other paid-outs.
- **Shifts:** closed with the cash count, and the occasional shortage.
- **Back office:**
  - purchase orders twice a week, deliveries (some short), supplier bills and
    Friday payments
  - kiosk top-ups received at the kiosk till
  - quarterly stock counts with small shrinkage
- **Staff:** 11 people over the years. Some joined or left part-way.

All of it went through the app's own till and hub code, so the stock ledger,
costs, shift reports, customer and supplier balances, and report summaries
agree with each other.

## Signing in

| Who | Username | Role | Password or PIN |
|---|---|---|---|
| Tolu Adebayo | `admin` | Admin | `tmit-demo-2023` |
| Chiamaka Okafor | `chiamaka` | Manager | `tmit-demo-2023` |
| Ibrahim Musa | `ibrahim` | Manager (from June 2024) | `tmit-demo-2023` |
| Femi Ogunleye | `femi` | Inventory | `tmit-demo-2023` |
| Cashiers (blessing, sade, ngozi, emeka, halima…) | | Cashier | PIN `2468` |

These are demo sign-ins for test data only.

## Opening it on a PC

1. Install Zebiventor and start it.
2. Choose **Restore a store from a data file** and pick `TMIT Bookstore.zebi.db`.
3. Sign in as `admin`.

On a PC that already runs a store, use **Settings → Backups → Restore from a
file** instead. That replaces the store on that PC (it saves a copy first).

To pass your own store's data to someone, use **Settings → Backups → Export
store data**.

## Making it again

```sh
go run ./cmd/zebidemo covers                     # once: download covers (cached)
go run ./cmd/zebidemo generate -scale 1.5 -out "demo-data/TMIT Bookstore.zebi.db"
```

`-from` and `-to` choose the days, `-scale` the volume, and `-seed` the
randomness (the same seed gives the same store). `zebidemo catalogue`
refreshes the book list from Open Library.
