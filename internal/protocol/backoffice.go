package protocol

// Back-office records. These live only at the hub and are reached through
// the admin API; tills never store them.

type Supplier struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContactName string `json:"contact_name"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Address     string `json:"address"`
	Notes       string `json:"notes"`
	Active      bool   `json:"active"`
	// Read-only aggregates.
	OpenOrders    int64 `json:"open_orders"`
	ReceivedMinor int64 `json:"received_minor"` // value of all goods received
}

// Purchase order statuses.
const (
	PODraft     = "draft"
	POOrdered   = "ordered"
	POPartial   = "partial"
	POReceived  = "received"
	POCancelled = "cancelled"
)

type PurchaseOrder struct {
	ID           string              `json:"id"`
	PONo         string              `json:"po_no"`
	SupplierID   string              `json:"supplier_id"`
	SupplierName string              `json:"supplier_name"`
	LocationID   string              `json:"location_id"`
	LocationName string              `json:"location_name"`
	Status       string              `json:"status"`
	ExpectedAt   int64               `json:"expected_at"`
	Note         string              `json:"note"`
	CreatedBy    string              `json:"created_by"`
	CreatedAt    int64               `json:"created_at"`
	UpdatedAt    int64               `json:"updated_at"`
	OrderedAt    int64               `json:"ordered_at"`
	TotalMinor   int64               `json:"total_minor"`
	LineCount    int64               `json:"line_count"`
	Lines        []PurchaseOrderLine `json:"lines,omitempty"`
	Receipts     []GoodsReceipt      `json:"receipts,omitempty"`
}

type PurchaseOrderLine struct {
	ID               string `json:"id"`
	VariantID        string `json:"variant_id"`
	ProductName      string `json:"product_name"`
	VariantName      string `json:"variant_name"`
	SKU              string `json:"sku"`
	Unit             string `json:"unit"`
	QtyOrderedMilli  int64  `json:"qty_ordered_milli"`
	QtyReceivedMilli int64  `json:"qty_received_milli"`
	UnitCostMinor    int64  `json:"unit_cost_minor"`
}

// PurchaseOrderInput creates or replaces a draft order.
type PurchaseOrderInput struct {
	ID         string              `json:"id"`
	SupplierID string              `json:"supplier_id"`
	LocationID string              `json:"location_id"`
	ExpectedAt int64               `json:"expected_at"`
	Note       string              `json:"note"`
	Lines      []PurchaseOrderLine `json:"lines"`
	// Submit marks the order as sent to the supplier (draft -> ordered).
	Submit bool `json:"submit"`
}

type GoodsReceipt struct {
	ID           string             `json:"id"`
	GRNNo        string             `json:"grn_no"`
	POID         string             `json:"po_id"`
	PONo         string             `json:"po_no"`
	SupplierID   string             `json:"supplier_id"`
	SupplierName string             `json:"supplier_name"`
	LocationID   string             `json:"location_id"`
	LocationName string             `json:"location_name"`
	SupplierRef  string             `json:"supplier_ref"` // supplier's invoice / delivery note no.
	Note         string             `json:"note"`
	ReceivedBy   string             `json:"received_by"`
	CreatedAt    int64              `json:"created_at"`
	TotalMinor   int64              `json:"total_minor"`
	Lines        []GoodsReceiptLine `json:"lines,omitempty"`
}

type GoodsReceiptLine struct {
	ID            string `json:"id"`
	POLineID      string `json:"po_line_id"`
	VariantID     string `json:"variant_id"`
	ProductName   string `json:"product_name"`
	VariantName   string `json:"variant_name"`
	SKU           string `json:"sku"`
	QtyMilli      int64  `json:"qty_milli"`
	UnitCostMinor int64  `json:"unit_cost_minor"`
	TotalMinor    int64  `json:"total_minor"`
	// NewPriceMinor, when > 0, updates the selling price on receipt.
	NewPriceMinor int64 `json:"new_price_minor,omitempty"`
}

// GoodsReceiptInput books goods into stock, against an order or directly.
type GoodsReceiptInput struct {
	POID        string             `json:"po_id"`
	SupplierID  string             `json:"supplier_id"`
	LocationID  string             `json:"location_id"`
	SupplierRef string             `json:"supplier_ref"`
	Note        string             `json:"note"`
	Lines       []GoodsReceiptLine `json:"lines"`
	// ClosePO marks the order received even if some lines are short.
	ClosePO bool `json:"close_po"`
}

type StockTransfer struct {
	ID               string              `json:"id"`
	TransferNo       string              `json:"transfer_no"`
	FromLocationID   string              `json:"from_location_id"`
	FromLocationName string              `json:"from_location_name"`
	ToLocationID     string              `json:"to_location_id"`
	ToLocationName   string              `json:"to_location_name"`
	Note             string              `json:"note"`
	CreatedBy        string              `json:"created_by"`
	CreatedAt        int64               `json:"created_at"`
	ItemsMilli       int64               `json:"items_milli"`
	Lines            []StockTransferLine `json:"lines,omitempty"`
}

type StockTransferLine struct {
	VariantID   string `json:"variant_id"`
	ProductName string `json:"product_name"`
	VariantName string `json:"variant_name"`
	SKU         string `json:"sku"`
	QtyMilli    int64  `json:"qty_milli"`
}

// Stock count statuses.
const (
	CountOpen      = "open"
	CountPosted    = "posted"
	CountCancelled = "cancelled"
)

// StockCount is a physical count of (part of) a location. Each line's
// expected quantity is taken from the stock ledger as of the moment it was
// counted, so the shop can keep trading while counting.
type StockCount struct {
	ID           string           `json:"id"`
	CountNo      string           `json:"count_no"`
	Name         string           `json:"name"`
	LocationID   string           `json:"location_id"`
	LocationName string           `json:"location_name"`
	CategoryID   string           `json:"category_id"`
	CategoryName string           `json:"category_name"`
	Status       string           `json:"status"`
	CreatedBy    string           `json:"created_by"`
	CreatedAt    int64            `json:"created_at"`
	PostedBy     string           `json:"posted_by"`
	PostedAt     int64            `json:"posted_at"`
	Note         string           `json:"note"`
	LineCount    int64            `json:"line_count"`
	Lines        []StockCountLine `json:"lines,omitempty"`
	// Warnings, e.g. tills that have not synced since counting started.
	Warnings []string `json:"warnings,omitempty"`
}

type StockCountLine struct {
	VariantID     string `json:"variant_id"`
	ProductName   string `json:"product_name"`
	VariantName   string `json:"variant_name"`
	SKU           string `json:"sku"`
	Unit          string `json:"unit"`
	CountedMilli  int64  `json:"counted_milli"`
	CountedAt     int64  `json:"counted_at"`
	CountedBy     string `json:"counted_by"`
	ExpectedMilli int64  `json:"expected_milli"` // ledger as of counted_at (final once posted)
	CostMinor     int64  `json:"cost_minor"`
	Uncounted     bool   `json:"uncounted"` // in scope but not counted (set to zero if posted with zero_uncounted)
}

type StockCountEntry struct {
	VariantID string `json:"variant_id"`
	QtyMilli  int64  `json:"qty_milli"`
	// Mode "add" adds to the existing count (scanning); "set" replaces it.
	Mode string `json:"mode"`
}

type StockCountPost struct {
	ZeroUncounted bool   `json:"zero_uncounted"`
	Note          string `json:"note"`
}

// StockMovement is one entry of the stock ledger.
type StockMovement struct {
	ID           string `json:"id"`
	VariantID    string `json:"variant_id"`
	LocationID   string `json:"location_id"`
	LocationName string `json:"location_name"`
	QtyMilli     int64  `json:"qty_milli"`
	Reason       string `json:"reason"`
	RefType      string `json:"ref_type"`
	RefID        string `json:"ref_id"`
	RefNo        string `json:"ref_no"`
	UserName     string `json:"user_name"`
	TerminalName string `json:"terminal_name"`
	Note         string `json:"note"`
	CreatedAt    int64  `json:"created_at"`
	BalanceMilli int64  `json:"balance_milli"` // running level at that location after this entry
}

// CustomerLedgerEntry is one change to what a customer owes.
type CustomerLedgerEntry struct {
	ID           string `json:"id"`
	At           int64  `json:"at"`
	Kind         string `json:"kind"` // sale | return | payment | adjustment | opening
	RefID        string `json:"ref_id"`
	RefNo        string `json:"ref_no"`
	AmountMinor  int64  `json:"amount_minor"` // + increases the debt
	BalanceMinor int64  `json:"balance_minor"`
	Note         string `json:"note"`
	UserName     string `json:"user_name"`
}

type CustomerStatement struct {
	Customer Customer              `json:"customer"`
	Entries  []CustomerLedgerEntry `json:"entries"`
}

type BalanceAdjustment struct {
	AmountMinor int64  `json:"amount_minor"` // + increases the debt
	Note        string `json:"note"`
}

// ---------- reports ----------

// SalesReportRow aggregates sales minus returns for one group. Net is
// revenue excluding tax after discounts and returns; Profit is Net - Cost.
type SalesReportRow struct {
	Key           string `json:"key"`
	Label         string `json:"label"`
	Sub           string `json:"sub"`
	Transactions  int64  `json:"transactions"`
	QtyMilli      int64  `json:"qty_milli"`
	GrossMinor    int64  `json:"gross_minor"`
	DiscountMinor int64  `json:"discount_minor"`
	TaxMinor      int64  `json:"tax_minor"`
	ReturnsMinor  int64  `json:"returns_minor"`
	NetMinor      int64  `json:"net_minor"`
	CostMinor     int64  `json:"cost_minor"`
	ProfitMinor   int64  `json:"profit_minor"`
}

type SalesReport struct {
	From   int64            `json:"from"`
	To     int64            `json:"to"`
	Group  string           `json:"group"`
	Totals SalesReportRow   `json:"totals"`
	Rows   []SalesReportRow `json:"rows"`
	// Payments by method over the same period.
	Payments []MethodTotal `json:"payments"`
}

type StockReportRow struct {
	VariantID         string `json:"variant_id"`
	ProductName       string `json:"product_name"`
	VariantName       string `json:"variant_name"`
	SKU               string `json:"sku"`
	CategoryName      string `json:"category_name"`
	Unit              string `json:"unit"`
	LocationID        string `json:"location_id"`
	LocationName      string `json:"location_name"`
	QtyMilli          int64  `json:"qty_milli"`
	ReorderLevelMilli int64  `json:"reorder_level_milli"`
	CostMinor         int64  `json:"cost_minor"`
	PriceMinor        int64  `json:"price_minor"`
	ValueMinor        int64  `json:"value_minor"`  // at cost
	RetailMinor       int64  `json:"retail_minor"` // at selling price
	SoldMilli30d      int64  `json:"sold_milli_30d"`
}

type StockReport struct {
	Rows        []StockReportRow `json:"rows"`
	ValueMinor  int64            `json:"value_minor"`
	RetailMinor int64            `json:"retail_minor"`
	LowCount    int64            `json:"low_count"`
	OutCount    int64            `json:"out_count"`
}

type ShiftReportRow struct {
	Shift
	TerminalName string `json:"terminal_name"`
	Expected     int64  `json:"expected_cash_minor"`
	Variance     int64  `json:"variance_minor"`
}

type AuditEntry struct {
	ID           string `json:"id"`
	At           int64  `json:"at"`
	UserName     string `json:"user_name"`
	TerminalName string `json:"terminal_name"`
	Action       string `json:"action"`
	Entity       string `json:"entity"`
	EntityID     string `json:"entity_id"`
	Details      string `json:"details"`
}

// ImportResult describes a catalogue CSV import (or its dry run).
type ImportResult struct {
	DryRun   bool          `json:"dry_run"`
	Rows     int           `json:"rows"`
	Created  int           `json:"created"`
	Updated  int           `json:"updated"`
	Skipped  int           `json:"skipped"`
	Errors   []ImportError `json:"errors"`
	Columns  []string      `json:"columns"`
	Category []string      `json:"new_categories"`
}

type ImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}
