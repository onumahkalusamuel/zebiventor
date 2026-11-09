// Back-office API at the hub (needs a connection and a hub session). Calls
// go through the till's authenticated proxy (App.HubCall).
import * as App from "../../wailsjs/go/main/App"
import type * as T from "./types"

/* eslint-disable @typescript-eslint/no-explicit-any */
const go = App as any

async function call<R>(method: "GET" | "POST" | "PUT", path: string, body?: unknown): Promise<R> {
  const out: string = await go.HubCall(method, path, body === undefined ? "" : JSON.stringify(body))
  return (out ? JSON.parse(out) : null) as R
}

function qs(params: Record<string, string | number | undefined>): string {
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") p.set(k, String(v))
  const s = p.toString()
  return s ? "?" + s : ""
}

type Id = { id: string }

export const hub = {
  suppliers: () => call<T.Supplier[]>("GET", "/api/admin/suppliers"),
  saveSupplier: (s: Partial<T.Supplier>) => call<Id>("POST", "/api/admin/suppliers", s),

  purchaseOrders: (status = "", supplier = "") => call<T.PurchaseOrder[]>("GET", "/api/admin/purchase-orders" + qs({ status, supplier })),
  purchaseOrder: (id: string) => call<T.PurchaseOrder>("GET", "/api/admin/purchase-orders/" + encodeURIComponent(id)),
  savePurchaseOrder: (o: T.PurchaseOrderInput) => call<Id>("POST", "/api/admin/purchase-orders", o),
  setPurchaseOrderStatus: (id: string, status: T.POStatus) => call<void>("POST", `/api/admin/purchase-orders/${id}/status`, { status }),

  receipts: (supplier = "") => call<T.GoodsReceipt[]>("GET", "/api/admin/receipts" + qs({ supplier })),
  receipt: (id: string) => call<T.GoodsReceipt>("GET", "/api/admin/receipts/" + encodeURIComponent(id)),
  receiveGoods: (r: T.GoodsReceiptInput) => call<Id>("POST", "/api/admin/receipts", r),

  bills: (supplier = "", status = "") => call<T.SupplierBill[]>("GET", "/api/admin/bills" + qs({ supplier, status })),
  bill: (id: string) =>
    call<{ bill: T.SupplierBill; payments: T.SupplierPayment[] }>("GET", "/api/admin/bills/" + encodeURIComponent(id)),
  createBill: (b: T.SupplierBillInput) => call<Id>("POST", "/api/admin/bills", b),
  voidBill: (id: string, reason: string) => call<void>("POST", `/api/admin/bills/${id}/void`, { reason }),
  supplierPayments: (supplier = "") => call<T.SupplierPayment[]>("GET", "/api/admin/supplier-payments" + qs({ supplier })),
  supplierPayment: (id: string) => call<T.SupplierPayment>("GET", "/api/admin/supplier-payments/" + encodeURIComponent(id)),
  paySupplier: (p: T.SupplierPaymentInput) => call<Id>("POST", "/api/admin/supplier-payments", p),
  supplierStatement: (id: string) => call<T.SupplierStatement>("GET", `/api/admin/suppliers/${id}/statement`),
  payables: () => call<T.PayablesReport>("GET", "/api/admin/reports/payables"),

  locations: () => call<T.LocationInfo[]>("GET", "/api/admin/locations"),
  saveLocation: (l: Partial<T.Location>) => call<Id>("POST", "/api/admin/locations", l),

  transfers: (status = "") => call<T.StockTransfer[]>("GET", "/api/admin/transfers" + qs({ status })),
  transfer: (id: string) => call<T.StockTransfer>("GET", "/api/admin/transfers/" + encodeURIComponent(id)),
  createTransfer: (t: {
    from_location_id: string
    to_location_id: string
    note: string
    receive_now: boolean
    lines: { variant_id: string; qty_milli: number }[]
  }) => call<Id>("POST", "/api/admin/transfers", t),
  receiveTransfer: (id: string, r: { note: string; lines: { variant_id: string; received_qty_milli: number }[] }) =>
    call<void>("POST", `/api/admin/transfers/${id}/receive`, r),
  cancelTransfer: (id: string) => call<void>("POST", `/api/admin/transfers/${id}/cancel`, {}),

  counts: () => call<T.StockCount[]>("GET", "/api/admin/counts"),
  count: (id: string) => call<T.StockCount>("GET", "/api/admin/counts/" + encodeURIComponent(id)),
  createCount: (c: { name: string; location_id: string; category_id: string }) => call<Id>("POST", "/api/admin/counts", c),
  countEntry: (id: string, e: { variant_id: string; qty_milli: number; mode: "add" | "set" | "remove" }) =>
    call<T.StockCountLine>("POST", `/api/admin/counts/${id}/entries`, e),
  postCount: (id: string, p: { zero_uncounted: boolean; note: string }) => call<void>("POST", `/api/admin/counts/${id}/post`, p),
  cancelCount: (id: string) => call<void>("POST", `/api/admin/counts/${id}/cancel`, {}),

  movements: (variant: string, location = "") => call<T.StockMovement[]>("GET", "/api/admin/stock/movements" + qs({ variant, location })),
  stock: (f: { location?: string; category?: string; q?: string; only?: string; limit?: number }) =>
    call<T.StockReport>("GET", "/api/admin/reports/stock" + qs(f)),

  statement: (customerId: string) => call<T.CustomerStatement>("GET", `/api/admin/customers/${customerId}/statement`),
  adjustBalance: (customerId: string, amount_minor: number, note: string) =>
    call<void>("POST", `/api/admin/customers/${customerId}/adjust`, { amount_minor, note }),

  salesReport: (from: number, to: number, group: T.SalesGroup) => call<T.SalesReport>("GET", "/api/admin/reports/sales" + qs({ from, to, group })),
  shiftReport: (from: number, to: number) => call<T.ShiftReportRow[]>("GET", "/api/admin/reports/shifts" + qs({ from, to })),
  audit: (from: number, to: number, action = "") => call<T.AuditEntry[]>("GET", "/api/admin/audit" + qs({ from, to, action })),

  importProducts: (csv: string, dry_run: boolean, location_id = "") => call<T.ImportResult>("POST", "/api/admin/import/products", { csv, dry_run, location_id }),
  exportProducts: (location = "") => call<{ csv: string }>("GET", "/api/admin/export/products" + qs({ location })),

  templates: () => call<T.ShopTemplate[]>("GET", "/api/admin/templates"),
  applyTemplate: (type: string, with_tax: boolean) => call<T.TemplateResult>("POST", "/api/admin/templates/apply", { type, with_tax }),

  backups: () => call<T.BackupStatus>("GET", "/api/admin/backups"),
  backupNow: () => call<T.BackupStatus>("POST", "/api/admin/backups", {}),
  saveBackupSettings: (copy_folder: string, keep: number) => call<T.BackupStatus>("PUT", "/api/admin/backups/settings", { copy_folder, keep }),

  updateStatus: () => call<T.HubUpdateStatus>("GET", "/api/admin/update"),
  checkForUpdates: () => call<T.HubUpdateStatus>("POST", "/api/admin/update/check", {}),
  saveUpdateFeed: (feed_url: string) => call<T.HubUpdateStatus>("PUT", "/api/admin/update/settings", { feed_url }),

  emailSettings: () => call<T.EmailSettings>("GET", "/api/admin/email"),
  saveEmailSettings: (s: T.EmailSettings) => call<void>("PUT", "/api/admin/email", s),
  sendTestEmail: (to: string) =>
    call<void>("POST", "/api/admin/email/send", { to: [to], subject: "Test email from Zebiventor", text: "If you can read this, email is set up correctly.", kind: "test" }),
}

/** Builds CSV text from rows (quotes as needed). */
export function toCSV(header: string[], rows: (string | number)[][]): string {
  const esc = (v: string | number) => {
    const s = String(v ?? "")
    return /[",\n\r]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s
  }
  return [header, ...rows].map((r) => r.map(esc).join(",")).join("\r\n") + "\r\n"
}

/** Money in major units for CSV (no symbol, no grouping). */
export function csvMoney(minor: number, digits = 2): string {
  return (minor / 10 ** digits).toFixed(digits)
}
