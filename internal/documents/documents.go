// Package documents renders A4 paperwork (purchase orders, transfer notes,
// supplier statements and payment advice) as self-contained HTML. The page
// opens in the system browser, which prints it or saves it as PDF.
package documents

import (
	"bytes"
	_ "embed"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/money"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

//go:embed page.html
var pageHTML string

var page = template.Must(template.New("page").Parse(pageHTML))

// Store is the letterhead.
type Store struct {
	Name  string
	Lines []string // address, phone, email, tax id
}

// StoreFromSettings builds the letterhead from the store settings.
func StoreFromSettings(s map[string]string) Store {
	st := Store{Name: s["store_name"]}
	for _, k := range []string{"store_address", "store_phone", "store_email"} {
		for _, line := range strings.Split(s[k], "\n") {
			if line = strings.TrimSpace(line); line != "" {
				st.Lines = append(st.Lines, line)
			}
		}
	}
	if v := strings.TrimSpace(s["tax_id"]); v != "" {
		st.Lines = append(st.Lines, "TIN: "+v)
	}
	return st
}

// Pair is a labelled value.
type Pair struct{ Label, Value string }

// Column of the main table.
type Column struct {
	Title string
	Right bool
}

// Doc is one printable document.
type Doc struct {
	Title     string // e.g. "Purchase order"
	Number    string // e.g. PO-000001
	Store     Store
	PartyHead string   // e.g. "Supplier"
	Party     []string // name first
	Meta      []Pair
	Columns   []Column
	Rows      [][]string
	Totals    []Pair
	Notes     string
	Signoffs  []string // signature boxes, e.g. "Sent by", "Received by"
	Footer    string
	Printed   string
	AutoPrint bool
}

// HTML renders the document.
func (d Doc) HTML() ([]byte, error) {
	if d.Printed == "" {
		d.Printed = time.Now().Format("2 Jan 2006 15:04")
	}
	var b bytes.Buffer
	err := page.Execute(&b, d)
	return b.Bytes(), err
}

func day(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).Format("2 Jan 2006")
}

func itemName(product, variant string) string {
	if variant == "" || strings.EqualFold(variant, "default") {
		return product
	}
	return product + " – " + variant
}

func party(name string, lines ...string) []string {
	out := []string{name}
	for _, l := range lines {
		for _, part := range strings.Split(l, "\n") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// PurchaseOrder lays out an order to send to the supplier.
func PurchaseOrder(o *protocol.PurchaseOrder, sup *protocol.Supplier, store Store, c money.Currency) Doc {
	d := Doc{Title: "Purchase order", Number: o.PONo, Store: store, PartyHead: "Supplier", AutoPrint: true,
		Columns: []Column{{Title: "#"}, {Title: "Item"}, {Title: "SKU"}, {Title: "Qty", Right: true}, {Title: "Unit cost", Right: true}, {Title: "Amount", Right: true}},
		Notes:   o.Note, Signoffs: []string{"Authorised by"}}
	if sup != nil {
		d.Party = party(sup.Name, sup.ContactName, sup.Address, sup.Phone, sup.Email)
	} else {
		d.Party = []string{o.SupplierName}
	}
	date := o.OrderedAt
	if date == 0 {
		date = o.CreatedAt
	}
	d.Meta = []Pair{{"Order date", day(date)}, {"Deliver to", o.LocationName}}
	if o.ExpectedAt > 0 {
		d.Meta = append(d.Meta, Pair{"Expected by", day(o.ExpectedAt)})
	}
	if o.Status == protocol.PODraft {
		d.Meta = append(d.Meta, Pair{"Status", "DRAFT – not yet sent"})
	}
	var total int64
	for i, l := range o.Lines {
		amt := l.QtyOrderedMilli * l.UnitCostMinor / money.QtyScale
		total += amt
		qty := money.FormatQty(l.QtyOrderedMilli)
		if l.Unit != "" && l.Unit != "pc" {
			qty += " " + l.Unit
		}
		d.Rows = append(d.Rows, []string{itoa(i + 1), itemName(l.ProductName, l.VariantName), l.SKU, qty, c.Format(l.UnitCostMinor), c.Format(amt)})
	}
	d.Totals = []Pair{{"Total", c.FormatSymbol(total)}}
	d.Footer = "Please quote " + o.PONo + " on your delivery note and invoice."
	return d
}

// TransferNote travels with goods moved between locations.
func TransferNote(t *protocol.StockTransfer, store Store) Doc {
	d := Doc{Title: "Transfer note", Number: t.TransferNo, Store: store, PartyHead: "Deliver to", AutoPrint: true,
		Party:    []string{t.ToLocationName},
		Meta:     []Pair{{"From", t.FromLocationName}, {"Sent", day(t.CreatedAt)}, {"Sent by", t.CreatedBy}},
		Notes:    t.Note,
		Signoffs: []string{"Sent by", "Carried by", "Received by"}}
	received := t.Status == protocol.TransferReceived
	d.Columns = []Column{{Title: "#"}, {Title: "Item"}, {Title: "SKU"}, {Title: "Qty sent", Right: true}}
	if received {
		d.Columns = append(d.Columns, Column{Title: "Received", Right: true})
		d.Meta = append(d.Meta, Pair{"Received", day(t.ReceivedAt)}, Pair{"Received by", t.ReceivedBy})
		d.Signoffs = nil
	} else {
		d.Columns = append(d.Columns, Column{Title: "Qty received", Right: true})
	}
	var sent, got int64
	for i, l := range t.Lines {
		row := []string{itoa(i + 1), itemName(l.ProductName, l.VariantName), l.SKU, money.FormatQty(l.QtyMilli), ""}
		if received {
			row[4] = money.FormatQty(l.ReceivedQtyMilli)
		}
		sent += l.QtyMilli
		got += l.ReceivedQtyMilli
		d.Rows = append(d.Rows, row)
	}
	d.Totals = []Pair{{"Items sent", money.FormatQty(sent)}}
	if received {
		d.Totals = append(d.Totals, Pair{"Items received", money.FormatQty(got)})
		if t.ReceiveNote != "" {
			d.Notes = strings.TrimSpace(d.Notes + "\nOn receipt: " + t.ReceiveNote)
		}
	}
	if t.Status == protocol.TransferCancelled {
		d.Meta = append(d.Meta, Pair{"Status", "CANCELLED"})
	}
	return d
}

var ledgerKinds = map[string]string{"bill": "Bill", "payment": "Payment", "void": "Bill voided", "opening": "Opening balance", "adjustment": "Adjustment"}

// SupplierStatement lists what was billed and paid, with the balance owed.
func SupplierStatement(st *protocol.SupplierStatement, store Store, c money.Currency) Doc {
	s := st.Supplier
	d := Doc{Title: "Supplier statement", Store: store, PartyHead: "Supplier", AutoPrint: true,
		Party:   party(s.Name, s.ContactName, s.Address, s.Phone, s.Email),
		Meta:    []Pair{{"Date", day(time.Now().UnixMilli())}},
		Columns: []Column{{Title: "Date"}, {Title: "Type"}, {Title: "Reference"}, {Title: "Details"}, {Title: "Billed", Right: true}, {Title: "Paid", Right: true}, {Title: "Balance", Right: true}}}
	if s.TermsDays > 0 {
		d.Meta = append(d.Meta, Pair{"Terms", itoa(int(s.TermsDays)) + " days"})
	}
	for _, e := range st.Entries {
		billed, paid := "", ""
		if e.AmountMinor >= 0 {
			billed = c.Format(e.AmountMinor)
		} else {
			paid = c.Format(-e.AmountMinor)
		}
		kind := ledgerKinds[e.Kind]
		if kind == "" {
			kind = e.Kind
		}
		d.Rows = append(d.Rows, []string{day(e.At), kind, e.RefNo, e.Note, billed, paid, c.Format(e.BalanceMinor)})
	}
	d.Totals = []Pair{{"Balance owed", c.FormatSymbol(s.BalanceMinor)}}
	if s.OverdueMinor > 0 {
		d.Totals = append(d.Totals, Pair{"of which overdue", c.FormatSymbol(s.OverdueMinor)})
	}
	if len(st.Bills) > 0 {
		var lines []string
		for _, b := range st.Bills {
			ref := b.BillNo
			if b.SupplierRef != "" {
				ref += " (" + b.SupplierRef + ")"
			}
			lines = append(lines, ref+": "+c.FormatSymbol(b.AmountMinor-b.PaidMinor)+" due "+day(b.DueDate))
		}
		d.Notes = "Unpaid bills\n" + strings.Join(lines, "\n")
	}
	return d
}

// PaymentAdvice tells a supplier which bills a payment covers.
func PaymentAdvice(p *protocol.SupplierPayment, sup *protocol.Supplier, store Store, c money.Currency) Doc {
	d := Doc{Title: "Payment advice", Number: p.PaymentNo, Store: store, PartyHead: "Paid to", AutoPrint: true,
		Meta:    []Pair{{"Date paid", day(p.PaidAt)}, {"Method", p.Method}},
		Columns: []Column{{Title: "Our bill"}, {Title: "Your invoice"}, {Title: "Amount paid", Right: true}},
		Notes:   p.Note}
	if sup != nil {
		d.Party = party(sup.Name, sup.ContactName, sup.Address, sup.Phone, sup.Email)
	} else {
		d.Party = []string{p.SupplierName}
	}
	if p.Reference != "" {
		d.Meta = append(d.Meta, Pair{"Reference", p.Reference})
	}
	var used int64
	for _, a := range p.Allocations {
		used += a.AmountMinor
		d.Rows = append(d.Rows, []string{a.BillNo, a.SupplierRef, c.Format(a.AmountMinor)})
	}
	if left := p.AmountMinor - used; left > 0 {
		d.Rows = append(d.Rows, []string{"On account", "", c.Format(left)})
	}
	d.Totals = []Pair{{"Total paid", c.FormatSymbol(p.AmountMinor)}}
	return d
}

func itoa(n int) string { return strconv.Itoa(n) }
