import { useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { CloudOff, Download, Loader2, Printer, Search } from "lucide-react"
import { toast } from "sonner"
import { EmailButton } from "@/components/email-document"
import { EmptyState } from "@/components/page"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { api, errorMessage } from "@/lib/api"
import { can, formatQty } from "@/lib/format"
import { hub } from "@/lib/hub"
import { useLoadedState, useMoney } from "@/lib/state"
import type { DocumentKind, StockReportRow } from "@/lib/types"
import { cn } from "@/lib/utils"

/** Back-office screens work on the hub's records, so they need it online. */
export function HubRequired({ children }: { children: ReactNode }) {
  const state = useLoadedState()
  if (state.has_session) return <>{children}</>
  return (
    <div className="px-8 pb-8">
      <EmptyState
        icon={<CloudOff />}
        title="The back office needs the hub"
        body={
          state.status.online
            ? "You signed in while the hub was unreachable. Sign out and in again to open the back office."
            : "This till can't reach the hub right now. Selling continues offline; purchasing, stock and reports return when the connection does."
        }
      />
    </div>
  )
}

/** Shows a back-office query's error in place. */
export function QueryError({ error }: { error: unknown }) {
  if (!error) return null
  return <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">{errorMessage(error)}</div>
}

export function useLocations() {
  return useQuery({ queryKey: ["bo", "locations"], queryFn: hub.locations, staleTime: 30_000 })
}

/** Location chooser; hidden when the store only has one location. */
export function LocationSelect({ value, onChange, className, label = true, includeInactive = false }: {
  value: string
  onChange: (id: string) => void
  className?: string
  label?: boolean
  includeInactive?: boolean
}) {
  const locs = useLocations()
  const list = (locs.data ?? []).filter((l) => includeInactive || l.active)
  if (list.length <= 1) return null
  return (
    <div className={cn("space-y-1.5", className)}>
      {label && <div className="text-sm font-medium">Location</div>}
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger className="h-10 w-full">
          <SelectValue placeholder="Choose a location" />
        </SelectTrigger>
        <SelectContent>
          {list.map((l) => (
            <SelectItem key={l.id} value={l.id}>
              {l.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

export function itemLabel(r: { product_name: string; variant_name: string }) {
  return r.variant_name ? `${r.product_name} · ${r.variant_name}` : r.product_name
}

/**
 * Search the hub catalogue (name, SKU, barcode, author…) with stock at a
 * location. Enter with a barcode that matches exactly one item picks it,
 * so a scanner works.
 */
export function ItemPicker({ location, onPick, placeholder = "Scan a barcode or search items…", autoFocus, exclude }: {
  location: string
  onPick: (row: StockReportRow) => void
  placeholder?: string
  autoFocus?: boolean
  exclude?: Set<string>
}) {
  const { fmt } = useMoney()
  const [q, setQ] = useState("")
  const [debounced, setDebounced] = useState("")
  const [active, setActive] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 180)
    return () => clearTimeout(t)
  }, [q])
  const results = useQuery({
    queryKey: ["bo", "pick", location, debounced],
    queryFn: () => hub.stock({ location, q: debounced, limit: 30 }),
    enabled: debounced.length > 0,
  })
  const rows = useMemo(() => (results.data?.rows ?? []).filter((r) => !exclude?.has(r.variant_id)), [results.data, exclude])
  useEffect(() => setActive(0), [debounced])

  function pick(r: StockReportRow) {
    onPick(r)
    setQ("")
    setDebounced("")
    inputRef.current?.focus()
  }

  async function onKey(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown") {
      e.preventDefault()
      setActive((a) => Math.min(a + 1, rows.length - 1))
    } else if (e.key === "ArrowUp") {
      e.preventDefault()
      setActive((a) => Math.max(a - 1, 0))
    } else if (e.key === "Enter") {
      e.preventDefault()
      const term = q.trim()
      if (!term) return
      if (debounced === term && rows[active]) return pick(rows[active])
      // Scanner: Enter arrives before the debounce; look it up now.
      try {
        const r = await hub.stock({ location, q: term, limit: 2 })
        if (r.rows.length === 1) pick(r.rows[0])
        else if (r.rows.length === 0) toast.error(`Nothing matches “${term}”`)
        else setDebounced(term)
      } catch (err) {
        toast.error(errorMessage(err))
      }
    } else if (e.key === "Escape") {
      setQ("")
    }
  }

  const open = debounced.length > 0 && q.trim().length > 0
  return (
    <div className="relative">
      <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input ref={inputRef} autoFocus={autoFocus} value={q} onChange={(e) => setQ(e.target.value)} onKeyDown={onKey} placeholder={placeholder} className="h-10 pl-9" />
      {results.isFetching && <Loader2 className="absolute top-1/2 right-3 size-4 -translate-y-1/2 animate-spin text-muted-foreground" />}
      {open && (
        <div className="absolute inset-x-0 top-full z-50 mt-1 max-h-80 overflow-y-auto rounded-lg border bg-popover p-1 shadow-lg">
          {rows.length === 0 && !results.isFetching && <div className="px-3 py-2 text-sm text-muted-foreground">No matching items</div>}
          {rows.map((r, i) => (
            <button
              key={r.variant_id}
              type="button"
              onMouseEnter={() => setActive(i)}
              onClick={() => pick(r)}
              className={cn("flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm", i === active && "bg-accent text-accent-foreground")}
            >
              <div className="min-w-0 flex-1">
                <div className="truncate font-medium">{itemLabel(r)}</div>
                <div className="truncate text-xs text-muted-foreground">
                  {r.sku}
                  {r.category_name && ` · ${r.category_name}`}
                </div>
              </div>
              <div className="text-right text-xs tabular">
                <div>{formatQty(r.qty_milli)} on hand</div>
                <div className="text-muted-foreground">cost {fmt(r.cost_minor)}</div>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// ---------- periods ----------

export type PeriodKey = "today" | "yesterday" | "week" | "month" | "last_month" | "30d" | "year" | "custom"

export const periodLabels: Record<PeriodKey, string> = {
  today: "Today",
  yesterday: "Yesterday",
  week: "This week",
  month: "This month",
  last_month: "Last month",
  "30d": "Last 30 days",
  year: "This year",
  custom: "Custom…",
}

function dayStart(d: Date) {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate())
}

export function toDateInput(ms: number) {
  const d = new Date(ms)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`
}

function fromDateInput(s: string) {
  const [y, m, d] = s.split("-").map(Number)
  return new Date(y, (m || 1) - 1, d || 1)
}

/** [from, to) in ms for a period. Weeks start on Monday. */
export function periodRange(key: PeriodKey, custom: { from: string; to: string }): [number, number] {
  const today = dayStart(new Date())
  const add = (d: Date, days: number) => new Date(d.getFullYear(), d.getMonth(), d.getDate() + days)
  switch (key) {
    case "today":
      return [today.getTime(), add(today, 1).getTime()]
    case "yesterday":
      return [add(today, -1).getTime(), today.getTime()]
    case "week": {
      const back = (today.getDay() + 6) % 7
      return [add(today, -back).getTime(), add(today, 1).getTime()]
    }
    case "month":
      return [new Date(today.getFullYear(), today.getMonth(), 1).getTime(), add(today, 1).getTime()]
    case "last_month":
      return [new Date(today.getFullYear(), today.getMonth() - 1, 1).getTime(), new Date(today.getFullYear(), today.getMonth(), 1).getTime()]
    case "30d":
      return [add(today, -29).getTime(), add(today, 1).getTime()]
    case "year":
      return [new Date(today.getFullYear(), 0, 1).getTime(), add(today, 1).getTime()]
    case "custom": {
      const f = fromDateInput(custom.from)
      const t = fromDateInput(custom.to)
      return [f.getTime(), add(t, 1).getTime()]
    }
  }
}

export function usePeriod(initial: PeriodKey = "today") {
  const [key, setKey] = useState<PeriodKey>(initial)
  const [custom, setCustom] = useState(() => ({ from: toDateInput(Date.now() - 6 * 86_400_000), to: toDateInput(Date.now()) }))
  const [from, to] = periodRange(key, custom)
  return { key, setKey, custom, setCustom, from, to }
}

export function PeriodPicker({ period }: { period: ReturnType<typeof usePeriod> }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Select value={period.key} onValueChange={(v) => period.setKey(v as PeriodKey)}>
        <SelectTrigger className="h-10 w-40">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {(Object.keys(periodLabels) as PeriodKey[]).map((k) => (
            <SelectItem key={k} value={k}>
              {periodLabels[k]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {period.key === "custom" && (
        <>
          <Input type="date" value={period.custom.from} max={period.custom.to} onChange={(e) => e.target.value && period.setCustom((c) => ({ ...c, from: e.target.value }))} className="h-10 w-40" />
          <span className="text-sm text-muted-foreground">to</span>
          <Input type="date" value={period.custom.to} min={period.custom.from} onChange={(e) => e.target.value && period.setCustom((c) => ({ ...c, to: e.target.value }))} className="h-10 w-40" />
        </>
      )}
    </div>
  )
}

/** Saves CSV via the native save dialog. */
export function ExportButton({ name, build, label = "Export CSV", disabled }: { name: string; build: () => string | Promise<string>; label?: string; disabled?: boolean }) {
  const [busy, setBusy] = useState(false)
  async function go() {
    setBusy(true)
    try {
      const path = await api.saveTextFile(name, await build())
      if (path) toast.success("Saved " + path)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Button variant="outline" className="h-10" onClick={go} disabled={busy || disabled}>
      {busy ? <Loader2 className="animate-spin" /> : <Download />} {label}
    </Button>
  )
}

/** A segmented filter (chips) used above tables. */
export function Chips<T extends string>({ value, onChange, options }: { value: T; onChange: (v: T) => void; options: { value: T; label: ReactNode }[] }) {
  return (
    <div className="flex w-fit rounded-lg border bg-card p-0.5">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={value === o.value}
          onClick={() => onChange(o.value)}
          className={cn("rounded-md px-3 py-1.5 text-sm font-medium transition-colors", value === o.value ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground")}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

/** Opens an A4 document in the browser to print or save as PDF. */
export async function printDocument(kind: DocumentKind, id: string) {
  try {
    await api.printDocument(kind, id)
    toast.success("Opened in your browser", { description: "Print it, or choose “Save as PDF” to email it." })
  } catch (e) {
    toast.error(errorMessage(e))
  }
}

export function PrintButton({ kind, id, label = "Print", variant = "outline" }: { kind: DocumentKind; id: string; label?: string; variant?: "outline" | "ghost" }) {
  const [busy, setBusy] = useState(false)
  const state = useLoadedState()
  const print = (
    <Button
      variant={variant}
      disabled={busy}
      onClick={async () => {
        setBusy(true)
        await printDocument(kind, id)
        setBusy(false)
      }}
    >
      {busy ? <Loader2 className="animate-spin" /> : <Printer />} {label}
    </Button>
  )
  if (!can(state.user?.role, "admin", "manager")) return print
  return (
    <>
      {print}
      <EmailButton kind={kind} id={id} variant={variant} />
    </>
  )
}
