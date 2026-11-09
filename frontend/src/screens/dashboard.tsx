// Dashboard: how the store is doing over a period, at a glance.
import { useQuery } from "@tanstack/react-query"
import { HubRequired, PeriodPicker, QueryError, usePeriod } from "@/components/backoffice"
import { BarList, ChartCard, Heatmap, SegmentedBar, StatTile, TimeSeries, type TimePoint } from "@/components/charts"
import { series } from "@/lib/viz"
import { Page, PageHeader } from "@/components/page"
import { hub } from "@/lib/hub"
import { useMoney } from "@/lib/state"
import type { SalesReport } from "@/lib/types"

const monthNames = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]

function timePoints(rep: SalesReport | undefined, byMonth: boolean): TimePoint[] {
  const now = new Date()
  const thisMonth = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`
  return (rep?.rows ?? []).map((r) => {
    const [y, m, d] = r.key.split("-").map(Number)
    // The current month isn't over: say so, or its dip looks like a slump.
    const label = byMonth ? `${monthNames[m - 1]} ${y}${r.key === thisMonth ? " (so far)" : ""}` : `${d} ${monthNames[m - 1]} ${y}`
    const tick = byMonth ? (r.key === thisMonth ? `${monthNames[m - 1]}*` : m === 1 ? String(y) : monthNames[m - 1]) : `${d} ${monthNames[m - 1]}`
    return { key: r.key, label, tick, values: [r.net_minor, r.profit_minor] }
  })
}

export function Dashboard() {
  return (
    <Page>
      <HubRequired>
        <DashboardBody />
      </HubRequired>
    </Page>
  )
}

function DashboardBody() {
  const { fmt } = useMoney()
  const period = usePeriod("12m")
  const { from, to } = period
  const span = to - from
  const prevFrom = from - span
  const byMonth = span > 62 * 86_400_000
  const opts = { placeholderData: (p: SalesReport | undefined) => p }
  const trend = useQuery({ queryKey: ["dash", "trend", from, to, byMonth], queryFn: () => hub.salesReport(from, to, byMonth ? "month" : "day"), ...opts })
  const prev = useQuery({ queryKey: ["dash", "prev", prevFrom, from], queryFn: () => hub.salesReport(prevFrom, from, "month"), enabled: period.key !== "all", ...opts })
  const cats = useQuery({ queryKey: ["dash", "cat", from, to], queryFn: () => hub.salesReport(from, to, "category"), ...opts })
  const items = useQuery({ queryKey: ["dash", "item", from, to], queryFn: () => hub.salesReport(from, to, "product"), ...opts })
  const tills = useQuery({ queryKey: ["dash", "till", from, to], queryFn: () => hub.salesReport(from, to, "till"), ...opts })
  const heat = useQuery({ queryKey: ["dash", "heat", from, to], queryFn: () => hub.salesReport(from, to, "weekhour"), ...opts })

  const t = trend.data?.totals
  const p = period.key === "all" ? undefined : prev.data?.totals // nothing to compare all time with
  const change = (a?: number, b?: number) => (a === undefined || !b ? null : (a - b) / Math.abs(b))
  const avg = (x?: { net_minor: number; transactions: number }) => (x && x.transactions ? x.net_minor / x.transactions : 0)
  const points = timePoints(trend.data, byMonth)
  const money = (v: number) => fmt(v)
  const margin = t && t.net_minor ? (t.profit_minor / t.net_minor) * 100 : 0
  const loading = trend.isFetching || cats.isFetching
  const catTotal = (cats.data?.rows ?? []).reduce((s, r) => s + Math.max(0, r.net_minor), 0)
  const tillTotal = (tills.data?.rows ?? []).reduce((s, r) => s + Math.max(0, r.net_minor), 0)
  const payments = (trend.data?.payments ?? []).filter((m) => m.net_minor > 0)

  return (
    <>
      <PageHeader title="Dashboard" description="Sales after returns, gross profit and where they came from. Every chart has a table view." />
      <div className="space-y-4 px-8 pb-8">
        <div className="flex items-center gap-3">
          <PeriodPicker period={period} />
          {period.key !== "all" && <span className="text-xs text-muted-foreground">Changes are against the {byMonth ? "same number of months" : "same number of days"} before.</span>}
        </div>
        <QueryError error={trend.error ?? cats.error} />

        <div className="grid grid-cols-4 gap-4">
          <StatTile label="Sales" value={t ? fmt(t.net_minor) : "–"} delta={change(t?.net_minor, p?.net_minor)} hint={`${(t?.transactions ?? 0).toLocaleString()} sales`} />
          <StatTile label="Gross profit" value={t ? fmt(t.profit_minor) : "–"} delta={change(t?.profit_minor, p?.profit_minor)} hint={`${margin.toFixed(1)}% margin`} />
          <StatTile label="Average sale" value={t ? fmt(Math.round(avg(t))) : "–"} delta={p ? change(avg(t), avg(p)) : null} />
          <StatTile label="Returns" value={t ? fmt(t.returns_minor) : "–"} hint={t && t.gross_minor ? `${((t.returns_minor / t.gross_minor) * 100).toFixed(1)}% of sales` : undefined} />
        </div>

        <ChartCard
          title={`Sales and gross profit, by ${byMonth ? "month" : "day"}`}
          sub={byMonth ? "* this month so far" : undefined}
          loading={loading}
          table={{ columns: [byMonth ? "Month" : "Day", "Sales", "Gross profit", "Sales count"], right: [1, 2, 3], rows: (trend.data?.rows ?? []).map((r, i) => [points[i]?.label ?? r.key, fmt(r.net_minor), fmt(r.profit_minor), r.transactions]) }}
        >
          <TimeSeries points={points} names={["Sales", "Gross profit"]} format={money} height={260} />
        </ChartCard>

        <div className="grid grid-cols-2 gap-4">
          <ChartCard
            title="Sales by category"
            loading={loading}
            table={{ columns: ["Category", "Sales", "Share", "Gross profit"], right: [1, 2, 3], rows: (cats.data?.rows ?? []).map((r) => [r.label, fmt(r.net_minor), catTotal ? `${((r.net_minor / catTotal) * 100).toFixed(1)}%` : "", fmt(r.profit_minor)]) }}
          >
            <BarList rows={(cats.data?.rows ?? []).slice(0, 10).map((r) => ({ label: r.label, value: r.net_minor }))} format={money} total={catTotal} />
          </ChartCard>
          <ChartCard
            title="Best sellers"
            sub="Top 10 by sales value"
            loading={loading}
            table={{
              columns: ["Item", "Copies", "Sales", "Gross profit"],
              right: [1, 2, 3],
              rows: (items.data?.rows ?? []).slice(0, 100).map((r) => [r.label, (r.qty_milli / 1000).toLocaleString(), fmt(r.net_minor), fmt(r.profit_minor)]),
            }}
          >
            <BarList
              rows={(items.data?.rows ?? []).slice(0, 10).map((r) => ({ label: r.label, sub: `${(r.qty_milli / 1000).toLocaleString()} sold`, value: r.net_minor }))}
              format={money}
            />
          </ChartCard>
        </div>

        <div className="grid grid-cols-[1fr_1.4fr] gap-4">
          <ChartCard
            title="Sales points"
            sub="Sales by till"
            loading={loading}
            table={{ columns: ["Till", "Sales", "Share", "Sales count"], right: [1, 2, 3], rows: (tills.data?.rows ?? []).map((r) => [r.label, fmt(r.net_minor), tillTotal ? `${((r.net_minor / tillTotal) * 100).toFixed(1)}%` : "", r.transactions]) }}
          >
            <BarList rows={(tills.data?.rows ?? []).map((r) => ({ label: r.label, sub: `${r.transactions.toLocaleString()} sales`, value: r.net_minor }))} format={money} total={tillTotal} color={series[0]} />
            <div className="mt-5">
              <div className="mb-2 text-xs font-medium text-[var(--viz-ink-2)]">How customers paid</div>
              <SegmentedBar parts={payments.map((m) => ({ label: m.method_name, value: m.net_minor }))} format={money} />
            </div>
          </ChartCard>
          <ChartCard
            title="Busiest times"
            sub="Number of sales by day of the week and hour"
            loading={loading}
            table={{ columns: ["Day and hour", "Sales count", "Sales"], right: [1, 2], rows: (heat.data?.rows ?? []).map((r) => [r.key, r.transactions, fmt(r.net_minor)]) }}
          >
            <Heatmap cells={new Map((heat.data?.rows ?? []).map((r) => [r.key, r.transactions]))} format={(v) => `${v.toLocaleString()} sales`} />
          </ChartCard>
        </div>
      </div>
    </>
  )
}
