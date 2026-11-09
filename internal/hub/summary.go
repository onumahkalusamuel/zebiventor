package hub

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Daily sales summaries. Reports over long periods read these instead of
// every sale line: a year of a busy shop is millions of lines but only
// thousands of summary rows. They are kept up to date in the same
// transaction as each sale and return, and built once for existing data.
//
//	sales_daily        day × hour × cashier × till × location × customer
//	sales_daily_items  day × item × location
//	sales_daily_cats   day × category × location
//	payments_daily     day × payment method
//
// docs counts sales (not returns), once per row: so "transactions" stays
// exact whichever way a report is grouped.

const (
	sqlDay  = "date(%s/1000,'unixepoch','localtime')"
	sqlHour = "CAST(strftime('%%H',%s/1000,'unixepoch','localtime') AS INTEGER)"
)

func dayOf(col string) string { return strings.ReplaceAll(sqlDay, "%s", col) }
func hourOf(col string) string {
	return strings.ReplaceAll(strings.ReplaceAll(sqlHour, "%%", "%"), "%s", col)
}

// Sums shared by every summary, as the line-level expressions of a sale
// line (l, s) and of a return line (rl, r, sl).
const (
	saleSums = `sum(l.qty_milli), sum(l.total_minor), sum(l.discount_minor), sum(l.tax_minor), 0,
		sum(l.total_minor - l.discount_minor - CASE WHEN s.tax_mode='inclusive' THEN l.tax_minor ELSE 0 END),
		sum((l.qty_milli * l.cost_minor + 500) / 1000)`
	returnSums = `sum(-rl.qty_milli), 0, 0, sum(-rl.tax_minor), sum(rl.total_minor), sum(-(rl.total_minor - rl.tax_minor)),
		sum(-CASE WHEN rl.restock THEN (rl.qty_milli * COALESCE(sl.cost_minor,0) + 500) / 1000 ELSE 0 END)`
	addSums = `qty=qty+excluded.qty, gross=gross+excluded.gross, disc=disc+excluded.disc, tax=tax+excluded.tax,
		ret=ret+excluded.ret, net=net+excluded.net, cost=cost+excluded.cost`
)

func summaryStatements(saleWhere, returnWhere string) []string {
	sDay, sHour := dayOf("s.created_at"), hourOf("s.created_at")
	rDay, rHour := dayOf("r.created_at"), hourOf("r.created_at")
	saleFrom := " FROM sale_lines l JOIN sales s ON s.id=l.sale_id LEFT JOIN variants v ON v.id=l.variant_id LEFT JOIN products p ON p.id=v.product_id WHERE s.status='completed' AND " + saleWhere
	retFrom := ` FROM return_lines rl JOIN returns r ON r.id=rl.return_id LEFT JOIN sale_lines sl ON sl.id=rl.sale_line_id
		LEFT JOIN sales os ON os.id=r.sale_id LEFT JOIN variants v ON v.id=rl.variant_id LEFT JOIN products p ON p.id=v.product_id WHERE ` + returnWhere
	rCust := "COALESCE(NULLIF(r.customer_id,''), os.customer_id, '')"
	return []string{
		// Sales: one doc per sale, change given.
		`INSERT INTO sales_daily(day,hour,user_id,terminal_id,location_id,customer_id,user_name,docs,change_minor,qty,gross,disc,tax,ret,net,cost)
		SELECT ` + sDay + `,` + sHour + `,s.user_id,s.terminal_id,s.location_id,s.customer_id,max(s.user_name),count(*),sum(s.change_minor),0,0,0,0,0,0,0
		FROM sales s WHERE s.status='completed' AND ` + saleWhere + ` GROUP BY 1,2,3,4,5,6
		ON CONFLICT DO UPDATE SET docs=docs+excluded.docs, change_minor=change_minor+excluded.change_minor, user_name=excluded.user_name`,
		`INSERT INTO sales_daily(day,hour,user_id,terminal_id,location_id,customer_id,user_name,docs,change_minor,qty,gross,disc,tax,ret,net,cost)
		SELECT ` + sDay + `,` + sHour + `,s.user_id,s.terminal_id,s.location_id,s.customer_id,max(s.user_name),0,0,` + saleSums + saleFrom + ` GROUP BY 1,2,3,4,5,6
		ON CONFLICT DO UPDATE SET ` + addSums,
		`INSERT INTO sales_daily(day,hour,user_id,terminal_id,location_id,customer_id,user_name,docs,change_minor,qty,gross,disc,tax,ret,net,cost)
		SELECT ` + rDay + `,` + rHour + `,r.user_id,r.terminal_id,r.location_id,` + rCust + `,max(r.user_name),0,0,` + returnSums + retFrom + ` GROUP BY 1,2,3,4,5,6
		ON CONFLICT DO UPDATE SET ` + addSums,
		// Items.
		`INSERT INTO sales_daily_items(variant_id,location_id,day,product_name,sub,docs,qty,gross,disc,tax,ret,net,cost)
		SELECT l.variant_id,s.location_id,` + sDay + `,max(l.product_name),max(CASE WHEN l.variant_name<>'' THEN l.variant_name ELSE l.sku END),
			count(DISTINCT s.id),` + saleSums + saleFrom + ` GROUP BY 1,2,3
		ON CONFLICT DO UPDATE SET docs=docs+excluded.docs, product_name=excluded.product_name, sub=excluded.sub, ` + addSums,
		`INSERT INTO sales_daily_items(variant_id,location_id,day,product_name,sub,docs,qty,gross,disc,tax,ret,net,cost)
		SELECT rl.variant_id,r.location_id,` + rDay + `,max(rl.product_name),max(CASE WHEN rl.variant_name<>'' THEN rl.variant_name ELSE rl.sku END),
			0,` + returnSums + retFrom + ` GROUP BY 1,2,3
		ON CONFLICT DO UPDATE SET ` + addSums,
		// Categories (as the item is categorised when the sale is recorded).
		`INSERT INTO sales_daily_cats(category_id,location_id,day,docs,qty,gross,disc,tax,ret,net,cost)
		SELECT COALESCE(p.category_id,''),s.location_id,` + sDay + `,count(DISTINCT s.id),` + saleSums + saleFrom + ` GROUP BY 1,2,3
		ON CONFLICT DO UPDATE SET docs=docs+excluded.docs, ` + addSums,
		`INSERT INTO sales_daily_cats(category_id,location_id,day,docs,qty,gross,disc,tax,ret,net,cost)
		SELECT COALESCE(p.category_id,''),r.location_id,` + rDay + `,0,` + returnSums + retFrom + ` GROUP BY 1,2,3
		ON CONFLICT DO UPDATE SET ` + addSums,
		// Payments taken at sales.
		`INSERT INTO payments_daily(day,method_name,kind,amount_minor)
		SELECT ` + sDay + `,p.method_name,p.kind,sum(p.amount_minor) FROM sale_payments p JOIN sales s ON s.id=p.sale_id
		WHERE s.status='completed' AND ` + saleWhere + ` GROUP BY 1,2,3
		ON CONFLICT DO UPDATE SET amount_minor=amount_minor+excluded.amount_minor`,
	}
}

// summariseSale adds one sale to the summaries (same transaction).
func summariseSale(ctx context.Context, tx *sql.Tx, saleID string) error {
	for i, q := range summaryStatements("s.id=?1", "0") {
		if i == 2 || i == 4 || i == 6 { // return statements
			continue
		}
		if _, err := tx.ExecContext(ctx, q, saleID); err != nil {
			return err
		}
	}
	return nil
}

// summariseReturn adds one return to the summaries (same transaction).
func summariseReturn(ctx context.Context, tx *sql.Tx, returnID string) error {
	for i, q := range summaryStatements("0", "r.id=?1") {
		if i == 2 || i == 4 || i == 6 {
			if _, err := tx.ExecContext(ctx, q, returnID); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildSummaries fills the summaries from all existing sales and returns
// (once, after the upgrade that adds them).
func (h *Hub) buildSummaries(ctx context.Context) error {
	if v, _ := h.Setting(ctx, "summaries_built"); v == "1" {
		return nil
	}
	return h.RebuildSummaries(ctx)
}

// RebuildSummaries recomputes the summaries from every sale and return.
func (h *Hub) RebuildSummaries(ctx context.Context) error {
	start := time.Now()
	err := h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		for _, t := range []string{"sales_daily", "sales_daily_items", "sales_daily_cats", "payments_daily"} {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+t); err != nil {
				return err
			}
		}
		for _, q := range summaryStatements("1", "1") {
			if _, err := tx.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES('summaries_built','1',0)
			ON CONFLICT(key) DO UPDATE SET value='1'`)
		return err
	})
	if err == nil {
		h.log.Info("sales summaries built", "took", time.Since(start).Round(time.Millisecond))
	}
	return err
}

// dayRange turns a [from, to) period into summary days when both ends are
// local midnights (every period the app offers); ok is false otherwise.
func dayRange(from, to int64) (fromDay, toDay string, ok bool) {
	f, t := time.UnixMilli(from), time.UnixMilli(to)
	mid := func(x time.Time) bool {
		return x.Hour() == 0 && x.Minute() == 0 && x.Second() == 0 && x.Nanosecond() == 0
	}
	if !mid(f) || !mid(t) {
		return "", "", false
	}
	return f.Format("2006-01-02"), t.Format("2006-01-02"), true
}
