import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { BarChart3, Loader2, ScrollText, Wallet } from "lucide-react"
import { Chips, ExportButton, HubRequired, PeriodPicker, QueryError, usePeriod } from "@/components/backoffice"
import { EmptyState, Page, PageHeader, StatCard } from "@/components/page"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableFooter, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { formatDateTime, formatQty } from "@/lib/format"
import { csvMoney, hub, toCSV } from "@/lib/hub"
import { useMoney } from "@/lib/state"
import type { SalesGroup, SalesReport, SalesReportRow } from "@/lib/types"
import { cn } from "@/lib/utils"

type Tab = "sales" | "shifts" | "activity"

export function Reports() {
  const [tab, setTab] = useState<Tab>("sales")
  const period = usePeriod("today")
  return (
    <Page>
      <PageHeader title="Reports" description="Every till's sales, returns and shifts, as recorded at the hub." actions={<PeriodPicker period={period} />} />
      <HubRequired>
        <div className="space-y-4 px-8 pb-8">
          <Chips
            value={tab}
            onChange={setTab}
            options={[
              { value: "sales", label: "Sales & profit" },
              { value: "shifts", label: "Shifts & cash" },
              { value: "activity", label: "Activity log" },
            ]}
          />
          {tab === "sales" && <SalesTab from={period.from} to={period.to} />}
          {tab === "shifts" && <ShiftsTab from={period.from} to={period.to} />}
          {tab === "activity" && <ActivityTab from={period.from} to={period.to} />}
        </div>
      </HubRequired>
    </Page>
  )
}

const groups: { value: SalesGroup; label: string; col: string }[] = [
  { value: "day", label: "By day", col: "Day" },
  { value: "product", label: "By item", col: "Item" },
  { value: "category", label: "By category", col: "Category" },
  { value: "cashier", label: "By cashier", col: "Cashier" },
  { value: "till", label: "By till", col: "Till" },
  { value: "customer", label: "By customer", col: "Customer" },
  { value: "hour", label: "By hour of day", col: "Hour" },
  { value: "weekday", label: "By day of week", col: "Day of week" },
  { value: "location", label: "By location", col: "Location" },
]

const pct = (num: number, den: number) => (den ? `${((num / den) * 100).toFixed(1)}%` : "–")

function SalesTab({ from, to }: { from: number; to: number }) {
  const { fmt, currency } = useMoney()
  const [group, setGroup] = useState<SalesGroup>("day")
  const rep = useQuery({ queryKey: ["bo", "sales", from, to, group], queryFn: () => hub.salesReport(from, to, group) })
  const r = rep.data
  const t = r?.totals
  const col = groups.find((g) => g.value === group)!.col
  const timeSeries = group === "day" || group === "hour" || group === "weekday"

  return (
    <>
      <QueryError error={rep.error} />
      <div className="grid grid-cols-5 gap-3">
        <StatCard label="Net sales" value={fmt(t?.net_minor ?? 0)} sub="after discounts & returns, excl. tax" />
        <StatCard label="Gross profit" value={fmt(t?.profit_minor ?? 0)} sub={`${pct(t?.profit_minor ?? 0, t?.net_minor ?? 0)} margin`} />
        <StatCard label="Transactions" value={t?.transactions ?? 0} sub={t?.transactions ? `${fmt(Math.round((t.net_minor + t.tax_minor) / t.transactions))} average` : undefined} />
        <StatCard label="Returns" value={fmt(t?.returns_minor ?? 0)} sub={`${fmt(t?.discount_minor ?? 0)} discounts given`} />
        <StatCard label="Tax collected" value={fmt(t?.tax_minor ?? 0)} />
      </div>
      <div className="flex items-center gap-3">
        <Select value={group} onValueChange={(v) => setGroup(v as SalesGroup)}>
          <SelectTrigger className="h-10 w-52">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {groups.map((g) => (
              <SelectItem key={g.value} value={g.value}>
                {g.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {rep.isFetching && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
        <div className="ml-auto">
          <ExportButton
            name={`sales-by-${group}.csv`}
            disabled={!r}
            build={() =>
              toCSV(
                [col, "Detail", "Transactions", "Quantity", "Gross", "Discounts", "Returns", "Tax", "Net sales", "Cost", "Gross profit", "Margin"],
                [...(r?.rows ?? []), ...(t ? [t] : [])].map((x) => [x.label, x.sub, x.transactions, formatQty(x.qty_milli), csvMoney(x.gross_minor, currency.digits),
                  csvMoney(x.discount_minor, currency.digits), csvMoney(x.returns_minor, currency.digits), csvMoney(x.tax_minor, currency.digits),
                  csvMoney(x.net_minor, currency.digits), csvMoney(x.cost_minor, currency.digits), csvMoney(x.profit_minor, currency.digits), pct(x.profit_minor, x.net_minor)]),
              )
            }
          />
        </div>
      </div>
      {r && r.rows.length === 0 ? (
        <EmptyState icon={<BarChart3 />} title="No sales in this period" />
      ) : (
        <>
          {timeSeries && r && r.rows.length > 1 && <NetSalesChart report={r} />}
          <div className="overflow-hidden rounded-xl border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40">
                  <TableHead className="pl-4">{col}</TableHead>
                  {group !== "product" && <TableHead className="text-right">Sales</TableHead>}
                  <TableHead className="text-right">Qty</TableHead>
                  <TableHead className="text-right">Discounts</TableHead>
                  <TableHead className="text-right">Returns</TableHead>
                  <TableHead className="text-right">Net sales</TableHead>
                  <TableHead className="text-right">Cost</TableHead>
                  <TableHead className="text-right">Profit</TableHead>
                  <TableHead className="pr-4 text-right">Margin</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(r?.rows ?? []).map((x) => (
                  <ReportRow key={x.key} x={x} showTx={group !== "product"} />
                ))}
              </TableBody>
              {t && (
                <TableFooter>
                  <ReportRow x={t} showTx={group !== "product"} strong />
                </TableFooter>
              )}
            </Table>
          </div>
          {r && r.payments.length > 0 && (
            <div className="space-y-2">
              <h2 className="text-sm font-semibold">Money in by payment method</h2>
              <div className="overflow-hidden rounded-xl border bg-card">
                <Table>
                  <TableHeader>
                    <TableRow className="bg-muted/40">
                      <TableHead className="pl-4">Method</TableHead>
                      <TableHead className="text-right">Sales</TableHead>
                      <TableHead className="text-right">Refunds</TableHead>
                      <TableHead className="text-right">Paid on accounts</TableHead>
                      <TableHead className="pr-4 text-right">Net</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {r.payments.map((m) => (
                      <TableRow key={m.method_name}>
                        <TableCell className="pl-4 font-medium">
                          {m.method_name}
                          {m.kind === "credit" && <span className="ml-2 text-xs text-muted-foreground">(not money received)</span>}
                        </TableCell>
                        <TableCell className="text-right tabular">{fmt(m.sales_minor)}</TableCell>
                        <TableCell className="text-right tabular">{m.refunds_minor ? fmt(-m.refunds_minor) : "–"}</TableCell>
                        <TableCell className="text-right tabular">{m.payments_minor ? fmt(m.payments_minor) : "–"}</TableCell>
                        <TableCell className="pr-4 text-right font-semibold tabular">{fmt(m.net_minor)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </div>
          )}
        </>
      )}
    </>
  )
}

function ReportRow({ x, showTx, strong }: { x: SalesReportRow; showTx: boolean; strong?: boolean }) {
  const { fmt } = useMoney()
  return (
    <TableRow className={cn(strong && "font-semibold")}>
      <TableCell className="pl-4">
        <div className={cn(!strong && "font-medium")}>{x.label}</div>
        {x.sub && <div className="text-xs font-normal text-muted-foreground">{x.sub}</div>}
      </TableCell>
      {showTx && <TableCell className="text-right tabular">{x.transactions}</TableCell>}
      <TableCell className="text-right tabular">{formatQty(x.qty_milli)}</TableCell>
      <TableCell className="text-right text-muted-foreground tabular">{x.discount_minor ? fmt(x.discount_minor) : "–"}</TableCell>
      <TableCell className="text-right text-muted-foreground tabular">{x.returns_minor ? fmt(x.returns_minor) : "–"}</TableCell>
      <TableCell className="text-right font-medium tabular">{fmt(x.net_minor)}</TableCell>
      <TableCell className="text-right text-muted-foreground tabular">{fmt(x.cost_minor)}</TableCell>
      <TableCell className={cn("text-right tabular", x.profit_minor < 0 && "text-red-600")}>{fmt(x.profit_minor)}</TableCell>
      <TableCell className="pr-4 text-right text-muted-foreground tabular">{pct(x.profit_minor, x.net_minor)}</TableCell>
    </TableRow>
  )
}

const emptyRow = (key: string, label: string): SalesReportRow => ({
  key, label, sub: "", transactions: 0, qty_milli: 0, gross_minor: 0, discount_minor: 0, tax_minor: 0, returns_minor: 0, net_minor: 0, cost_minor: 0, profit_minor: 0,
})

/** Time on the x-axis must be continuous: add empty days / hours between the first and last. */
function fillGaps(report: SalesReport): SalesReportRow[] {
  const rows = report.rows
  if (rows.length < 2 || report.group === "weekday") return rows
  const byKey = new Map(rows.map((r) => [r.key, r]))
  const out: SalesReportRow[] = []
  if (report.group === "hour") {
    for (let h = Number(rows[0].key); h <= Number(rows[rows.length - 1].key); h++) {
      const k = String(h).padStart(2, "0")
      out.push(byKey.get(k) ?? emptyRow(k, `${k}:00`))
    }
    return out
  }
  const d = new Date(rows[0].key + "T12:00")
  const last = new Date(rows[rows.length - 1].key + "T12:00")
  for (; d <= last && out.length < 400; d.setDate(d.getDate() + 1)) {
    const k = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`
    out.push(byKey.get(k) ?? emptyRow(k, k))
  }
  return out
}

/** Single-series column chart of net sales over time (day / hour / weekday). */
function NetSalesChart({ report }: { report: SalesReport }) {
  const { fmt } = useMoney()
  const [hover, setHover] = useState<number | null>(null)
  const rows = fillGaps(report)
  const max = Math.max(1, ...rows.map((r) => r.net_minor))
  const ticks = [1, 0.5, 0]
  const label = (r: SalesReportRow) =>
    report.group === "day" ? new Date(r.key + "T12:00").toLocaleDateString(undefined, { day: "numeric", month: "short" }) : r.label
  const every = Math.ceil(rows.length / 14) // thin out axis labels
  return (
    <div className="rounded-xl border bg-card p-4">
      <div className="mb-3 text-sm font-semibold">Net sales</div>
      <div className="relative flex h-48 gap-2 pl-16">
        {/* recessive gridlines + axis labels */}
        {ticks.map((f) => (
          <div key={f} className="pointer-events-none absolute inset-x-0 flex items-center" style={{ bottom: `${f * 100}%`, transform: "translateY(50%)" }}>
            <span className="w-14 pr-2 text-right text-[11px] text-muted-foreground tabular">{fmt(Math.round(max * f))}</span>
            <div className="h-px flex-1 bg-border/60" />
          </div>
        ))}
        <div className="relative z-10 flex flex-1 items-end gap-0.5">
          {rows.map((r, i) => (
            <div
              key={r.key}
              className="relative flex h-full flex-1 items-end justify-center"
              onMouseEnter={() => setHover(i)}
              onMouseLeave={() => setHover(null)}
            >
              <div
                className={cn("w-full max-w-6 rounded-t-[4px] bg-primary transition-opacity", hover !== null && hover !== i && "opacity-50")}
                style={{ height: `${Math.max(0, (r.net_minor / max) * 100)}%`, minHeight: r.net_minor > 0 ? 2 : 0 }}
              />
              {hover === i && (
                <div className="pointer-events-none absolute bottom-full z-20 mb-2 min-w-36 rounded-lg border bg-popover px-3 py-2 text-xs whitespace-nowrap shadow-md">
                  <div className="font-medium">{label(r)}</div>
                  <div className="mt-1 flex justify-between gap-4">
                    <span className="text-muted-foreground">Net sales</span>
                    <span className="font-semibold tabular">{fmt(r.net_minor)}</span>
                  </div>
                  <div className="flex justify-between gap-4">
                    <span className="text-muted-foreground">Profit</span>
                    <span className="tabular">{fmt(r.profit_minor)}</span>
                  </div>
                  <div className="flex justify-between gap-4">
                    <span className="text-muted-foreground">Sales</span>
                    <span className="tabular">{r.transactions}</span>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
      <div className="mt-1.5 flex gap-0.5 pl-16">
        {rows.map((r, i) => (
          <div key={r.key} className="flex-1 truncate text-center text-[11px] text-muted-foreground">
            {i % every === 0 ? label(r) : ""}
          </div>
        ))}
      </div>
    </div>
  )
}

function ShiftsTab({ from, to }: { from: number; to: number }) {
  const { fmt } = useMoney()
  const list = useQuery({ queryKey: ["bo", "shifts", from, to], queryFn: () => hub.shiftReport(from, to) })
  const closed = (list.data ?? []).filter((s) => s.status === "closed")
  const short = closed.reduce((s, x) => s + Math.min(0, x.variance_minor), 0)
  const over = closed.reduce((s, x) => s + Math.max(0, x.variance_minor), 0)
  return (
    <>
      <QueryError error={list.error} />
      <div className="grid grid-cols-4 gap-3">
        <StatCard label="Shifts" value={list.data?.length ?? 0} sub={`${closed.length} closed`} />
        <StatCard label="Cash short" value={fmt(short)} className={cn(short < 0 && "border-red-500/40")} />
        <StatCard label="Cash over" value={fmt(over)} />
        <StatCard label="Takings (closed shifts)" value={fmt(closed.reduce((s, x) => s + (x.summary ? x.summary.net_sales_minor - x.summary.returns_minor : 0), 0))} />
      </div>
      {list.data?.length === 0 ? (
        <EmptyState icon={<Wallet />} title="No shifts in this period" />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">Till</TableHead>
                <TableHead>Opened</TableHead>
                <TableHead>Closed</TableHead>
                <TableHead className="text-right">Takings</TableHead>
                <TableHead className="text-right">Expected cash</TableHead>
                <TableHead className="text-right">Counted</TableHead>
                <TableHead className="pr-4 text-right">Variance</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(list.data ?? []).map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="pl-4 font-medium">{s.terminal_name}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatDateTime(s.opened_at)}
                    <div className="text-xs">{s.opened_by_name}</div>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {s.status === "open" ? (
                      <Badge variant="outline" className="border-sky-500/40 text-sky-700 dark:text-sky-400">
                        Open
                      </Badge>
                    ) : (
                      <>
                        {formatDateTime(s.closed_at)}
                        <div className="text-xs">{s.closed_by_name}</div>
                      </>
                    )}
                  </TableCell>
                  <TableCell className="text-right tabular">{s.summary ? fmt(s.summary.net_sales_minor - s.summary.returns_minor) : "–"}</TableCell>
                  <TableCell className="text-right tabular">{s.status === "closed" ? fmt(s.expected_cash_minor) : "–"}</TableCell>
                  <TableCell className="text-right tabular">{s.status === "closed" ? fmt(s.counted_cash_minor) : "–"}</TableCell>
                  <TableCell className={cn("pr-4 text-right font-semibold tabular", s.variance_minor < 0 ? "text-red-600" : s.variance_minor > 0 && "text-amber-700 dark:text-amber-400")}>
                    {s.status === "closed" ? (s.variance_minor > 0 ? "+" : "") + fmt(s.variance_minor) : "–"}
                    {s.note && <div className="text-xs font-normal text-muted-foreground">{s.note}</div>}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </>
  )
}

const actionLabels: Record<string, string> = {
  "sale.totals_mismatch": "Sale totals didn't add up",
  "sale.unknown_variant": "Sale of an unknown item",
  "sale.credit_without_customer": "Credit sale without a customer",
  "return.over_returned": "More returned than sold",
  "shift.cash_variance": "Cash variance at close",
  "stock.adjustment": "Stock adjusted",
  "stock.count": "Stock counted",
  "stock.count_posted": "Stock count posted",
  "stock.transfer": "Stock transferred",
  "goods.received": "Goods received",
  "po.save": "Purchase order saved",
  "po.status": "Purchase order status changed",
  "product.create": "Product created",
  "product.update": "Product updated",
  "user.create": "Staff account created",
  "user.update": "Staff account changed",
  "settings.update": "Store settings changed",
  "customer.balance_adjust": "Customer balance adjusted",
  "catalogue.import": "Catalogue imported",
}

/** Turns an audit entry's JSON details into "key: value · …" with money and quantities formatted. */
function describe(details: string, fmt: (minor: number) => string): string {
  let d: Record<string, unknown>
  try {
    d = JSON.parse(details)
  } catch {
    return details
  }
  if (!d || typeof d !== "object") return ""
  const money = /amount|total|variance|expected|counted|value|subtotal|paid|sum/
  return Object.entries(d)
    .filter(([, v]) => v !== "" && v !== null && typeof v !== "object")
    .map(([k, v]) => {
      const label = k.replace(/_milli$/, "").replace(/_/g, " ")
      if (typeof v === "number" && k.endsWith("_milli")) return `${label}: ${formatQty(v)}`
      if (typeof v === "number" && money.test(k)) return `${label}: ${fmt(v)}`
      return `${label}: ${v}`
    })
    .join(" · ")
}

function ActivityTab({ from, to }: { from: number; to: number }) {
  const { fmt } = useMoney()
  const [q, setQ] = useState("")
  const log = useQuery({ queryKey: ["bo", "audit", from, to], queryFn: () => hub.audit(from, to) })
  const needle = q.trim().toLowerCase()
  const rows = (log.data ?? []).filter(
    (e) => !needle || [e.action, actionLabels[e.action] ?? "", e.user_name, e.terminal_name, e.details].some((s) => s.toLowerCase().includes(needle)),
  )
  const alert = (a: string) => /mismatch|unknown|over_returned|variance|without_customer/.test(a)
  return (
    <>
      <QueryError error={log.error} />
      <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Filter: person, till, action…" className="h-10 max-w-sm" />
      {rows.length === 0 ? (
        <EmptyState icon={<ScrollText />} title="Nothing recorded in this period" />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">When</TableHead>
                <TableHead>What</TableHead>
                <TableHead>Who</TableHead>
                <TableHead className="pr-4">Details</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((e) => (
                <TableRow key={e.id}>
                  <TableCell className="pl-4 whitespace-nowrap text-muted-foreground">{formatDateTime(e.at)}</TableCell>
                  <TableCell className={cn("font-medium", alert(e.action) && "text-amber-700 dark:text-amber-400")}>{actionLabels[e.action] ?? e.action}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {e.user_name}
                    {e.terminal_name && <div className="text-xs">{e.terminal_name}</div>}
                  </TableCell>
                  <TableCell className="max-w-md truncate pr-4 text-xs text-muted-foreground" title={e.details}>
                    {describe(e.details, fmt)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </>
  )
}
