// JSON shapes returned by the Go backend (see internal/protocol, internal/terminal, app.go).

export type Role = "admin" | "manager" | "inventory" | "cashier"

export interface User {
  id: string
  name: string
  username: string
  role: Role
}

export interface Currency {
  code: string
  symbol: string
  digits: number
}

export interface SyncStatus {
  paired: boolean
  online: boolean
  revoked: boolean
  hub_url: string
  last_sync_at: number
  last_error: string
  pending: number
  rejected: number
  rev: number
}

export interface Pairing {
  hub_url: string
  hub_id: string
  store_name: string
  terminal_id: string
  terminal_name: string
  receipt_prefix: string
  location_id: string
}

export interface AppState {
  version: string
  data_dir: string
  mode: "" | "host" | "till"
  paired: boolean
  pairing: Pairing
  user?: User | null
  status: SyncStatus
  settings: Record<string, string>
  currency: Currency
  hub_running: boolean
  hub_error: string
  hub_port: number
  hub_addresses: string[] | null
  has_session: boolean
  error: string
}

export interface Item {
  variant_id: string
  product_id: string
  name: string
  variant_name: string
  sku: string
  price_minor: number
  unit: string
  allow_fraction: boolean
  track_stock: boolean
  stock_milli: number
  category_id: string
  attributes: Record<string, string> | null
  pack_qty_milli: number
  tax_rate_bp: number
}

export interface PaymentMethod {
  id: string
  name: string
  kind: "cash" | "card" | "transfer" | "mobile_money" | "credit" | "voucher" | "other"
  requires_reference: boolean
  opens_drawer: boolean
  sort_order: number
  active: boolean
}

export interface SaleLine {
  id: string
  original_price_minor: number
  tax_rate_bp: number
  track_stock: boolean
  line_no: number
  variant_id: string
  product_name: string
  variant_name: string
  sku: string
  qty_milli: number
  unit_price_minor: number
  discount_minor: number
  tax_minor: number
  total_minor: number
}

export interface SalePayment {
  id: string
  method_id: string
  method_name: string
  kind: string
  amount_minor: number
  reference: string
}

export interface Sale {
  id: string
  receipt_no: string
  user_name: string
  status: string
  subtotal_minor: number
  discount_minor: number
  tax_minor: number
  total_minor: number
  paid_minor: number
  change_minor: number
  created_at: number
  synced_at: number
  customer_id: string
  customer_name: string
  tax_mode: string
  approved_by: string
  lines: SaleLine[]
  payments: SalePayment[]
}

export interface SaleSummary {
  id: string
  kind: "sale" | "return" | "payment"
  customer: string
  receipt_no: string
  user_name: string
  total_minor: number
  items: number
  methods: string
  created_at: number
  synced: boolean
}


export interface SaleResult {
  sale: Sale
  printed: boolean
  print_error: string
}

export interface Category {
  id: string
  parent_id: string
  name: string
  active: boolean
}

export interface Product {
  id: string
  name: string
  category_id: string
  description: string
  attributes: Record<string, string>
  unit: string
  allow_fraction: boolean
  track_stock: boolean
  tax_rate_bp: number
  active: boolean
  updated_at?: number
}

export interface Variant {
  id: string
  product_id: string
  name: string
  sku: string
  price_minor: number
  cost_minor: number
  active: boolean
  reorder_level_milli: number
}

export interface VariantDetail extends Variant {
  barcodes: string[]
  stock_milli: number
}

export interface ProductDetail extends Product {
  category_name: string
  variants: VariantDetail[]
}

export interface VariantInput extends Variant {
  barcodes: string[]
  opening_qty_milli: number
  opening_location_id: string
}

export interface ProductInput {
  product: Product
  variants: VariantInput[]
}

export interface StaffUser {
  id: string
  name: string
  username: string
  role: Role
  active: boolean
  has_pin: boolean
}

export interface UserInput {
  id: string
  name: string
  username: string
  role: Role
  password: string
  pin: string
  active: boolean
}

export interface TerminalInfo {
  id: string
  name: string
  location_id: string
  location_name: string
  receipt_prefix: string
  active: boolean
  paired_at: number
  last_seen_at: number
  last_ip: string
  app_version: string
}

export interface PairingCode {
  code: string
  expires_at: number
}

export interface DiscoveredHub {
  hub_id: string
  store_name: string
  url: string
  host: string
}

export interface HubInfo {
  hub_id: string
  store_name: string
  version: string
  setup_complete: boolean
}

export type TransportKind = "none" | "system" | "network" | "device"

export interface PrinterConfig {
  target: { kind: TransportKind; address: string }
  paper: "80mm" | "58mm"
  mode: "raster" | "text"
  auto_print: boolean
  open_drawer: boolean
  feed_lines: number
  cut: boolean
}

export interface SetupHostInput {
  store_name: string
  business_type: string
  location_name: string
  address: string
  phone: string
  currency_code: string
  currency_symbol: string
  admin_name: string
  admin_username: string
  admin_password: string
  terminal_name: string
  port: number
  demo: boolean
}

export interface CartLineInput {
  variant_id: string
  qty_milli: number
  price_override?: number
}

export interface PaymentInput {
  method_id: string
  amount_minor: number
  reference: string
}

export interface SaleInput {
  lines: CartLineInput[]
  payments: PaymentInput[]
  discount_minor: number
  customer_id: string
  note: string
  approval_token: string
}

export interface StockAdjustRequest {
  variant_id: string
  location_id: string
  delta_milli?: number
  count_milli?: number
  note: string
}

export interface Customer {
  id: string
  name: string
  phone: string
  email: string
  notes: string
  active: boolean
  balance_minor: number
  updated_at: number
}

export interface MethodTotal {
  method_name: string
  kind: string
  sales_minor: number
  refunds_minor: number
  payments_minor: number
  net_minor: number
}

export interface ShiftSummary {
  sales_count: number
  items_milli: number
  gross_minor: number
  discount_minor: number
  tax_minor: number
  net_sales_minor: number
  returns_count: number
  returns_minor: number
  by_method: MethodTotal[] | null
  opening_float_minor: number
  cash_sales_minor: number
  cash_refunds_minor: number
  paid_in_minor: number
  paid_out_minor: number
  account_payments_count: number
  account_payments_minor: number
  cash_account_payments_minor: number
  expected_cash_minor: number
  counted_cash_minor: number
  variance_minor: number
  closed: boolean
}

export interface Shift {
  id: string
  status: "open" | "closed"
  opened_by_name: string
  opened_at: number
  opening_float_minor: number
  closed_by_name: string
  closed_at: number
  counted_cash_minor: number
  note: string
  summary?: ShiftSummary
}

export interface CashMovement {
  id: string
  kind: "paid_in" | "paid_out"
  amount_minor: number
  reason: string
  user_name: string
  created_at: number
}

export interface ReturnableLine extends SaleLine {
  returned_milli: number
  returnable_milli: number
  paid_minor: number
}

export interface ReturnableSale {
  sale: Sale
  lines: ReturnableLine[]
  source: "local" | "hub"
  partial: boolean
}

export interface ReturnInput {
  receipt_no: string
  lines: { sale_line_id: string; qty_milli: number; restock: boolean }[]
  refunds: PaymentInput[]
  reason: string
  approval_token: string
}

export interface Return {
  id: string
  return_no: string
  sale_receipt_no: string
  total_minor: number
  tax_minor: number
  customer_name: string
}

export interface ReturnResult {
  return: Return
  printed: boolean
  print_error: string
}

export interface HeldSale {
  id: string
  label: string
  user_name: string
  item_count: number
  total_minor: number
  payload: string
  created_at: number
}

export interface ApprovalResult {
  token: string
  name: string
}

// ---------- payments on account ----------

export interface AccountPaymentInput {
  customer_id: string
  method_id: string
  amount_minor: number
  reference: string
  note: string
}

export interface AccountPayment {
  id: string
  payment_no: string
  customer_id: string
  customer_name: string
  method_name: string
  kind: string
  amount_minor: number
  reference: string
  balance_before_minor: number
  created_at: number
}

export interface AccountPaymentResult {
  payment: AccountPayment
  printed: boolean
  print_error: string
}

export interface CustomerLedgerEntry {
  id: string
  at: number
  kind: "opening" | "sale" | "return" | "payment" | "adjustment"
  ref_id: string
  ref_no: string
  amount_minor: number
  balance_minor: number
  note: string
  user_name: string
}

export interface CustomerStatement {
  customer: Customer
  entries: CustomerLedgerEntry[]
}

// ---------- back office (hub) ----------

export interface Location {
  id: string
  name: string
  code: string
  active: boolean
}

export interface LocationInfo extends Location {
  terminals: number
  value_minor: number
  is_default: boolean
}

export interface Supplier {
  id: string
  name: string
  contact_name: string
  phone: string
  email: string
  address: string
  notes: string
  active: boolean
  open_orders: number
  received_minor: number
}

export type POStatus = "draft" | "ordered" | "partial" | "received" | "cancelled"

export interface PurchaseOrderLine {
  id: string
  variant_id: string
  product_name: string
  variant_name: string
  sku: string
  unit: string
  qty_ordered_milli: number
  qty_received_milli: number
  unit_cost_minor: number
}

export interface PurchaseOrder {
  id: string
  po_no: string
  supplier_id: string
  supplier_name: string
  location_id: string
  location_name: string
  status: POStatus
  expected_at: number
  note: string
  created_by: string
  created_at: number
  updated_at: number
  ordered_at: number
  total_minor: number
  line_count: number
  lines?: PurchaseOrderLine[]
  receipts?: GoodsReceipt[]
}

export interface PurchaseOrderInput {
  id: string
  supplier_id: string
  location_id: string
  expected_at: number
  note: string
  lines: { variant_id: string; qty_ordered_milli: number; unit_cost_minor: number }[]
  submit: boolean
}

export interface GoodsReceiptLine {
  id?: string
  po_line_id: string
  variant_id: string
  product_name: string
  variant_name: string
  sku: string
  qty_milli: number
  unit_cost_minor: number
  total_minor: number
  new_price_minor?: number
}

export interface GoodsReceipt {
  id: string
  grn_no: string
  po_id: string
  po_no: string
  supplier_id: string
  supplier_name: string
  location_id: string
  location_name: string
  supplier_ref: string
  note: string
  received_by: string
  created_at: number
  total_minor: number
  lines?: GoodsReceiptLine[]
}

export interface GoodsReceiptInput {
  po_id: string
  supplier_id: string
  location_id: string
  supplier_ref: string
  note: string
  lines: { po_line_id: string; variant_id: string; qty_milli: number; unit_cost_minor: number; new_price_minor: number }[]
  close_po: boolean
}

export interface StockTransferLine {
  variant_id: string
  product_name: string
  variant_name: string
  sku: string
  qty_milli: number
}

export interface StockTransfer {
  id: string
  transfer_no: string
  from_location_id: string
  from_location_name: string
  to_location_id: string
  to_location_name: string
  note: string
  created_by: string
  created_at: number
  items_milli: number
  lines?: StockTransferLine[]
}

export type CountStatus = "open" | "posted" | "cancelled"

export interface StockCountLine {
  variant_id: string
  product_name: string
  variant_name: string
  sku: string
  unit: string
  counted_milli: number
  counted_at: number
  counted_by: string
  expected_milli: number
  cost_minor: number
  uncounted: boolean
}

export interface StockCount {
  id: string
  count_no: string
  name: string
  location_id: string
  location_name: string
  category_id: string
  category_name: string
  status: CountStatus
  created_by: string
  created_at: number
  posted_by: string
  posted_at: number
  note: string
  line_count: number
  lines?: StockCountLine[]
  warnings?: string[]
}

export interface StockMovement {
  id: string
  location_name: string
  qty_milli: number
  reason: string
  ref_type: string
  ref_no: string
  user_name: string
  terminal_name: string
  note: string
  created_at: number
  balance_milli: number
}

export interface StockReportRow {
  variant_id: string
  product_name: string
  variant_name: string
  sku: string
  category_name: string
  unit: string
  location_id: string
  location_name: string
  qty_milli: number
  reorder_level_milli: number
  cost_minor: number
  price_minor: number
  value_minor: number
  retail_minor: number
  sold_milli_30d: number
}

export interface StockReport {
  rows: StockReportRow[]
  value_minor: number
  retail_minor: number
  low_count: number
  out_count: number
}

export type SalesGroup = "day" | "hour" | "weekday" | "product" | "category" | "cashier" | "till" | "customer" | "location"

export interface SalesReportRow {
  key: string
  label: string
  sub: string
  transactions: number
  qty_milli: number
  gross_minor: number
  discount_minor: number
  tax_minor: number
  returns_minor: number
  net_minor: number
  cost_minor: number
  profit_minor: number
}

export interface SalesReport {
  from: number
  to: number
  group: SalesGroup
  totals: SalesReportRow
  rows: SalesReportRow[]
  payments: MethodTotal[]
}

export interface ShiftReportRow extends Shift {
  terminal_name: string
  expected_cash_minor: number
  variance_minor: number
}

export interface AuditEntry {
  id: string
  at: number
  user_name: string
  terminal_name: string
  action: string
  entity: string
  entity_id: string
  details: string
}

export interface ImportResult {
  dry_run: boolean
  rows: number
  created: number
  updated: number
  skipped: number
  errors: { row: number; message: string }[]
  columns: string[]
  new_categories: string[]
}
