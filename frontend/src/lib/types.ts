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
  hub_version: string
  update_version: string
  version_error: string
  resent: number
  encrypted: boolean
  security_code: string
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
  image_hash?: string
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
  image_hash?: string
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
  encrypted: boolean
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
  tls_fingerprint: string
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
  refunds: { method_id: string; method_name: string; kind: string; amount_minor: number }[] | null
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
  terms_days: number
  open_orders: number
  received_minor: number
  balance_minor: number
  overdue_minor: number
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
  bill_id: string
  bill_no: string
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
  unit: string
  qty_milli: number
  received_qty_milli: number
  cost_minor: number
}

export type TransferStatus = "in_transit" | "received" | "cancelled"

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
  status: TransferStatus
  received_by: string
  received_at: number
  receive_note: string
  cost_minor: number
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

export type SalesGroup = "day" | "hour" | "weekday" | "month" | "weekhour" | "product" | "category" | "cashier" | "till" | "customer" | "location"

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

// ---------- money owed to suppliers ----------

export type BillStatus = "open" | "paid" | "void"

export interface SupplierBill {
  id: string
  bill_no: string
  supplier_id: string
  supplier_name: string
  supplier_ref: string
  grn_id: string
  grn_no: string
  bill_date: number
  due_date: number
  amount_minor: number
  paid_minor: number
  status: BillStatus
  note: string
  created_by: string
  created_at: number
}

export interface SupplierBillInput {
  supplier_id: string
  supplier_ref: string
  grn_id: string
  bill_date: number
  due_date: number
  amount_minor: number
  note: string
}

export interface SupplierAllocation {
  bill_id: string
  bill_no: string
  supplier_ref: string
  amount_minor: number
}

export interface SupplierPayment {
  id: string
  payment_no: string
  supplier_id: string
  supplier_name: string
  method: string
  reference: string
  amount_minor: number
  paid_at: number
  note: string
  created_by: string
  created_at: number
  allocations?: SupplierAllocation[]
}

export interface SupplierPaymentInput {
  supplier_id: string
  method: string
  reference: string
  amount_minor: number
  paid_at: number
  note: string
  bill_ids: string[]
}

export interface SupplierLedgerEntry {
  id: string
  at: number
  kind: "opening" | "bill" | "payment" | "void" | "adjustment"
  ref_id: string
  ref_no: string
  amount_minor: number
  balance_minor: number
  note: string
  user_name: string
}

export interface SupplierStatement {
  supplier: Supplier
  entries: SupplierLedgerEntry[]
  open_bills: SupplierBill[]
}

export interface PayablesRow {
  supplier_id: string
  supplier_name: string
  not_due_minor: number
  days_1_30_minor: number
  days_31_60_minor: number
  days_61_90_minor: number
  over_90_minor: number
  total_minor: number
  credit_minor: number
}

export interface PayablesReport {
  as_of: number
  rows: PayablesRow[]
  totals: PayablesRow
}

export type DocumentKind = "purchase_order" | "transfer" | "supplier_statement" | "payment_advice"

export interface Diagnostics {
  version: string
  os: string
  mode: string
  data_dir: string
  log_file: string
  terminal: string
  hub_url: string
  hub_running: boolean
  hub_error: string
  sync: SyncStatus
  printer: PrinterConfig
  printers: string[] | null
  printers_error: string
  database_bytes: Record<string, number>
  time: string
}

// ---------- phase 4: shop types, backups, updates, email ----------

export interface ProductField {
  key: string
  label: string
  placeholder?: string
}

export interface ShopTemplate {
  type: string
  label: string
  description: string
  categories: string[]
  units: string[]
  fields: ProductField[]
  payment_methods: PaymentMethod[]
  tax_mode: string
  tax_name: string
  default_tax_bp: number
}

export interface TemplateResult {
  categories: string[] | null
  payment_methods: string[] | null
}

export interface BackupFile {
  name: string
  path: string
  size: number
  at: number
  where: "local" | "copy"
}

export interface BackupStatus {
  last_at: number
  last_error: string
  folder: string
  copy_folder: string
  copy_last_at: number
  copy_error: string
  keep: number
  warning: string
  files: BackupFile[] | null
}

export interface BackupInfo {
  path: string
  store_name: string
  hub_id: string
  same_store: boolean
  taken_at: number
  products: number
  sales: number
  last_sale_at: number
  schema_version: number
  problem: string
}

export interface UpdateManifest {
  version: string
  platform: string
  file: string
  sha256: string
  size: number
  notes?: string
  released_at?: number
}

export interface UpdatePackage {
  manifest: UpdateManifest
  signed: boolean
  stored_at: number
  source: "feed" | "upload"
}

export interface HubUpdateStatus {
  hub_version: string
  packages: UpdatePackage[] | null
  feed_url: string
  default_feed: string
  last_check_at: number
  last_check_error: string
  signing_key: boolean
}

export interface UpdateState {
  current: string
  available: string
  notes: string
  signed: boolean
  downloading: boolean
  done: number
  total: number
  ready: string
  error: string
  can_install: boolean
}

export interface EmailSettings {
  host: string
  port: number
  security: "starttls" | "tls" | "none"
  username: string
  password?: string
  password_set: boolean
  from: string
  from_name: string
}

export interface EmailDraft {
  to: string
  cc: string
  subject: string
  message: string
  ready: boolean
}

export interface IncomingTransfer extends StockTransfer {
  received_here: boolean
}

export const STORE_CREDIT = "store_credit"

export interface NoReceiptInput {
  lines: { variant_id: string; qty_milli: number; unit_refund_minor: number; restock: boolean }[]
  customer_id: string
  refunds: { method_id: string; amount_minor: number; reference: string }[]
  reason: string
  approval_token: string
}

export interface ExpectedDelivery {
  order: PurchaseOrder
  received_here: boolean
}

export interface TillLocation {
  id: string
  name: string
}
