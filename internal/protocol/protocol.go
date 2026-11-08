// Package protocol defines the records and messages exchanged between the
// hub and terminals. All timestamps are unix milliseconds, money is int64
// minor units and quantities are int64 milli-units (see package money).
package protocol

import "encoding/json"

// Version is bumped on breaking protocol changes.
const Version = 1

// Roles, from most to least privileged.
const (
	RoleAdmin     = "admin"
	RoleManager   = "manager"
	RoleInventory = "inventory"
	RoleCashier   = "cashier"
)

// Payment method kinds.
const (
	PayCash     = "cash"
	PayCard     = "card"
	PayTransfer = "transfer"
	PayMobile   = "mobile_money"
	PayCredit   = "credit"
	PayVoucher  = "voucher"
	PayOther    = "other"
)

// Entity names used in the hub changelog.
const (
	EntitySetting       = "setting"
	EntityLocation      = "location"
	EntityUser          = "user"
	EntityCategory      = "category"
	EntityProduct       = "product"
	EntityVariant       = "variant"
	EntityBarcode       = "barcode"
	EntityPaymentMethod = "payment_method"
	EntityStockLevel    = "stock_level"
)

// ---------- master data (flows hub -> terminals) ----------

type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Location struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Active bool   `json:"active"`
}

// SyncUser carries credential hashes so tills can authenticate offline.
type SyncUser struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	PasswordHash string `json:"password_hash"`
	PinHash      string `json:"pin_hash"`
	Active       bool   `json:"active"`
}

type Category struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
	Active   bool   `json:"active"`
}

// Product is the sellable concept (a book title, a drink). Shop-specific
// fields (ISBN, author, publisher, expiry policy, ...) live in Attributes so
// one schema serves bookshops, groceries and distributors alike.
type Product struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	CategoryID    string            `json:"category_id"`
	Description   string            `json:"description"`
	Attributes    map[string]string `json:"attributes"`
	Unit          string            `json:"unit"`           // pc, kg, l, box...
	AllowFraction bool              `json:"allow_fraction"` // sold by weight/volume
	TrackStock    bool              `json:"track_stock"`
	TaxRateBP     int64             `json:"tax_rate_bp"` // basis points; reserved for phase 2
	Active        bool              `json:"active"`
	UpdatedAt     int64             `json:"updated_at"`
}

// Variant is the stock-keeping unit (hardcover vs paperback, 50cl vs 1l).
// Every product has at least one.
type Variant struct {
	ID         string `json:"id"`
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	SKU        string `json:"sku"`
	PriceMinor int64  `json:"price_minor"`
	CostMinor  int64  `json:"cost_minor"`
	Active     bool   `json:"active"`
}

// Barcode maps a scanned code to a variant. PackQtyMilli lets one barcode
// represent a pack (a carton of 24 = 24000).
type Barcode struct {
	Code         string `json:"code"`
	VariantID    string `json:"variant_id"`
	PackQtyMilli int64  `json:"pack_qty_milli"`
	Deleted      bool   `json:"deleted"`
}

type PaymentMethod struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Kind              string `json:"kind"`
	RequiresReference bool   `json:"requires_reference"`
	OpensDrawer       bool   `json:"opens_drawer"`
	SortOrder         int    `json:"sort_order"`
	Active            bool   `json:"active"`
}

type StockLevel struct {
	VariantID  string `json:"variant_id"`
	LocationID string `json:"location_id"`
	QtyMilli   int64  `json:"qty_milli"`
}

// ---------- transactions (flow terminals -> hub) ----------

type Sale struct {
	ID            string        `json:"id"`
	ReceiptNo     string        `json:"receipt_no"`
	TerminalID    string        `json:"terminal_id"`
	LocationID    string        `json:"location_id"`
	UserID        string        `json:"user_id"`
	UserName      string        `json:"user_name"`
	CustomerID    string        `json:"customer_id"`
	Status        string        `json:"status"`
	SubtotalMinor int64         `json:"subtotal_minor"`
	DiscountMinor int64         `json:"discount_minor"`
	TaxMinor      int64         `json:"tax_minor"`
	TotalMinor    int64         `json:"total_minor"`
	PaidMinor     int64         `json:"paid_minor"`
	ChangeMinor   int64         `json:"change_minor"`
	Note          string        `json:"note"`
	CreatedAt     int64         `json:"created_at"`
	SyncedAt      int64         `json:"synced_at"`
	Lines         []SaleLine    `json:"lines"`
	Payments      []SalePayment `json:"payments"`
}

type SaleLine struct {
	ID             string `json:"id"`
	LineNo         int    `json:"line_no"`
	VariantID      string `json:"variant_id"`
	ProductName    string `json:"product_name"`
	VariantName    string `json:"variant_name"`
	SKU            string `json:"sku"`
	QtyMilli       int64  `json:"qty_milli"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	DiscountMinor  int64  `json:"discount_minor"`
	TaxMinor       int64  `json:"tax_minor"`
	TotalMinor     int64  `json:"total_minor"`
	TrackStock     bool   `json:"track_stock"`
}

type SalePayment struct {
	ID          string `json:"id"`
	MethodID    string `json:"method_id"`
	MethodName  string `json:"method_name"`
	Kind        string `json:"kind"`
	AmountMinor int64  `json:"amount_minor"`
	Reference   string `json:"reference"`
}

// ---------- sync messages ----------

// Op kinds pushed from terminals.
const OpSaleCreate = "sale.create"

// Op is one durable outbox entry. ID is a ULID created on the terminal and
// is the idempotency key: the hub applies each op at most once.
type Op struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt int64           `json:"created_at"`
}

type PushRequest struct {
	Ops []Op `json:"ops"`
}

const (
	OpApplied   = "applied"
	OpDuplicate = "duplicate"
	OpRejected  = "rejected"
)

type OpResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type PushResponse struct {
	Results []OpResult `json:"results"`
}

// PullResponse carries every entity whose latest revision is > since.
// Rev is the highest revision included; More means call again immediately.
type PullResponse struct {
	Rev            int64           `json:"rev"`
	More           bool            `json:"more"`
	Settings       []Setting       `json:"settings,omitempty"`
	Locations      []Location      `json:"locations,omitempty"`
	Users          []SyncUser      `json:"users,omitempty"`
	Categories     []Category      `json:"categories,omitempty"`
	Products       []Product       `json:"products,omitempty"`
	Variants       []Variant       `json:"variants,omitempty"`
	Barcodes       []Barcode       `json:"barcodes,omitempty"`
	PaymentMethods []PaymentMethod `json:"payment_methods,omitempty"`
	StockLevels    []StockLevel    `json:"stock_levels,omitempty"`
}

// ---------- pairing & auth ----------

type HubInfo struct {
	HubID           string `json:"hub_id"`
	StoreName       string `json:"store_name"`
	Version         string `json:"version"`
	ProtocolVersion int    `json:"protocol_version"`
	SetupComplete   bool   `json:"setup_complete"`
	Rev             int64  `json:"rev"`
	ServerTime      int64  `json:"server_time"`
}

type PairRequest struct {
	Code         string `json:"code"`
	TerminalName string `json:"terminal_name"`
	AppVersion   string `json:"app_version"`
}

type PairResponse struct {
	HubID         string `json:"hub_id"`
	StoreName     string `json:"store_name"`
	TerminalID    string `json:"terminal_id"`
	TerminalName  string `json:"terminal_name"`
	Token         string `json:"token"`
	ReceiptPrefix string `json:"receipt_prefix"`
	LocationID    string `json:"location_id"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	SessionToken string   `json:"session_token"`
	ExpiresAt    int64    `json:"expires_at"`
	User         SyncUser `json:"user"`
}

type PairingCodeRequest struct {
	TerminalName string `json:"terminal_name"`
	LocationID   string `json:"location_id"`
}

type PairingCode struct {
	Code      string `json:"code"`
	ExpiresAt int64  `json:"expires_at"`
}

type TerminalInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	LocationID    string `json:"location_id"`
	LocationName  string `json:"location_name"`
	ReceiptPrefix string `json:"receipt_prefix"`
	Active        bool   `json:"active"`
	PairedAt      int64  `json:"paired_at"`
	LastSeenAt    int64  `json:"last_seen_at"`
	LastIP        string `json:"last_ip"`
	AppVersion    string `json:"app_version"`
}

// ProductInput is an upsert of a product with its variants and barcodes.
type ProductInput struct {
	Product  Product        `json:"product"`
	Variants []VariantInput `json:"variants"`
}

type VariantInput struct {
	Variant
	Barcodes []string `json:"barcodes"`
	// OpeningQtyMilli, when non-zero on a new variant, records an opening
	// stock movement at the given location.
	OpeningQtyMilli int64  `json:"opening_qty_milli"`
	OpeningLocation string `json:"opening_location_id"`
}

type StockAdjustRequest struct {
	VariantID  string `json:"variant_id"`
	LocationID string `json:"location_id"`
	// Exactly one of DeltaMilli (relative) or CountMilli (absolute count) is used.
	DeltaMilli *int64 `json:"delta_milli,omitempty"`
	CountMilli *int64 `json:"count_milli,omitempty"`
	Note       string `json:"note"`
}

type UserInput struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Password string `json:"password"` // empty = unchanged
	Pin      string `json:"pin"`      // empty = unchanged
	Active   bool   `json:"active"`
}

// APIError is the JSON error body returned by the hub.
type APIError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}
