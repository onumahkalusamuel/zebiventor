// Small, dependency-free SVG/HTML charts following the dataviz method:
// thin marks, recessive axes, text in text colours (never the series
// colour), a hover/focus readout on every chart, and a table view so no
// value is only reachable by hovering. Colours come from the --viz-* roles.
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { BarChart3, Table2 } from "lucide-react"
import { cn } from "@/lib/utils"
import { compact, series } from "@/lib/viz"


function useWidth<T extends HTMLElement>() {
  const ref = useRef<T>(null)
  const [w, setW] = useState(0)
  useEffect(() => {
    if (!ref.current) return
    const ro = new ResizeObserver((e) => setW(Math.floor(e[0].contentRect.width)))
    ro.observe(ref.current)
    return () => ro.disconnect()
  }, [])
  return [ref, w] as const
}

/** Clean axis ticks from 0 to just above max. */
function ticks(max: number, count = 4): number[] {
  if (max <= 0) return [0]
  const raw = max / count
  const mag = 10 ** Math.floor(Math.log10(raw))
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw) ?? raw
  const out = []
  for (let v = 0; v <= max + step * 0.001; v += step) out.push(v)
  if (out[out.length - 1] < max) out.push(out[out.length - 1] + step)
  return out
}

// ---------- card with chart/table toggle ----------

export function ChartCard({
  title,
  sub,
  table,
  className,
  children,
  loading,
}: {
  title: string
  sub?: string
  table: { columns: string[]; rows: (string | number)[][]; right?: number[] }
  className?: string
  children: ReactNode
  loading?: boolean
}) {
  const [asTable, setAsTable] = useState(false)
  return (
    <section className={cn("flex flex-col rounded-xl border bg-[var(--viz-surface)] p-4", className)} aria-busy={loading}>
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-[var(--viz-ink)]">{title}</h3>
          {sub && <p className="text-xs text-[var(--viz-ink-2)]">{sub}</p>}
        </div>
        <button
          onClick={() => setAsTable(!asTable)}
          className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-[var(--viz-ink-2)] hover:bg-muted"
          aria-pressed={asTable}
          aria-label={asTable ? `Show ${title} as a chart` : `Show ${title} as a table`}
        >
          {asTable ? <BarChart3 className="size-3.5" /> : <Table2 className="size-3.5" />}
          {asTable ? "Chart" : "Table"}
        </button>
      </div>
      <div className={cn("min-h-0 flex-1 transition-opacity", loading && "opacity-50")}>
        {asTable ? (
          <div className="max-h-80 overflow-auto">
            <table className="w-full text-xs">
              <thead className="sticky top-0 bg-[var(--viz-surface)]">
                <tr>
                  {table.columns.map((c, i) => (
                    <th key={c} className={cn("border-b py-1.5 pr-3 font-medium text-[var(--viz-ink-2)]", table.right?.includes(i) ? "text-right" : "text-left")}>
                      {c}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {table.rows.map((r, j) => (
                  <tr key={j} className="border-b border-[var(--viz-grid)]">
                    {r.map((v, i) => (
                      <td key={i} className={cn("py-1 pr-3 text-[var(--viz-ink)]", table.right?.includes(i) && "text-right tabular")}>
                        {v}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          children
        )}
      </div>
    </section>
  )
}

function Tooltip({ x, y, children, width }: { x: number; y: number; children: ReactNode; width: number }) {
  const left = Math.min(Math.max(x + 12, 0), Math.max(0, width - 190))
  return (
    <div
      role="status"
      className="pointer-events-none absolute z-10 w-max max-w-[220px] rounded-lg border bg-popover px-3 py-2 text-xs shadow-md"
      style={{ left, top: Math.max(0, y - 10) }}
    >
      {children}
    </div>
  )
}

export function Legend({ items }: { items: { name: string; color: string; kind?: "line" | "box" }[] }) {
  return (
    <ul className="mb-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--viz-ink-2)]">
      {items.map((it) => (
        <li key={it.name} className="flex items-center gap-1.5">
          {it.kind === "box" ? (
            <span className="size-2.5 rounded-[3px]" style={{ background: it.color }} />
          ) : (
            <span className="h-0.5 w-3.5 rounded" style={{ background: it.color }} />
          )}
          {it.name}
        </li>
      ))}
    </ul>
  )
}

// ---------- time series (lines, first series with an area wash) ----------

export interface TimePoint {
  key: string
  label: string // full label for the readout ("Mar 2025", "12 Mar 2025")
  tick: string // short axis label
  values: number[]
}

export function TimeSeries({ points, names, format, height = 240 }: { points: TimePoint[]; names: string[]; format: (v: number) => string; height?: number }) {
  const [ref, width] = useWidth<HTMLDivElement>()
  const [hover, setHover] = useState<number | null>(null)
  const pad = { l: 52, r: 12, t: 8, b: 24 }
  const w = Math.max(0, width - pad.l - pad.r)
  const h = height - pad.t - pad.b
  const max = Math.max(0, ...points.flatMap((p) => p.values))
  const yt = ticks(max)
  const top = yt[yt.length - 1] || 1
  const x = (i: number) => pad.l + (points.length <= 1 ? w / 2 : (i * w) / (points.length - 1))
  const y = (v: number) => pad.t + h - (Math.max(0, v) / top) * h
  const every = Math.max(1, Math.ceil(points.length / Math.max(2, Math.floor(w / 70))))
  const path = (s: number) => points.map((p, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(p.values[s]).toFixed(1)}`).join("")
  const area = points.length ? `${path(0)}L${x(points.length - 1)},${y(0)}L${x(0)},${y(0)}Z` : ""
  const pick = (clientX: number, el: Element) => {
    const r = el.getBoundingClientRect()
    const rel = clientX - r.left - pad.l
    setHover(Math.min(points.length - 1, Math.max(0, Math.round((rel / Math.max(1, w)) * (points.length - 1)))))
  }
  return (
    <div ref={ref} className="relative">
      {names.length > 1 && <Legend items={names.map((n, i) => ({ name: n, color: series[i] }))} />}
      {width > 0 && (
        <svg
          width={width}
          height={height}
          role="img"
          aria-label={`${names.join(" and ")} over time. Use left and right arrows to read values.`}
          tabIndex={0}
          className="block focus-visible:outline-2 focus-visible:outline-ring"
          onPointerMove={(e) => pick(e.clientX, e.currentTarget)}
          onPointerLeave={() => setHover(null)}
          onKeyDown={(e) => {
            if (!points.length) return
            if (e.key === "ArrowRight") setHover((h) => Math.min(points.length - 1, (h ?? -1) + 1))
            if (e.key === "ArrowLeft") setHover((h) => Math.max(0, (h ?? points.length) - 1))
            if (e.key === "Escape") setHover(null)
          }}
          onBlur={() => setHover(null)}
        >
          {yt.map((t) => (
            <g key={t}>
              <line x1={pad.l} x2={pad.l + w} y1={y(t)} y2={y(t)} stroke={t === 0 ? "var(--viz-axis)" : "var(--viz-grid)"} strokeWidth={1} />
              <text x={pad.l - 6} y={y(t)} dy="0.32em" textAnchor="end" className="fill-[var(--viz-muted)] text-[10px] tabular">
                {compact(t / 100)}
              </text>
            </g>
          ))}
          {points.map((p, i) =>
            i % every === 0 ? (
              <text key={p.key} x={x(i)} y={height - 6} textAnchor="middle" className="fill-[var(--viz-muted)] text-[10px]">
                {p.tick}
              </text>
            ) : null,
          )}
          <path d={area} fill={series[0]} opacity={0.1} />
          {names.map((_, s) => (
            <path key={s} d={path(s)} fill="none" stroke={series[s]} strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" />
          ))}
          {hover !== null && points[hover] && (
            <g>
              <line x1={x(hover)} x2={x(hover)} y1={pad.t} y2={pad.t + h} stroke="var(--viz-axis)" strokeWidth={1} />
              {names.map((_, s) => (
                <circle key={s} cx={x(hover)} cy={y(points[hover].values[s])} r={4} fill={series[s]} stroke="var(--viz-surface)" strokeWidth={2} />
              ))}
            </g>
          )}
        </svg>
      )}
      {hover !== null && points[hover] && (
        <Tooltip x={x(hover)} y={pad.t} width={width}>
          <div className="mb-1 text-[var(--viz-ink-2)]">{points[hover].label}</div>
          {names.map((n, s) => (
            <div key={n} className="flex items-center gap-2">
              <span className="h-0.5 w-3 rounded" style={{ background: series[s] }} />
              <span className="font-semibold text-[var(--viz-ink)] tabular">{format(points[hover].values[s])}</span>
              <span className="text-[var(--viz-ink-2)]">{n}</span>
            </div>
          ))}
        </Tooltip>
      )}
    </div>
  )
}

// ---------- horizontal bars (single series, value at the tip) ----------

export function BarList({
  rows,
  format,
  color = series[0],
  total,
}: {
  rows: { label: string; sub?: string; value: number }[]
  format: (v: number) => string
  color?: string
  total?: number
}) {
  const max = Math.max(1, ...rows.map((r) => r.value))
  const [hover, setHover] = useState<number | null>(null)
  return (
    <ul className="space-y-1.5">
      {rows.map((r, i) => {
        const pct = Math.max(0, r.value) / max
        return (
          <li
            key={r.label + i}
            tabIndex={0}
            onPointerEnter={() => setHover(i)}
            onPointerLeave={() => setHover(null)}
            onFocus={() => setHover(i)}
            onBlur={() => setHover(null)}
            aria-label={`${r.label}: ${format(r.value)}${total ? `, ${((r.value / total) * 100).toFixed(1)}%` : ""}`}
            className="grid grid-cols-[minmax(0,38%)_1fr] items-center gap-3 rounded-md px-1 py-0.5 text-xs outline-none hover:bg-muted/50 focus-visible:bg-muted/60"
          >
            <div className="min-w-0">
              <div className="truncate text-[var(--viz-ink)]" title={r.label}>
                {r.label}
              </div>
              {r.sub && <div className="truncate text-[var(--viz-muted)]">{r.sub}</div>}
            </div>
            <div className="flex items-center gap-2">
              <div
                className="h-3.5 rounded-r-[4px] transition-[filter]"
                style={{ width: `${Math.max(pct * 72, r.value > 0 ? 0.5 : 0)}%`, background: color, filter: hover === i ? "brightness(1.12)" : undefined }}
              />
              <span className="shrink-0 font-medium text-[var(--viz-ink)] tabular">{format(r.value)}</span>
              {total ? <span className="shrink-0 text-[var(--viz-muted)] tabular">{((r.value / total) * 100).toFixed(0)}%</span> : null}
            </div>
          </li>
        )
      })}
    </ul>
  )
}

// ---------- heatmap (weekday × hour, one sequential hue) ----------

const days = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]
const steps = [0, 1, 2, 3, 4, 5, 6].map((i) => `var(--viz-seq-${i})`)

export function Heatmap({ cells, format }: { cells: Map<string, number>; format: (v: number) => string }) {
  const hours = useMemo(() => {
    const hs = [...cells.keys()].map((k) => Number(k.split("-")[1]))
    if (!hs.length) return []
    const lo = Math.min(...hs)
    const hi = Math.max(...hs)
    return Array.from({ length: hi - lo + 1 }, (_, i) => lo + i)
  }, [cells])
  const max = Math.max(1, ...cells.values())
  const [hover, setHover] = useState<string | null>(null)
  const order = [1, 2, 3, 4, 5, 6, 0] // week starts Monday
  return (
    <div className="relative overflow-x-auto">
      <table className="w-full border-separate border-spacing-[2px] text-[10px]">
        <thead>
          <tr>
            <th />
            {hours.map((h) => (
              <th key={h} className="font-normal text-[var(--viz-muted)]">
                {h % 2 === 0 ? String(h).padStart(2, "0") : ""}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {order.map((d) => (
            <tr key={d}>
              <th className="pr-1 text-right font-normal text-[var(--viz-muted)]">{days[d]}</th>
              {hours.map((h) => {
                const k = `${d}-${String(h).padStart(2, "0")}`
                const v = cells.get(k) ?? 0
                const step = v <= 0 ? -1 : Math.min(6, Math.floor((v / max) * 7))
                return (
                  <td
                    key={k}
                    tabIndex={0}
                    aria-label={`${days[d]} ${String(h).padStart(2, "0")}:00, ${format(v)}`}
                    onPointerEnter={() => setHover(k)}
                    onPointerLeave={() => setHover(null)}
                    onFocus={() => setHover(k)}
                    onBlur={() => setHover(null)}
                    className="h-5 min-w-4 rounded-[3px] outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    style={{ background: step < 0 ? "var(--viz-grid)" : steps[step], outline: hover === k ? "2px solid var(--viz-ink)" : undefined }}
                  />
                )
              })}
            </tr>
          ))}
        </tbody>
      </table>
      <div className="mt-2 flex items-center gap-2 text-[10px] text-[var(--viz-muted)]">
        Less
        {steps.map((c) => (
          <span key={c} className="h-2.5 w-4 rounded-[2px]" style={{ background: c }} />
        ))}
        More
        {hover && (
          <span className="ml-auto text-xs text-[var(--viz-ink)]">
            <b className="tabular">{format(cells.get(hover) ?? 0)}</b>{" "}
            <span className="text-[var(--viz-ink-2)]">
              {days[Number(hover.split("-")[0])]} {hover.split("-")[1]}:00
            </span>
          </span>
        )}
      </div>
    </div>
  )
}

// ---------- tall segmented bar (one whole, split into parts) ----------

export function SegmentedBar({ parts, format }: { parts: { label: string; value: number }[]; format: (v: number) => string }) {
  const [ref, width] = useWidth<HTMLDivElement>()
  const shown = parts.filter((p) => p.value > 0)
  const total = shown.reduce((s, p) => s + p.value, 0) || 1
  const [hover, setHover] = useState<number | null>(null)
  return (
    <div ref={ref}>
      <div className="flex h-7 gap-0.5" role="img" aria-label={shown.map((p) => `${p.label} ${format(p.value)} (${((p.value / total) * 100).toFixed(0)}%)`).join(", ")}>
        {shown.map((p, i) => {
          const share = p.value / total
          const fits = share * width > 44
          return (
            <div
              key={p.label}
              tabIndex={0}
              aria-label={`${p.label}: ${format(p.value)}, ${(share * 100).toFixed(1)}%`}
              onPointerEnter={() => setHover(i)}
              onPointerLeave={() => setHover(null)}
              onFocus={() => setHover(i)}
              onBlur={() => setHover(null)}
              className="flex items-center rounded-[4px] px-2 text-[11px] font-semibold outline-none focus-visible:ring-2 focus-visible:ring-ring"
              style={{ width: `${share * 100}%`, background: series[i % series.length], color: i % series.length === 3 ? "#0b0b0b" : "#ffffff", filter: hover === i ? "brightness(1.12)" : undefined }}
            >
              {fits ? `${(share * 100).toFixed(0)}%` : ""}
            </div>
          )
        })}
      </div>
      <Legend items={shown.map((p, i) => ({ name: p.label, color: series[i % series.length], kind: "box" as const }))} />
      <div className="h-4 text-xs">
        {hover !== null && shown[hover] && (
          <span>
            <b className="text-[var(--viz-ink)] tabular">{format(shown[hover].value)}</b>{" "}
            <span className="text-[var(--viz-ink-2)]">
              {shown[hover].label} · {((shown[hover].value / total) * 100).toFixed(1)}%
            </span>
          </span>
        )}
      </div>
    </div>
  )
}

// ---------- stat tile ----------

export function StatTile({ label, value, delta, hint }: { label: string; value: string; delta?: number | null; hint?: string }) {
  const up = (delta ?? 0) >= 0
  return (
    <div className="rounded-xl border bg-[var(--viz-surface)] p-4">
      <div className="text-xs text-[var(--viz-ink-2)]">{label}</div>
      <div className="mt-1 text-2xl font-semibold text-[var(--viz-ink)]">{value}</div>
      <div className="mt-1 flex items-center gap-1.5 text-xs">
        {delta !== undefined && delta !== null && isFinite(delta) && (
          <span className="font-medium" style={{ color: up ? "var(--viz-up)" : "var(--viz-down)" }}>
            {up ? "▲" : "▼"} {Math.abs(delta * 100).toFixed(1)}%
          </span>
        )}
        {hint && <span className="text-[var(--viz-muted)]">{hint}</span>}
      </div>
    </div>
  )
}
