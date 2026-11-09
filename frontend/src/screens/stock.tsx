import { useEffect, useMemo, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { AlertTriangle, ArrowRightLeft, ClipboardCheck, History, Loader2, MapPin, PackageCheck, Plus, Search, Trash2, Warehouse, X } from "lucide-react"
import { toast } from "sonner"
import { ArrivingHere, SendStockButton } from "@/components/arriving"
import { Chips, ExportButton, HubRequired, ItemPicker, LocationSelect, PrintButton, QueryError, itemLabel, useLocations } from "@/components/backoffice"
import { EmptyState, Page, PageHeader, StatCard } from "@/components/page"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Switch } from "@/components/ui/switch"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api, errorMessage } from "@/lib/api"
import { formatDateTime, formatQty, parseQty } from "@/lib/format"
import { csvMoney, hub, toCSV } from "@/lib/hub"
import { useLoadedState, useMoney } from "@/lib/state"
import type { LocationInfo, StockCount, StockCountLine, StockReportRow, StockTransfer } from "@/lib/types"
import { cn } from "@/lib/utils"

type Tab = "levels" | "counts" | "transfers" | "locations"

export function Stock() {
  const [tab, setTab] = useState<Tab>("levels")
  return (
    <Page>
      <PageHeader title="Stock" description="What you hold, where, and what it's worth. Counts and transfers update every till within seconds." actions={<SendStockButton />} />
      <ArrivingHere />
      <HubRequired>
        <div className="space-y-4 px-8 pb-8">
          <Chips
            value={tab}
            onChange={setTab}
            options={[
              { value: "levels", label: "Stock levels" },
              { value: "counts", label: "Stock counts" },
              { value: "transfers", label: "Transfers" },
              { value: "locations", label: "Locations" },
            ]}
          />
          {tab === "levels" && <LevelsTab />}
          {tab === "counts" && <CountsTab />}
          {tab === "transfers" && <TransfersTab />}
          {tab === "locations" && <LocationsTab />}
        </div>
      </HubRequired>
    </Page>
  )
}

// ---------- levels ----------

function LevelsTab() {
  const state = useLoadedState()
  const { fmt, currency } = useMoney()
  const [location, setLocation] = useState(state.pairing.location_id)
  const [category, setCategory] = useState("all")
  const [only, setOnly] = useState<"" | "low" | "out">("")
  const [q, setQ] = useState("")
  const [debounced, setDebounced] = useState("")
  const [history, setHistory] = useState<StockReportRow | null>(null)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 250)
    return () => clearTimeout(t)
  }, [q])
  const categories = useQuery({ queryKey: ["categories"], queryFn: api.categories })
  const rep = useQuery({
    queryKey: ["bo", "stock", location, category, only, debounced],
    queryFn: () => hub.stock({ location, category: category === "all" ? "" : category, only, q: debounced }),
  })
  const r = rep.data

  return (
    <>
      <div className="grid grid-cols-4 gap-3">
        <StatCard label="Stock value at cost" value={fmt(r?.value_minor ?? 0)} />
        <StatCard label="At selling price" value={fmt(r?.retail_minor ?? 0)} sub={r && r.value_minor > 0 ? `${fmt(r.retail_minor - r.value_minor)} potential margin` : undefined} />
        <StatCard label="Low on stock" value={r?.low_count ?? 0} sub="at or below reorder level" className={cn((r?.low_count ?? 0) > 0 && "border-amber-500/40")} />
        <StatCard label="Out of stock" value={r?.out_count ?? 0} className={cn((r?.out_count ?? 0) > 0 && "border-red-500/40")} />
      </div>
      <div className="flex flex-wrap items-end gap-3">
        <div className="relative w-72">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search name, SKU, barcode…" className="h-10 pl-9" />
        </div>
        <Select value={category} onValueChange={setCategory}>
          <SelectTrigger className="h-10 w-48">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All categories</SelectItem>
            {(categories.data ?? []).map((c) => (
              <SelectItem key={c.id} value={c.id}>
                {c.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <LocationSelect value={location} onChange={setLocation} label={false} className="w-48" />
        <Chips value={only} onChange={setOnly} options={[{ value: "", label: "All" }, { value: "low", label: "Low" }, { value: "out", label: "Out" }]} />
        <div className="ml-auto">
          <ExportButton
            name="stock.csv"
            disabled={!r}
            build={() =>
              toCSV(
                ["Item", "Variant", "SKU", "Category", "Location", "On hand", "Reorder level", "Sold (30 days)", "Unit cost", "Price", "Value at cost", "Value at price"],
                (r?.rows ?? []).map((x) => [x.product_name, x.variant_name, x.sku, x.category_name, x.location_name, formatQty(x.qty_milli), formatQty(x.reorder_level_milli),
                  formatQty(x.sold_milli_30d), csvMoney(x.cost_minor, currency.digits), csvMoney(x.price_minor, currency.digits), csvMoney(x.value_minor, currency.digits), csvMoney(x.retail_minor, currency.digits)]),
              )
            }
          />
        </div>
      </div>
      <QueryError error={rep.error} />
      {r && r.rows.length === 0 ? (
        <EmptyState icon={<Warehouse />} title={only === "low" ? "Nothing is running low" : only === "out" ? "Nothing is out of stock" : "No stock-tracked items match"} />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">Item</TableHead>
                <TableHead>Category</TableHead>
                <TableHead className="text-right">On hand</TableHead>
                <TableHead className="text-right">Reorder at</TableHead>
                <TableHead className="text-right" title="Sold at this location in the last 30 days">
                  Sold 30d
                </TableHead>
                <TableHead className="text-right">Unit cost</TableHead>
                <TableHead className="text-right">Value</TableHead>
                <TableHead className="w-10 pr-4" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {(r?.rows ?? []).map((x) => {
                const low = x.reorder_level_milli > 0 && x.qty_milli <= x.reorder_level_milli
                return (
                  <TableRow key={x.variant_id} className="cursor-pointer" onClick={() => setHistory(x)}>
                    <TableCell className="pl-4">
                      <div className="font-medium">{itemLabel(x)}</div>
                      <div className="text-xs text-muted-foreground">{x.sku}</div>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{x.category_name}</TableCell>
                    <TableCell className={cn("text-right font-medium tabular", x.qty_milli <= 0 ? "text-red-600" : low && "text-amber-700 dark:text-amber-400")}>
                      {formatQty(x.qty_milli)} {x.unit !== "pc" && <span className="text-xs text-muted-foreground">{x.unit}</span>}
                      {low && x.qty_milli > 0 && <AlertTriangle className="ml-1 inline size-3.5" />}
                    </TableCell>
                    <TableCell className="text-right text-muted-foreground tabular">{x.reorder_level_milli ? formatQty(x.reorder_level_milli) : "–"}</TableCell>
                    <TableCell className="text-right text-muted-foreground tabular">{x.sold_milli_30d ? formatQty(x.sold_milli_30d) : "–"}</TableCell>
                    <TableCell className="text-right tabular">{fmt(x.cost_minor)}</TableCell>
                    <TableCell className="text-right font-medium tabular">{fmt(x.value_minor)}</TableCell>
                    <TableCell className="pr-4">
                      <History className="size-4 text-muted-foreground" />
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}
      <HistorySheet row={history} location={location} onClose={() => setHistory(null)} />
    </>
  )
}

const reasonLabel: Record<string, string> = {
  opening: "Opening stock",
  sale: "Sale",
  return: "Customer return",
  receipt: "Delivery",
  adjustment: "Adjustment",
  count: "Stock count",
  transfer_in: "Transfer in",
  transfer_out: "Transfer out",
}

function HistorySheet({ row, location, onClose }: { row: StockReportRow | null; location: string; onClose: () => void }) {
  const moves = useQuery({ queryKey: ["bo", "moves", row?.variant_id, location], queryFn: () => hub.movements(row!.variant_id, location), enabled: !!row })
  return (
    <Sheet open={!!row} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-[600px] gap-0 sm:max-w-[600px]">
        <SheetHeader className="border-b">
          <SheetTitle>{row && itemLabel(row)}</SheetTitle>
          <SheetDescription>
            Stock history at {row?.location_name}: every sale, delivery, count and transfer, newest first.
          </SheetDescription>
        </SheetHeader>
        <div className="flex-1 overflow-y-auto">
          <QueryError error={moves.error} />
          {moves.isLoading && <Loader2 className="mx-auto mt-10 size-5 animate-spin text-muted-foreground" />}
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">When</TableHead>
                <TableHead>What</TableHead>
                <TableHead className="text-right">Change</TableHead>
                <TableHead className="pr-4 text-right">Balance</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(moves.data ?? []).map((m) => (
                <TableRow key={m.id}>
                  <TableCell className="pl-4 text-muted-foreground">{formatDateTime(m.created_at)}</TableCell>
                  <TableCell>
                    <div>
                      {reasonLabel[m.reason] ?? m.reason} {m.ref_no && <span className="text-muted-foreground tabular">{m.ref_no}</span>}
                    </div>
                    <div className="text-xs text-muted-foreground">{[m.user_name, m.terminal_name, m.note].filter(Boolean).join(" · ")}</div>
                  </TableCell>
                  <TableCell className={cn("text-right font-medium tabular", m.qty_milli > 0 ? "text-emerald-700 dark:text-emerald-400" : "text-red-600")}>
                    {m.qty_milli > 0 ? "+" : ""}
                    {formatQty(m.qty_milli)}
                  </TableCell>
                  <TableCell className="pr-4 text-right tabular">{formatQty(m.balance_milli)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </SheetContent>
    </Sheet>
  )
}

// ---------- counts ----------

function CountsTab() {
  const [openId, setOpenId] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const list = useQuery({ queryKey: ["bo", "counts"], queryFn: hub.counts })
  if (openId) return <CountScreen id={openId} onBack={() => setOpenId(null)} />
  return (
    <>
      <Button onClick={() => setCreating(true)}>
        <ClipboardCheck /> Start a stock count
      </Button>
      <QueryError error={list.error} />
      {list.data?.length === 0 ? (
        <EmptyState
          icon={<ClipboardCheck />}
          title="No stock counts yet"
          body="Count a whole location or one category. Keep selling while you count: each item is compared with what the system expected at the moment it was counted."
        />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">Count</TableHead>
                <TableHead>Scope</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Started</TableHead>
                <TableHead className="pr-4 text-right">Items counted</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(list.data ?? []).map((c) => (
                <TableRow key={c.id} className="cursor-pointer" onClick={() => setOpenId(c.id)}>
                  <TableCell className="pl-4">
                    <div className="font-medium">{c.name}</div>
                    <div className="text-xs text-muted-foreground tabular">{c.count_no}</div>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {c.location_name}
                    {c.category_name && ` · ${c.category_name}`}
                  </TableCell>
                  <TableCell>
                    <CountBadge status={c.status} />
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatDateTime(c.created_at)} · {c.created_by}
                  </TableCell>
                  <TableCell className="pr-4 text-right tabular">{c.line_count}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {creating && (
        <NewCountDialog
          onClose={() => setCreating(false)}
          onCreated={(id) => {
            setCreating(false)
            setOpenId(id)
          }}
        />
      )}
    </>
  )
}

function CountBadge({ status }: { status: StockCount["status"] }) {
  const s = {
    open: ["In progress", "border-sky-500/40 text-sky-700 dark:text-sky-400"],
    posted: ["Posted", "border-emerald-500/40 text-emerald-700 dark:text-emerald-400"],
    cancelled: ["Cancelled", "text-muted-foreground"],
  }[status]
  return (
    <Badge variant="outline" className={s[1]}>
      {s[0]}
    </Badge>
  )
}

function NewCountDialog({ onClose, onCreated }: { onClose: () => void; onCreated: (id: string) => void }) {
  const state = useLoadedState()
  const qc = useQueryClient()
  const categories = useQuery({ queryKey: ["categories"], queryFn: api.categories })
  const [name, setName] = useState("")
  const [location, setLocation] = useState(state.pairing.location_id)
  const [category, setCategory] = useState("all")
  const [busy, setBusy] = useState(false)
  async function create() {
    setBusy(true)
    try {
      const r = await hub.createCount({ name, location_id: location, category_id: category === "all" ? "" : category })
      qc.invalidateQueries({ queryKey: ["bo", "counts"] })
      onCreated(r.id)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Start a stock count</DialogTitle>
          <DialogDescription>Counting one category at a time keeps counts short. Several people can count at once from different tills.</DialogDescription>
        </DialogHeader>
        <div className="space-y-1.5">
          <Label>Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Fiction shelves, end of month" className="h-10" autoFocus />
        </div>
        <LocationSelect value={location} onChange={setLocation} />
        <div className="space-y-1.5">
          <Label>What to count</Label>
          <Select value={category} onValueChange={setCategory}>
            <SelectTrigger className="h-10 w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Everything at this location</SelectItem>
              {(categories.data ?? []).map((c) => (
                <SelectItem key={c.id} value={c.id}>
                  Only {c.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={create} disabled={busy}>
            {busy && <Loader2 className="animate-spin" />} Start counting
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function CountScreen({ id, onBack }: { id: string; onBack: () => void }) {
  const { fmt } = useMoney()
  const qc = useQueryClient()
  const count = useQuery({ queryKey: ["bo", "count", id], queryFn: () => hub.count(id), refetchInterval: 10_000 })
  const [scanMode, setScanMode] = useState<"add" | "set">("add")
  const [pending, setPending] = useState<StockReportRow | null>(null)
  const [qty, setQty] = useState("")
  const [posting, setPosting] = useState(false)
  const [editing, setEditing] = useState<StockCountLine | null>(null)
  const c = count.data
  const open = c?.status === "open"

  function upsertLocal(line: StockCountLine) {
    qc.setQueryData(["bo", "count", id], (old: StockCount | undefined) => {
      if (!old) return old
      const rest = (old.lines ?? []).filter((l) => l.variant_id !== line.variant_id)
      return { ...old, lines: line.product_name ? [line, ...rest] : rest }
    })
  }

  async function entry(variant_id: string, qty_milli: number, mode: "add" | "set" | "remove") {
    try {
      const line = await hub.countEntry(id, { variant_id, qty_milli, mode })
      upsertLocal(line)
      return true
    } catch (e) {
      toast.error(errorMessage(e))
      return false
    }
  }

  async function onPick(r: StockReportRow) {
    if (scanMode === "add") {
      if (await entry(r.variant_id, 1000, "add")) toast.success(`+1 ${itemLabel(r)}`, { duration: 1200 })
    } else {
      setPending(r)
      setQty("")
    }
  }

  const lines = useMemo(() => c?.lines ?? [], [c])
  const totals = useMemo(() => {
    let units = 0
    let value = 0
    let off = 0
    for (const l of lines) {
      const d = l.counted_milli - l.expected_milli
      if (d !== 0) off++
      units += d
      value += Math.round((d * l.cost_minor) / 1000)
    }
    return { units, value, off }
  }, [lines])

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Button variant="ghost" onClick={onBack}>
          ← All counts
        </Button>
        {c && (
          <div className="min-w-0">
            <div className="flex items-center gap-2 font-semibold">
              {c.name} <CountBadge status={c.status} />
            </div>
            <div className="text-xs text-muted-foreground">
              {c.count_no} · {c.location_name}
              {c.category_name ? ` · ${c.category_name} only` : " · all items"}
            </div>
          </div>
        )}
        {open && (
          <div className="ml-auto flex gap-2">
            <Button
              variant="ghost"
              onClick={async () => {
                try {
                  await hub.cancelCount(id)
                  qc.invalidateQueries({ queryKey: ["bo"] })
                  onBack()
                } catch (e) {
                  toast.error(errorMessage(e))
                }
              }}
            >
              Cancel count
            </Button>
            <Button onClick={() => setPosting(true)} disabled={lines.length === 0}>
              <ClipboardCheck /> Review & post
            </Button>
          </div>
        )}
      </div>
      <QueryError error={count.error} />
      {(c?.warnings ?? []).map((w) => (
        <div key={w} className="flex items-center gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-800 dark:text-amber-300">
          <AlertTriangle className="size-4 shrink-0" /> {w}
        </div>
      ))}
      <div className="grid grid-cols-4 gap-3">
        <StatCard label="Items counted" value={lines.length} />
        <StatCard label="Items with a difference" value={totals.off} />
        <StatCard label="Net difference" value={(totals.units > 0 ? "+" : "") + formatQty(totals.units)} sub="units" />
        <StatCard label="Value of difference" value={fmt(totals.value)} sub="at cost" className={cn(totals.value < 0 && "border-red-500/40")} />
      </div>
      {open && (
        <div className="flex items-center gap-3">
          <div className="flex-1">
            <ItemPicker location={c!.location_id} onPick={onPick} autoFocus placeholder={scanMode === "add" ? "Scan each item: every scan adds 1…" : "Scan or search, then type the quantity…"} />
          </div>
          <Chips value={scanMode} onChange={setScanMode} options={[{ value: "add", label: "Scan +1" }, { value: "set", label: "Type quantity" }]} />
        </div>
      )}
      <div className="overflow-hidden rounded-xl border bg-card">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/40">
              <TableHead className="pl-4">Item</TableHead>
              <TableHead className="text-right">Counted</TableHead>
              <TableHead className="text-right" title="What the system expected when the item was counted">
                Expected
              </TableHead>
              <TableHead className="text-right">Difference</TableHead>
              <TableHead className="text-right">Value</TableHead>
              <TableHead className="w-20 pr-4" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {lines.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="py-10 text-center text-muted-foreground">
                  Nothing counted yet
                </TableCell>
              </TableRow>
            )}
            {lines.map((l) => {
              const d = l.counted_milli - l.expected_milli
              return (
                <TableRow key={l.variant_id}>
                  <TableCell className="pl-4">
                    <div className="font-medium">
                      {itemLabel(l)}
                      {l.uncounted && <Badge variant="outline" className="ml-2">Not counted, set to 0</Badge>}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {l.sku} · {formatDateTime(l.counted_at)} {l.counted_by && `· ${l.counted_by}`}
                    </div>
                  </TableCell>
                  <TableCell className="text-right font-medium tabular">{formatQty(l.counted_milli)}</TableCell>
                  <TableCell className="text-right text-muted-foreground tabular">{formatQty(l.expected_milli)}</TableCell>
                  <TableCell className={cn("text-right font-medium tabular", d > 0 ? "text-emerald-700 dark:text-emerald-400" : d < 0 && "text-red-600")}>
                    {d === 0 ? "✓" : (d > 0 ? "+" : "") + formatQty(d)}
                  </TableCell>
                  <TableCell className="text-right tabular">{d === 0 ? "" : fmt(Math.round((d * l.cost_minor) / 1000))}</TableCell>
                  <TableCell className="pr-4 text-right">
                    {open && (
                      <>
                        <Button aria-label="Change count"
                          variant="ghost"
                          size="icon-sm"
                          title="Change count"
                          onClick={() => {
                            setQty(formatQty(l.counted_milli))
                            setEditing(l)
                          }}
                        >
                          <span className="text-xs">✎</span>
                        </Button>
                        <Button aria-label="Remove from count" variant="ghost" size="icon-sm" title="Remove" onClick={() => entry(l.variant_id, 0, "remove")}>
                          <X />
                        </Button>
                      </>
                    )}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
      {(pending || editing) && (
        <Dialog open onOpenChange={(o) => !o && (setPending(null), setEditing(null))}>
          <DialogContent className="sm:max-w-sm">
            <form
              className="space-y-4"
              onSubmit={async (e) => {
                e.preventDefault()
                const n = qty.trim() === "0" ? 0 : parseQty(qty)
                if (isNaN(n)) return
                const vid = pending?.variant_id ?? editing!.variant_id
                if (await entry(vid, n, "set")) {
                  setPending(null)
                  setEditing(null)
                }
              }}
            >
              <DialogHeader>
                <DialogTitle>How many {itemLabel((pending ?? editing)!)}?</DialogTitle>
                <DialogDescription>Enter the total you can see on the shelf.</DialogDescription>
              </DialogHeader>
              <Input autoFocus onFocus={(e) => e.target.select()} value={qty} onChange={(e) => setQty(e.target.value)} placeholder="0" className="h-12 text-center text-xl tabular" />
              <DialogFooter>
                <Button type="submit" className="w-full" disabled={qty.trim() === "" || (qty.trim() !== "0" && isNaN(parseQty(qty)))}>
                  Save count
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      )}
      {posting && c && <PostCountDialog count={c} totals={totals} onClose={() => setPosting(false)} onPosted={() => qc.invalidateQueries({ queryKey: ["bo"] })} />}
    </div>
  )
}

function PostCountDialog({ count, totals, onClose, onPosted }: { count: StockCount; totals: { units: number; value: number; off: number }; onClose: () => void; onPosted: () => void }) {
  const { fmt } = useMoney()
  const [zero, setZero] = useState(false)
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  async function post() {
    setBusy(true)
    try {
      await hub.postCount(count.id, { zero_uncounted: zero, note })
      toast.success("Count posted; stock corrected on every till")
      onPosted()
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Post {count.name}?</DialogTitle>
          <DialogDescription>
            Stock is corrected for the {totals.off} item{totals.off === 1 ? "" : "s"} that differ ({fmt(totals.value)} at cost). This can't be undone; a later count can correct it.
          </DialogDescription>
        </DialogHeader>
        <label className="flex items-start gap-3 rounded-lg border p-3 text-sm">
          <Switch checked={zero} onCheckedChange={setZero} className="mt-0.5" />
          <span>
            <span className="font-medium">Set items I didn't count to zero</span>
            <span className="block text-muted-foreground">
              Only for a full count of {count.category_name ? `“${count.category_name}”` : "this location"}: anything in stock that wasn't scanned is treated as missing.
            </span>
          </span>
        </label>
        <div className="space-y-1.5">
          <Label>Note</Label>
          <Input value={note} onChange={(e) => setNote(e.target.value)} className="h-10" />
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Keep counting
          </Button>
          <Button onClick={post} disabled={busy}>
            {busy && <Loader2 className="animate-spin" />} Post count
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---------- transfers ----------

function TransferBadge({ status }: { status: StockTransfer["status"] }) {
  const s = {
    in_transit: ["In transit", "border-amber-500/40 text-amber-700 dark:text-amber-400"],
    received: ["Received", "border-emerald-500/40 text-emerald-700 dark:text-emerald-400"],
    cancelled: ["Cancelled", "text-muted-foreground"],
  }[status]
  return (
    <Badge variant="outline" className={s[1]}>
      {s[0]}
    </Badge>
  )
}

function TransfersTab() {
  const locs = useLocations()
  const { fmt } = useMoney()
  const [creating, setCreating] = useState(false)
  const [openId, setOpenId] = useState<string | null>(null)
  const [filter, setFilter] = useState<"" | "in_transit">("")
  const list = useQuery({ queryKey: ["bo", "transfers", filter], queryFn: () => hub.transfers(filter) })
  const transit = useQuery({ queryKey: ["bo", "transfers", "in_transit"], queryFn: () => hub.transfers("in_transit") })
  const active = (locs.data ?? []).filter((l) => l.active)
  if (locs.data && active.length < 2) {
    return <EmptyState icon={<ArrowRightLeft />} title="Transfers need two locations" body="Add a store room or warehouse under Locations, then move stock between them here." />
  }
  const inTransit = transit.data ?? []
  const transitValue = inTransit.reduce((s, t) => s + t.cost_minor, 0)
  return (
    <>
      <div className="flex flex-wrap items-center gap-3">
        <Button onClick={() => setCreating(true)}>
          <ArrowRightLeft /> New transfer
        </Button>
        <Chips
          value={filter}
          onChange={setFilter}
          options={[
            { value: "", label: "All" },
            { value: "in_transit", label: `In transit${inTransit.length ? ` (${inTransit.length})` : ""}` },
          ]}
        />
        {inTransit.length > 0 && (
          <span className="text-sm text-muted-foreground">
            {fmt(transitValue)} of stock on the way (at cost)
          </span>
        )}
      </div>
      <QueryError error={list.error} />
      {list.data?.length === 0 ? (
        <EmptyState icon={<ArrowRightLeft />} title={filter ? "Nothing in transit" : "No transfers yet"} />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">Transfer</TableHead>
                <TableHead>From → to</TableHead>
                <TableHead>Sent</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Note</TableHead>
                <TableHead className="pr-4 text-right">Units</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(list.data ?? []).map((t) => (
                <TableRow key={t.id} className="cursor-pointer" onClick={() => setOpenId(t.id)}>
                  <TableCell className="pl-4 font-medium tabular">{t.transfer_no}</TableCell>
                  <TableCell>
                    {t.from_location_name} → {t.to_location_name}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatDateTime(t.created_at)} · {t.created_by}
                  </TableCell>
                  <TableCell>
                    <TransferBadge status={t.status} />
                  </TableCell>
                  <TableCell className="max-w-56 truncate text-muted-foreground">{t.note}</TableCell>
                  <TableCell className="pr-4 text-right tabular">{formatQty(t.items_milli)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      <TransferSheet id={openId} onClose={() => setOpenId(null)} />
      {creating && (
        <TransferDialog
          locations={active}
          onClose={() => setCreating(false)}
          onCreated={(id) => {
            setCreating(false)
            setOpenId(id)
          }}
        />
      )}
    </>
  )
}

function TransferSheet({ id, onClose }: { id: string | null; onClose: () => void }) {
  const qc = useQueryClient()
  const detail = useQuery({ queryKey: ["bo", "transfer", id], queryFn: () => hub.transfer(id!), enabled: !!id })
  const t = detail.data
  const [receiving, setReceiving] = useState(false)
  const [got, setGot] = useState<Record<string, string>>({})
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  const [confirmCancel, setConfirmCancel] = useState(false)
  function close() {
    setReceiving(false)
    setConfirmCancel(false)
    setNote("")
    onClose()
  }
  function startReceive() {
    setGot(Object.fromEntries((t?.lines ?? []).map((l) => [l.variant_id, formatQty(l.qty_milli)])))
    setReceiving(true)
  }
  const parsed = Object.fromEntries(Object.entries(got).map(([k, v]) => [k, parseQty(v)]))
  const lines = t?.lines ?? []
  const bad = lines.some((l) => {
    const q = parsed[l.variant_id]
    return isNaN(q) || q < 0 || q > l.qty_milli
  })
  const short = lines.reduce((s, l) => s + Math.max(0, l.qty_milli - (parsed[l.variant_id] ?? l.qty_milli)), 0)
  async function receive() {
    if (!t) return
    setBusy(true)
    try {
      await hub.receiveTransfer(t.id, { note, lines: lines.map((l) => ({ variant_id: l.variant_id, received_qty_milli: parsed[l.variant_id] })) })
      toast.success(`${t.transfer_no} received at ${t.to_location_name}`)
      setReceiving(false)
      qc.invalidateQueries({ queryKey: ["bo"] })
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  async function cancel() {
    if (!t) return
    setBusy(true)
    try {
      await hub.cancelTransfer(t.id)
      toast.success(`${t.transfer_no} cancelled; stock is back at ${t.from_location_name}`)
      setConfirmCancel(false)
      qc.invalidateQueries({ queryKey: ["bo"] })
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Sheet open={!!id} onOpenChange={(o) => !o && close()}>
      <SheetContent className="w-[560px] gap-0 sm:max-w-[560px]">
        <SheetHeader className="border-b">
          <SheetTitle className="flex items-center gap-2">
            Transfer {t?.transfer_no} {t && <TransferBadge status={t.status} />}
          </SheetTitle>
          <SheetDescription>
            {t && `${t.from_location_name} → ${t.to_location_name} · sent ${formatDateTime(t.created_at)} by ${t.created_by}`}
            {t?.status === "received" && t.received_at > 0 && ` · received ${formatDateTime(t.received_at)} by ${t.received_by}`}
          </SheetDescription>
        </SheetHeader>
        <div className="flex-1 overflow-y-auto">
          {t?.note && <p className="border-b px-4 py-3 text-sm text-muted-foreground">{t.note}</p>}
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">Item</TableHead>
                <TableHead className="text-right">Sent</TableHead>
                <TableHead className="pr-4 text-right">Received</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {lines.map((l) => {
                const shortBy = t?.status === "received" ? l.qty_milli - l.received_qty_milli : 0
                return (
                  <TableRow key={l.variant_id}>
                    <TableCell className="pl-4">
                      <div className="font-medium">{itemLabel(l)}</div>
                      <div className="text-xs text-muted-foreground">{l.sku}</div>
                    </TableCell>
                    <TableCell className="text-right tabular">{formatQty(l.qty_milli)}</TableCell>
                    <TableCell className="w-32 pr-4 text-right tabular">
                      {receiving ? (
                        <Input
                          value={got[l.variant_id] ?? ""}
                          onChange={(e) => setGot((g) => ({ ...g, [l.variant_id]: e.target.value }))}
                          onFocus={(e) => e.target.select()}
                          className={cn("h-9 text-right tabular", (isNaN(parsed[l.variant_id]) || parsed[l.variant_id] > l.qty_milli) && "border-destructive")}
                        />
                      ) : t?.status === "received" ? (
                        <span className={cn(shortBy > 0 && "font-medium text-destructive")}>
                          {formatQty(l.received_qty_milli)}
                          {shortBy > 0 && <span className="block text-xs">{formatQty(shortBy)} short</span>}
                        </span>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
          {t?.receive_note && <p className="px-4 py-3 text-sm text-muted-foreground">On receipt: {t.receive_note}</p>}
        </div>
        {t && (
          <div className="space-y-3 border-t p-4">
            {receiving ? (
              <>
                {short > 0 && (
                  <p className="flex items-center gap-2 text-sm text-amber-700 dark:text-amber-400">
                    <AlertTriangle className="size-4" /> {formatQty(short)} short. The missing stock is written off; it has already left {t.from_location_name}.
                  </p>
                )}
                <Input placeholder="Note (e.g. one carton damaged)" value={note} onChange={(e) => setNote(e.target.value)} className="h-10" />
                <div className="flex justify-end gap-2">
                  <Button variant="ghost" onClick={() => setReceiving(false)}>
                    Back
                  </Button>
                  <Button onClick={receive} disabled={busy || bad}>
                    {busy && <Loader2 className="animate-spin" />} Book in at {t.to_location_name}
                  </Button>
                </div>
              </>
            ) : confirmCancel ? (
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm">Return all items to {t.from_location_name}?</span>
                <div className="flex gap-2">
                  <Button variant="ghost" onClick={() => setConfirmCancel(false)}>
                    Keep
                  </Button>
                  <Button variant="destructive" onClick={cancel} disabled={busy}>
                    {busy && <Loader2 className="animate-spin" />} Cancel transfer
                  </Button>
                </div>
              </div>
            ) : (
              <div className="flex flex-wrap justify-end gap-2">
                <PrintButton kind="transfer" id={t.id} label="Transfer note" />
                {t.status === "in_transit" && (
                  <>
                    <Button variant="outline" onClick={() => setConfirmCancel(true)}>
                      <X /> Cancel
                    </Button>
                    <Button onClick={startReceive}>
                      <PackageCheck /> Receive
                    </Button>
                  </>
                )}
              </div>
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}

function TransferDialog({ locations, onClose, onCreated }: { locations: LocationInfo[]; onClose: () => void; onCreated: (id: string) => void }) {
  const state = useLoadedState()
  const qc = useQueryClient()
  const [from, setFrom] = useState(state.pairing.location_id)
  const [to, setTo] = useState(locations.find((l) => l.id !== state.pairing.location_id)?.id ?? "")
  const [note, setNote] = useState("")
  const [now, setNow] = useState(false)
  const [lines, setLines] = useState<{ row: StockReportRow; qty: string }[]>([])
  const [busy, setBusy] = useState(false)
  const exclude = useMemo(() => new Set(lines.map((l) => l.row.variant_id)), [lines])
  const parsed = lines.map((l) => parseQty(l.qty))
  const valid = from && to && from !== to && lines.length > 0 && parsed.every((q) => !isNaN(q) && q > 0)
  const toName = locations.find((l) => l.id === to)?.name ?? "the destination"
  async function submit() {
    setBusy(true)
    try {
      const { id } = await hub.createTransfer({
        from_location_id: from,
        to_location_id: to,
        note,
        receive_now: now,
        lines: lines.map((l, i) => ({ variant_id: l.row.variant_id, qty_milli: parsed[i] })),
      })
      toast.success(now ? "Stock moved" : `Sent. Receive it at ${toName} when it arrives.`)
      qc.invalidateQueries({ queryKey: ["bo"] })
      onCreated(id)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  const sel = (value: string, onChange: (v: string) => void) => (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="h-10 w-full">
        <SelectValue placeholder="Choose" />
      </SelectTrigger>
      <SelectContent>
        {locations.map((l) => (
          <SelectItem key={l.id} value={l.id}>
            {l.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Move stock between locations</DialogTitle>
          <DialogDescription>
            {now ? "Stock leaves one location and arrives at the other straight away." : `Stock leaves now and stays in transit until someone at ${toName} receives it.`}
          </DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label>From</Label>
            {sel(from, (v) => {
              setFrom(v)
              setLines([])
            })}
          </div>
          <div className="space-y-1.5">
            <Label>To</Label>
            {sel(to, setTo)}
          </div>
        </div>
        <ItemPicker location={from} exclude={exclude} onPick={(row) => setLines((ls) => [...ls, { row, qty: "1" }])} />
        <div className="min-h-[140px] flex-1 overflow-y-auto rounded-lg border">
          <Table>
            <TableBody>
              {lines.length === 0 && (
                <TableRow>
                  <TableCell className="py-10 text-center text-muted-foreground">Scan or search to add items</TableCell>
                </TableRow>
              )}
              {lines.map((l, i) => (
                <TableRow key={l.row.variant_id}>
                  <TableCell className="pl-3">
                    <div className="font-medium">{itemLabel(l.row)}</div>
                    <div className="text-xs text-muted-foreground">{formatQty(l.row.qty_milli)} at source</div>
                  </TableCell>
                  <TableCell className="w-32">
                    <Input value={l.qty} onChange={(e) => setLines((ls) => ls.map((x, j) => (j === i ? { ...x, qty: e.target.value } : x)))} className={cn("h-9 text-right tabular", isNaN(parsed[i]) && "border-destructive")} />
                  </TableCell>
                  <TableCell className="w-10 pr-3">
                    <Button aria-label="Remove line" variant="ghost" size="icon-sm" onClick={() => setLines((ls) => ls.filter((_, j) => j !== i))}>
                      <Trash2 />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <div className="space-y-1.5">
          <Label>Note</Label>
          <Input value={note} onChange={(e) => setNote(e.target.value)} className="h-10" placeholder="e.g. driver, vehicle, reason" />
        </div>
        <label className="flex items-center gap-3 text-sm">
          <Switch checked={now} onCheckedChange={setNow} />
          <span>
            Arrives now <span className="text-muted-foreground">(both locations are in the same building)</span>
          </span>
        </label>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={!valid || busy}>
            {busy && <Loader2 className="animate-spin" />} {now ? "Move stock" : "Send"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---------- locations ----------

function LocationsTab() {
  const { fmt } = useMoney()
  const qc = useQueryClient()
  const locs = useLocations()
  const [editing, setEditing] = useState<Partial<LocationInfo> | null>(null)
  const [busy, setBusy] = useState(false)
  async function save() {
    if (!editing) return
    setBusy(true)
    try {
      await hub.saveLocation({ id: editing.id ?? "", name: editing.name ?? "", code: editing.code ?? "", active: editing.active ?? true })
      toast.success("Location saved")
      qc.invalidateQueries({ queryKey: ["bo"] })
      setEditing(null)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <>
      <Button onClick={() => setEditing({ active: true })}>
        <Plus /> Add a location
      </Button>
      <p className="text-sm text-muted-foreground">Shop floors, store rooms and warehouses each hold their own stock. Tills sell from the location chosen when they were paired.</p>
      <QueryError error={locs.error} />
      <div className="overflow-hidden rounded-xl border bg-card">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/40">
              <TableHead className="pl-4">Location</TableHead>
              <TableHead>Code</TableHead>
              <TableHead className="text-right">Tills</TableHead>
              <TableHead className="pr-4 text-right">Stock value</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(locs.data ?? []).map((l) => (
              <TableRow key={l.id} className={cn("cursor-pointer", !l.active && "opacity-50")} onClick={() => setEditing(l)}>
                <TableCell className="pl-4 font-medium">
                  <MapPin className="mr-1.5 inline size-4 text-muted-foreground" />
                  {l.name}
                  {l.is_default && <Badge variant="secondary" className="ml-2">Main</Badge>}
                  {!l.active && <Badge variant="outline" className="ml-2">Inactive</Badge>}
                </TableCell>
                <TableCell className="text-muted-foreground">{l.code}</TableCell>
                <TableCell className="text-right tabular">{l.terminals}</TableCell>
                <TableCell className="pr-4 text-right font-medium tabular">{fmt(l.value_minor)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {editing && (
        <Dialog open onOpenChange={(o) => !o && setEditing(null)}>
          <DialogContent className="sm:max-w-sm">
            <DialogHeader>
              <DialogTitle>{editing.id ? "Edit location" : "New location"}</DialogTitle>
            </DialogHeader>
            <div className="space-y-1.5">
              <Label>Name</Label>
              <Input autoFocus value={editing.name ?? ""} onChange={(e) => setEditing({ ...editing, name: e.target.value })} placeholder="e.g. Back store, Ikeja warehouse" className="h-10" />
            </div>
            <div className="space-y-1.5">
              <Label>Short code (optional)</Label>
              <Input value={editing.code ?? ""} onChange={(e) => setEditing({ ...editing, code: e.target.value })} className="h-10" />
            </div>
            {editing.id && !editing.is_default && (
              <label className="flex items-center gap-2 text-sm">
                <Switch checked={editing.active ?? true} onCheckedChange={(v) => setEditing({ ...editing, active: v })} /> Active
              </label>
            )}
            <DialogFooter>
              <Button variant="ghost" onClick={() => setEditing(null)}>
                Cancel
              </Button>
              <Button onClick={save} disabled={!editing.name?.trim() || busy}>
                {busy && <Loader2 className="animate-spin" />} Save
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </>
  )
}
