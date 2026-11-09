// Typed façade over the generated Wails bindings. The generated model
// classes carry helper methods, so plain objects are passed through `any`.
import * as App from "../../wailsjs/go/main/App"
import { EventsOn } from "../../wailsjs/runtime/runtime"
import type * as T from "./types"

/* eslint-disable @typescript-eslint/no-explicit-any */
const go = App as any

export const api = {
  getState: (): Promise<T.AppState> => go.GetState(),

  setupHost: (input: T.SetupHostInput): Promise<void> => go.SetupHost(input),
  discoverHubs: (): Promise<T.DiscoveredHub[]> => go.DiscoverHubs(),
  checkHub: (url: string): Promise<T.HubInfo> => go.CheckHub(url),
  joinHub: (url: string, code: string, name: string): Promise<void> => go.JoinHub(url, code, name),
  setHubAddress: (url: string): Promise<void> => go.SetHubAddress(url),
  syncNow: (): Promise<void> => go.SyncNow(),
  retryRejected: (): Promise<void> => go.RetryRejected(),

  login: (username: string, secret: string): Promise<T.User> => go.Login(username, secret),
  logout: (): Promise<void> => go.Logout(),

  searchItems: (q: string): Promise<T.Item[]> => go.SearchItems(q),
  lookupCode: (code: string): Promise<T.Item | null> => go.LookupCode(code),
  paymentMethods: (): Promise<T.PaymentMethod[]> => go.PaymentMethods(),
  completeSale: (input: T.SaleInput): Promise<T.SaleResult> => go.CompleteSale(input),
  recentSales: (q: string): Promise<T.SaleSummary[]> => go.RecentSales(q),
  getSale: (id: string): Promise<T.Sale> => go.GetSale(id),

  getPrinterConfig: (): Promise<T.PrinterConfig> => go.GetPrinterConfig(),
  savePrinterConfig: (c: T.PrinterConfig): Promise<void> => go.SavePrinterConfig(c),
  listPrinters: (): Promise<string[]> => go.ListPrinters(),
  testDrawer: (c: T.PrinterConfig): Promise<void> => go.TestDrawer(c),
  printDocument: (kind: T.DocumentKind, id: string): Promise<string> => go.PrintDocument(kind, id),
  getDiagnostics: (): Promise<T.Diagnostics> => go.GetDiagnostics(),
  saveDiagnostics: (): Promise<string> => go.SaveDiagnostics(),
  openDataFolder: (): Promise<void> => go.OpenDataFolder(),
  printSale: (id: string, reprint: boolean): Promise<void> => go.PrintSale(id, reprint),
  receiptPreview: (id: string, reprint: boolean): Promise<string> => go.ReceiptPreview(id, reprint),
  testPreview: (c: T.PrinterConfig): Promise<string> => go.TestPreview(c),
  testPrint: (c: T.PrinterConfig): Promise<void> => go.TestPrint(c),
  openDrawer: (approvalToken: string): Promise<void> => go.OpenDrawer(approvalToken),

  approve: (username: string, secret: string): Promise<T.ApprovalResult> => go.Approve(username, secret),

  lookupForReturn: (receipt: string): Promise<T.ReturnableSale> => go.LookupForReturn(receipt),
  completeReturn: (input: T.ReturnInput): Promise<T.ReturnResult> => go.CompleteReturn(input),
  returnPreview: (id: string, reprint: boolean): Promise<string> => go.ReturnPreview(id, reprint),
  printReturn: (id: string, reprint: boolean): Promise<void> => go.PrintReturn(id, reprint),

  currentShift: (): Promise<T.Shift | null> => go.CurrentShift(),
  openShift: (floatMinor: number): Promise<T.Shift> => go.OpenShift(floatMinor),
  cashMovement: (kind: "paid_in" | "paid_out", amountMinor: number, reason: string, approvalToken: string): Promise<void> =>
    go.CashMovement(kind, amountMinor, reason, approvalToken),
  shiftMovements: (shiftId: string): Promise<T.CashMovement[]> => go.ShiftMovements(shiftId),
  closeShift: (countedMinor: number, note: string): Promise<{ shift: T.Shift; print_error: string }> => go.CloseShift(countedMinor, note),
  listShifts: (): Promise<T.Shift[]> => go.ListShifts(),
  shiftPreview: (id: string): Promise<string> => go.ShiftPreview(id),
  printShift: (id: string): Promise<void> => go.PrintShift(id),

  holdSale: (h: Partial<T.HeldSale>): Promise<string> => go.HoldSale(h),
  heldSales: (): Promise<T.HeldSale[]> => go.HeldSales(),
  recallHeldSale: (id: string): Promise<T.HeldSale> => go.RecallHeldSale(id),

  receiveAccountPayment: (input: T.AccountPaymentInput): Promise<T.AccountPaymentResult> => go.ReceiveAccountPayment(input),
  accountPaymentPreview: (id: string, reprint: boolean): Promise<string> => go.AccountPaymentPreview(id, reprint),
  printAccountPayment: (id: string): Promise<void> => go.PrintAccountPayment(id),
  saveTextFile: (name: string, content: string): Promise<string> => go.SaveTextFile(name, content),

  searchCustomers: (q: string): Promise<T.Customer[]> => go.SearchCustomers(q),
  saveCustomer: (c: Partial<T.Customer>): Promise<T.Customer> => go.SaveCustomer(c),

  listProducts: (q: string): Promise<T.ProductDetail[]> => go.ListProducts(q),
  categories: (): Promise<T.Category[]> => go.Categories(),
  saveProduct: (p: T.ProductInput): Promise<string> => go.SaveProduct(p),
  saveCategory: (c: Partial<T.Category>): Promise<string> => go.SaveCategory(c),
  adjustStock: (r: T.StockAdjustRequest): Promise<void> => go.AdjustStock(r),

  createPairingCode: (name: string): Promise<T.PairingCode> => go.CreatePairingCode(name),
  listTerminals: (): Promise<T.TerminalInfo[]> => go.ListTerminals(),
  revokeTerminal: (id: string): Promise<void> => go.RevokeTerminal(id),
  listUsers: (): Promise<T.StaffUser[]> => go.ListUsers(),
  saveUser: (u: T.UserInput): Promise<string> => go.SaveUser(u),
  saveStoreSettings: (v: Record<string, string>): Promise<void> => go.SaveStoreSettings(v),
  savePaymentMethod: (m: Partial<T.PaymentMethod>): Promise<string> => go.SavePaymentMethod(m),
}

export function onSyncStatus(cb: (s: T.SyncStatus) => void): () => void {
  return EventsOn("sync:status", cb)
}

/** Wails rejects with the Go error string; normalise to a message. */
export function errorMessage(e: unknown): string {
  if (typeof e === "string") return e
  if (e instanceof Error) return e.message
  return "Something went wrong"
}
