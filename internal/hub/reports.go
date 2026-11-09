package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// Report groupings.
var salesGroups = map[string]struct{ key, label, sub string }{
	"day":      {"date(f.at/1000,'unixepoch','localtime')", "date(f.at/1000,'unixepoch','localtime')", "''"},
	"hour":     {"strftime('%H',f.at/1000,'unixepoch','localtime')", "strftime('%H:00',f.at/1000,'unixepoch','localtime')", "''"},
	"weekday":  {"strftime('%w',f.at/1000,'unixepoch','localtime')", "strftime('%w',f.at/1000,'unixepoch','localtime')", "''"},
	"month":    {"strftime('%Y-%m',f.at/1000,'unixepoch','localtime')", "strftime('%Y-%m',f.at/1000,'unixepoch','localtime')", "''"},
	"weekhour": {"strftime('%w-%H',f.at/1000,'unixepoch','localtime')", "strftime('%w-%H',f.at/1000,'unixepoch','localtime')", "''"},
	"product":  {"f.variant_id", "max(f.product_name)", "max(CASE WHEN f.variant_name<>'' THEN f.variant_name ELSE f.sku END)"},
	"category": {"COALESCE(p.category_id,'')", "COALESCE(max(c.name),'Uncategorised')", "''"},
	"cashier":  {"f.user_id", "max(f.user_name)", "''"},
	"till":     {"f.terminal_id", "COALESCE(max(t.name),f.terminal_id)", "''"},
	"customer": {"f.customer_id", "COALESCE(max(cu.name),'Walk-in customers')", "''"},
	"location": {"f.location_id", "COALESCE(max(l.name),f.location_id)", "''"},
}

// The fact table: sale lines (+) and return lines (-) in the period. net is
// revenue excluding tax after discounts; cost is cost of goods sold (a
// return that goes back on the shelf reverses its cost; one that does not
// is a loss and keeps it).
const salesFacts = `WITH f AS (
	SELECT s.created_at AS at, s.id AS doc, 1 AS is_sale, s.user_id, s.user_name, s.terminal_id, s.location_id, s.customer_id,
		l.variant_id, l.product_name, l.variant_name, l.sku, l.qty_milli AS qty, l.total_minor AS gross, l.discount_minor AS disc,
		l.tax_minor AS tax, l.total_minor - l.discount_minor - CASE WHEN s.tax_mode='inclusive' THEN l.tax_minor ELSE 0 END AS net,
		(l.qty_milli * l.cost_minor + 500) / 1000 AS cost, 0 AS ret
	FROM sale_lines l JOIN sales s ON s.id=l.sale_id
	WHERE s.created_at >= ?1 AND s.created_at < ?2 AND s.status='completed'
	UNION ALL
	SELECT r.created_at, r.id, 0, r.user_id, r.user_name, r.terminal_id, r.location_id, COALESCE(NULLIF(r.customer_id,''), os.customer_id, ''),
		rl.variant_id, rl.product_name, rl.variant_name, rl.sku, -rl.qty_milli, 0, 0,
		-rl.tax_minor, -(rl.total_minor - rl.tax_minor),
		-CASE WHEN rl.restock THEN (rl.qty_milli * COALESCE(sl.cost_minor,0) + 500) / 1000 ELSE 0 END, rl.total_minor
	FROM return_lines rl JOIN returns r ON r.id=rl.return_id LEFT JOIN sale_lines sl ON sl.id=rl.sale_line_id LEFT JOIN sales os ON os.id=r.sale_id
	WHERE r.created_at >= ?1 AND r.created_at < ?2
)`

// SalesReport aggregates sales minus returns over [from, to) by group.
func (h *Hub) SalesReport(ctx context.Context, by *Session, from, to int64, group string) (*protocol.SalesReport, error) {
	if !by.Can(managerRoles...) {
		return nil, ErrForbidden
	}
	g, ok := salesGroups[group]
	if !ok {
		return nil, invalid("unknown report grouping %q", group)
	}
	if to <= from {
		return nil, invalid("the end of the period must be after its start")
	}
	q := salesFacts + `
	SELECT ` + g.key + `, ` + g.label + `, ` + g.sub + `,
		count(DISTINCT CASE WHEN f.is_sale=1 THEN f.doc END), sum(f.qty), sum(f.gross), sum(f.disc), sum(f.tax), sum(f.ret), sum(f.net), sum(f.cost)
	FROM f LEFT JOIN variants v ON v.id=f.variant_id LEFT JOIN products p ON p.id=v.product_id LEFT JOIN categories c ON c.id=p.category_id
		LEFT JOIN terminals t ON t.id=f.terminal_id LEFT JOIN customers cu ON cu.id=f.customer_id LEFT JOIN locations l ON l.id=f.location_id
	GROUP BY 1 ORDER BY ` + map[bool]string{true: "1", false: "10 DESC"}[group == "day" || group == "hour" || group == "weekday" || group == "month" || group == "weekhour"]
	fromDay, toDay, whole := dayRange(from, to)
	var rows *sql.Rows
	var err error
	if whole {
		rows, err = h.db.R().QueryContext(ctx, summaryQuery(group), fromDay, toDay)
	} else {
		rows, err = h.db.R().QueryContext(ctx, q, from, to)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rep := &protocol.SalesReport{From: from, To: to, Group: group, Rows: []protocol.SalesReportRow{}}
	weekdays := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	for rows.Next() {
		var r protocol.SalesReportRow
		if err := rows.Scan(&r.Key, &r.Label, &r.Sub, &r.Transactions, &r.QtyMilli, &r.GrossMinor, &r.DiscountMinor, &r.TaxMinor,
			&r.ReturnsMinor, &r.NetMinor, &r.CostMinor); err != nil {
			return nil, err
		}
		if group == "weekday" && len(r.Key) == 1 {
			r.Label = weekdays[r.Key[0]-'0']
		}
		r.ProfitMinor = r.NetMinor - r.CostMinor
		t := &rep.Totals
		t.QtyMilli += r.QtyMilli
		t.GrossMinor += r.GrossMinor
		t.DiscountMinor += r.DiscountMinor
		t.TaxMinor += r.TaxMinor
		t.ReturnsMinor += r.ReturnsMinor
		t.NetMinor += r.NetMinor
		t.CostMinor += r.CostMinor
		t.ProfitMinor += r.ProfitMinor
		rep.Rows = append(rep.Rows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rep.Totals.Label = "Total"
	if whole {
		h.db.R().QueryRowContext(ctx, "SELECT COALESCE(sum(docs),0) FROM sales_daily WHERE day>=? AND day<?", fromDay, toDay).Scan(&rep.Totals.Transactions)
	} else {
		h.db.R().QueryRowContext(ctx, "SELECT count(*) FROM sales WHERE created_at>=? AND created_at<? AND status='completed'", from, to).Scan(&rep.Totals.Transactions)
	}
	rep.Payments, err = h.paymentTotals(ctx, from, to)
	return rep, err
}

// paymentTotals is money in and out by payment method over a period:
// sales (cash net of change), refunds and payments received on account.
func (h *Hub) paymentTotals(ctx context.Context, from, to int64) ([]protocol.MethodTotal, error) {
	r := h.db.R()
	salePayments := `SELECT p.method_name, p.kind, sum(p.amount_minor) FROM sale_payments p JOIN sales s ON s.id=p.sale_id
			WHERE s.created_at>=? AND s.created_at<? AND s.status='completed' GROUP BY 1,2`
	changeQ := "SELECT COALESCE(sum(change_minor),0) FROM sales WHERE created_at>=? AND created_at<? AND status='completed'"
	fromDay, toDay, whole := dayRange(from, to)
	if whole {
		salePayments = "SELECT method_name, kind, sum(amount_minor) FROM payments_daily WHERE day>=? AND day<? GROUP BY 1,2"
		changeQ = "SELECT COALESCE(sum(change_minor),0) FROM sales_daily WHERE day>=? AND day<?"
	}
	methods := map[string]*protocol.MethodTotal{}
	get := func(name, kind string) *protocol.MethodTotal {
		m, ok := methods[name]
		if !ok {
			m = &protocol.MethodTotal{MethodName: name, Kind: kind}
			methods[name] = m
		}
		return m
	}
	for _, src := range []struct {
		q   string
		add func(*protocol.MethodTotal, int64)
	}{
		{salePayments, func(m *protocol.MethodTotal, v int64) { m.SalesMinor += v }},
		{`SELECT f.method_name, f.kind, sum(f.amount_minor) FROM return_refunds f JOIN returns r ON r.id=f.return_id
			WHERE r.created_at>=? AND r.created_at<? GROUP BY 1,2`, func(m *protocol.MethodTotal, v int64) { m.RefundsMinor += v }},
		{`SELECT method_name, kind, sum(amount_minor) FROM account_payments WHERE created_at>=? AND created_at<? GROUP BY 1,2`,
			func(m *protocol.MethodTotal, v int64) { m.PaymentsMinor += v }},
	} {
		var a, b any = from, to
		if whole && (src.q == salePayments) {
			a, b = fromDay, toDay
		}
		rows, err := r.QueryContext(ctx, src.q, a, b)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var name, kind string
			var v int64
			rows.Scan(&name, &kind, &v)
			src.add(get(name, kind), v)
		}
		rows.Close()
	}
	var change int64
	if whole {
		r.QueryRowContext(ctx, changeQ, fromDay, toDay).Scan(&change)
	} else {
		r.QueryRowContext(ctx, changeQ, from, to).Scan(&change)
	}
	if change > 0 {
		cash := ""
		for name, m := range methods {
			if m.Kind == protocol.PayCash && (cash == "" || name < cash) {
				cash = name
			}
		}
		if cash == "" {
			cash = "Cash"
		}
		get(cash, protocol.PayCash).SalesMinor -= change
	}
	out := []protocol.MethodTotal{}
	for _, m := range methods {
		m.NetMinor = m.SalesMinor - m.RefundsMinor + m.PaymentsMinor
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NetMinor > out[j].NetMinor })
	return out, nil
}

// StockReportFilter narrows the stock report.
type StockReportFilter struct {
	LocationID string
	CategoryID string
	Query      string
	Only       string // "" | low | out | negative
	Limit      int
}

// StockReport lists stock on hand with its value at cost and at retail.
func (h *Hub) StockReport(ctx context.Context, by *Session, f StockReportFilter) (*protocol.StockReport, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	r := h.db.R()
	if f.LocationID == "" {
		f.LocationID = defaultLocation(ctx, r)
	}
	if f.Limit <= 0 || f.Limit > 20000 {
		f.Limit = 5000
	}
	since := time.UnixMilli(startOfDayMs()).AddDate(0, 0, -30).Format("2006-01-02")
	query := `SELECT v.id, p.name, v.name, v.sku, COALESCE(c.name,''), p.unit, COALESCE(s.qty_milli,0), v.reorder_level_milli, v.cost_minor, v.price_minor,
		COALESCE((SELECT sum(d.qty) FROM sales_daily_items d WHERE d.variant_id=v.id AND d.location_id=? AND d.day>=?),0)
		FROM variants v JOIN products p ON p.id=v.product_id LEFT JOIN categories c ON c.id=p.category_id
		LEFT JOIN stock_levels s ON s.variant_id=v.id AND s.location_id=?`
	conds := []string{"p.active=1", "v.active=1", "p.track_stock=1"}
	args := []any{f.LocationID, since, f.LocationID}
	if f.CategoryID != "" {
		conds = append(conds, "p.category_id=?")
		args = append(args, f.CategoryID)
	}
	for _, w := range strings.Fields(strings.ToLower(f.Query)) {
		like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(w) + "%"
		conds = append(conds, `(lower(p.name) LIKE ? ESCAPE '\' OR lower(v.name) LIKE ? ESCAPE '\' OR lower(v.sku) LIKE ? ESCAPE '\' OR lower(p.attributes) LIKE ? ESCAPE '\'
			OR EXISTS (SELECT 1 FROM barcodes b WHERE b.variant_id=v.id AND b.deleted=0 AND b.code=?))`)
		args = append(args, like, like, like, like, w)
	}
	switch f.Only {
	case "low":
		conds = append(conds, "v.reorder_level_milli>0 AND COALESCE(s.qty_milli,0) <= v.reorder_level_milli")
	case "out":
		conds = append(conds, "COALESCE(s.qty_milli,0) <= 0")
	case "negative":
		conds = append(conds, "COALESCE(s.qty_milli,0) < 0")
	case "":
	default:
		return nil, invalid("unknown stock filter %q", f.Only)
	}
	args = append(args, f.Limit)
	rows, err := r.QueryContext(ctx, query+" WHERE "+strings.Join(conds, " AND ")+" ORDER BY p.name COLLATE NOCASE, v.name LIMIT ?", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var locName string
	r.QueryRowContext(ctx, "SELECT name FROM locations WHERE id=?", f.LocationID).Scan(&locName)
	rep := &protocol.StockReport{Rows: []protocol.StockReportRow{}}
	for rows.Next() {
		x := protocol.StockReportRow{LocationID: f.LocationID, LocationName: locName}
		if err := rows.Scan(&x.VariantID, &x.ProductName, &x.VariantName, &x.SKU, &x.CategoryName, &x.Unit, &x.QtyMilli, &x.ReorderLevelMilli,
			&x.CostMinor, &x.PriceMinor, &x.SoldMilli30d); err != nil {
			return nil, err
		}
		if x.QtyMilli > 0 {
			x.ValueMinor = mulDiv(x.QtyMilli, x.CostMinor, 1000)
			x.RetailMinor = mulDiv(x.QtyMilli, x.PriceMinor, 1000)
		}
		rep.ValueMinor += x.ValueMinor
		rep.RetailMinor += x.RetailMinor
		if x.QtyMilli <= 0 {
			rep.OutCount++
		} else if x.ReorderLevelMilli > 0 && x.QtyMilli <= x.ReorderLevelMilli {
			rep.LowCount++
		}
		rep.Rows = append(rep.Rows, x)
	}
	return rep, rows.Err()
}

// ShiftReport lists shifts opened in a period across all tills.
func (h *Hub) ShiftReport(ctx context.Context, by *Session, from, to int64) ([]protocol.ShiftReportRow, error) {
	if !by.Can(managerRoles...) {
		return nil, ErrForbidden
	}
	rows, err := h.db.R().QueryContext(ctx, `SELECT s.id,s.terminal_id,COALESCE(t.name,''),s.location_id,s.status,s.opened_by,s.opened_by_name,s.opened_at,
		s.opening_float_minor,s.closed_by,s.closed_by_name,s.closed_at,s.counted_cash_minor,s.expected_cash_minor,s.variance_minor,s.note,s.summary
		FROM shifts s LEFT JOIN terminals t ON t.id=s.terminal_id WHERE s.opened_at>=? AND s.opened_at<? ORDER BY s.opened_at DESC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.ShiftReportRow{}
	for rows.Next() {
		var x protocol.ShiftReportRow
		var summary string
		if err := rows.Scan(&x.ID, &x.TerminalID, &x.TerminalName, &x.LocationID, &x.Status, &x.OpenedBy, &x.OpenedByName, &x.OpenedAt,
			&x.OpeningFloatMinor, &x.ClosedBy, &x.ClosedByName, &x.ClosedAt, &x.CountedCashMinor, &x.Expected, &x.Variance, &x.Note, &summary); err != nil {
			return nil, err
		}
		if x.Status == "closed" && summary != "" && summary != "{}" && summary != "null" {
			x.Summary = &protocol.ShiftSummary{}
			_ = json.Unmarshal([]byte(summary), x.Summary)
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// AuditLog lists recorded events in a period, newest first.
func (h *Hub) AuditLog(ctx context.Context, by *Session, from, to int64, action string) ([]protocol.AuditEntry, error) {
	if !by.Can(managerRoles...) {
		return nil, ErrForbidden
	}
	q := `SELECT a.id,a.at,COALESCE(u.name,a.user_id),COALESCE(t.name,''),a.action,a.entity,a.entity_id,a.details
		FROM audit_log a LEFT JOIN users u ON u.id=a.user_id LEFT JOIN terminals t ON t.id=a.terminal_id WHERE a.at>=? AND a.at<?`
	args := []any{from, to}
	if action != "" {
		q += " AND a.action LIKE ?"
		args = append(args, action+"%")
	}
	rows, err := h.db.R().QueryContext(ctx, q+" ORDER BY a.at DESC LIMIT 1000", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.AuditEntry{}
	for rows.Next() {
		var e protocol.AuditEntry
		if err := rows.Scan(&e.ID, &e.At, &e.UserName, &e.TerminalName, &e.Action, &e.Entity, &e.EntityID, &e.Details); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// summaryQuery reads a sales report from the daily summaries ([?1, ?2)
// days). Columns match the raw report: key, label, sub, transactions,
// qty, gross, discount, tax, returns, net, cost.
func summaryQuery(group string) string {
	const sums = "sum(docs), sum(qty), sum(gross), sum(disc), sum(tax), sum(ret), sum(net), sum(cost)"
	const gsums = "g.docs, g.qty, g.gross, g.disc, g.tax, g.ret, g.net, g.cost"
	const inner = "sum(docs) docs, sum(qty) qty, sum(gross) gross, sum(disc) disc, sum(tax) tax, sum(ret) ret, sum(net) net, sum(cost) cost"
	const rng = " WHERE day>=?1 AND day<?2 "
	byNet := " ORDER BY 10 DESC"
	joined := func(col, table, label string) string {
		return "SELECT g.k, " + label + ", '', " + gsums + " FROM (SELECT " + col + " k, " + inner + " FROM " + table + rng + "GROUP BY 1) g"
	}
	switch group {
	case "day":
		return "SELECT day, day, '', " + sums + " FROM sales_daily" + rng + "GROUP BY day ORDER BY 1"
	case "hour":
		return "SELECT printf('%02d',hour), printf('%02d:00',hour), '', " + sums + " FROM sales_daily" + rng + "GROUP BY hour ORDER BY 1"
	case "weekday":
		return "SELECT strftime('%w',day), strftime('%w',day), '', " + sums + " FROM sales_daily" + rng + "GROUP BY 1 ORDER BY 1"
	case "month":
		return "SELECT substr(day,1,7), substr(day,1,7), '', " + sums + " FROM sales_daily" + rng + "GROUP BY 1 ORDER BY 1"
	case "weekhour":
		return "SELECT strftime('%w',day) || '-' || printf('%02d',hour), strftime('%w',day) || '-' || printf('%02d',hour), '', " + sums + " FROM sales_daily" + rng + "GROUP BY 1 ORDER BY 1"
	case "cashier":
		return "SELECT user_id, max(user_name), '', " + sums + " FROM sales_daily" + rng + "GROUP BY user_id" + byNet
	case "till":
		return joined("terminal_id", "sales_daily", "COALESCE(t.name,g.k)") + " LEFT JOIN terminals t ON t.id=g.k" + byNet
	case "customer":
		return joined("customer_id", "sales_daily", "COALESCE(cu.name,'Walk-in customers')") + " LEFT JOIN customers cu ON cu.id=g.k" + byNet
	case "location":
		return joined("location_id", "sales_daily", "COALESCE(l.name,g.k)") + " LEFT JOIN locations l ON l.id=g.k" + byNet
	case "product":
		return "SELECT variant_id, max(product_name), max(sub), " + sums + " FROM sales_daily_items" + rng + "GROUP BY variant_id" + byNet
	case "category":
		return joined("category_id", "sales_daily_cats", "COALESCE(c.name,'Uncategorised')") + " LEFT JOIN categories c ON c.id=g.k" + byNet
	}
	return ""
}
