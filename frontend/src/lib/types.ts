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
  lines: SaleLine[]
  payments: SalePayment[]
}

export interface SaleSummary {
  id: string
  receipt_no: string
  user_name: string
  total_minor: number
  items: number
  methods: string
  created_at: number
  synced: boolean
}

export interface DayTotals {
  count: number
  total_minor: number
  by_method: Record<string, number>
  unsynced: number
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
  note: string
}

export interface StockAdjustRequest {
  variant_id: string
  location_id: string
  delta_milli?: number
  count_milli?: number
  note: string
}
