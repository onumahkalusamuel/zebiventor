// Stock on its way to this till's location. Receiving works offline: it is
// queued like a sale and reaches the hub when the connection returns.
import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { AlertTriangle, CloudOff, Loader2, PackageCheck, Search, Send, Trash2, Truck } from "lucide-react"
import { toast } from "sonner"
import { itemLabel } from "@/components/backoffice"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api, errorMessage } from "@/lib/api"
import { formatDate, formatDateTime, formatQty, parseQty } from "@/lib/format"
import { useLoadedState } from "@/lib/state"
import type { ExpectedDelivery, IncomingTransfer, Item, TillLocation } from "@/lib/types"
import { cn } from "@/lib/utils"

export function ArrivingHere() {
  const state = useLoadedState()
  const list = useQuery({ queryKey: ["incoming", state.status.rev], queryFn: api.incomingTransfers, refetchInterval: 30_000 })
  const deliveries = useQuery({ queryKey: ["incoming", "po", state.status.rev], queryFn: api.expectedDeliveries, refetchInterval: 30_000 })
  const outgoing = useQuery({ queryKey: ["incoming", "out", state.status.rev, state.status.pending], queryFn: api.outgoingTransfers, refetchInterval: 30_000 })
  const [open, setOpen] = useState<IncomingTransfer | null>(null)
  const [openPO, setOpenPO] = useState<ExpectedDelivery | null>(null)
  const items = list.data ?? []
  const pos = deliveries.data ?? []
  const out = outgoing.data ?? []
  if (!items.length && !pos.length && !out.length) return null
  return (
    <section className="mx-8 mb-4 rounded-xl border bg-card">
      <div className="flex items-center gap-2 border-b px-4 py-3">
        <Truck className="size-4 text-muted-foreground" />
        <h3 className="text-sm font-medium">On its way here</h3>
        <span className="text-xs text-muted-foreground">Count what arrives and book it in, even if the hub can't be reached.</span>
      </div>
      <div className="divide-y">
        {pos.map(({ order: o, received_here }) => (
          <div key={o.id} className="flex items-center gap-4 px-4 py-2.5 text-sm">
            <div className="w-28 font-medium tabular">{o.po_no}</div>
            <div className="flex-1">
              from {o.supplier_name} · {o.lines?.length ?? 0} items ordered{o.status === "partial" && " · part delivered"}
              {o.expected_at > 0 && <span className="text-muted-foreground"> · due {formatDate(o.expected_at)}</span>}
            </div>
            {received_here ? (
              <Badge variant="secondary">
                <CloudOff /> Delivery booked, waiting to sync
              </Badge>
            ) : (
              <Button size="sm" onClick={() => setOpenPO({ order: o, received_here })}>
                <PackageCheck /> Receive delivery
              </Button>
            )}
          </div>
        ))}
        {out.map((t) => (
          <div key={t.id} className="flex items-center gap-4 px-4 py-2.5 text-sm text-muted-foreground">
            <div className="w-28 font-medium">Sending</div>
            <div className="flex-1">
              to {t.to_location_name} · {formatQty(t.items_milli)} items · {formatDateTime(t.created_at)}
            </div>
            <Badge variant="secondary">
              <CloudOff /> Waiting for the hub to number it
            </Badge>
          </div>
        ))}
        {items.map((t) => (
          <div key={t.id} className="flex items-center gap-4 px-4 py-2.5 text-sm">
            <div className="w-28 font-medium tabular">{t.transfer_no}</div>
            <div className="flex-1">
              from {t.from_location_name} · {formatQty(t.items_milli)} items · sent {formatDateTime(t.created_at)}
              {t.note && <span className="text-muted-foreground"> · {t.note}</span>}
            </div>
            {t.received_here ? (
              <Badge variant="secondary">
                <CloudOff /> Received, waiting to sync
              </Badge>
            ) : (
              <Button size="sm" onClick={() => setOpen(t)}>
                <PackageCheck /> Receive
              </Button>
            )}
          </div>
        ))}
      </div>
      {open && <ReceiveDialog t={open} onClose={() => setOpen(null)} />}
      {openPO && <DeliveryDialog o={openPO} onClose={() => setOpenPO(null)} />}
    </section>
  )
}

function ReceiveDialog({ t, onClose }: { t: IncomingTransfer; onClose: () => void }) {
  const qc = useQueryClient()
  const [got, setGot] = useState<Record<string, string>>(() => Object.fromEntries((t.lines ?? []).map((l) => [l.variant_id, formatQty(l.qty_milli)])))
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  const lines = t.lines ?? []
  const parsed = Object.fromEntries(Object.entries(got).map(([k, v]) => [k, parseQty(v)]))
  const bad = lines.some((l) => isNaN(parsed[l.variant_id]) || parsed[l.variant_id] < 0 || parsed[l.variant_id] > l.qty_milli)
  const short = lines.reduce((s, l) => s + Math.max(0, l.qty_milli - (parsed[l.variant_id] ?? l.qty_milli)), 0)

  async function save() {
    setBusy(true)
    try {
      await api.receiveTransferHere(t.id, { note, lines: lines.map((l) => ({ variant_id: l.variant_id, received_qty_milli: parsed[l.variant_id] })) })
      toast.success(`${t.transfer_no} booked in at ${t.to_location_name}`)
      qc.invalidateQueries({ queryKey: ["incoming"] })
      qc.invalidateQueries({ queryKey: ["bo"] })
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && !busy && onClose()}>
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Receive {t.transfer_no}</DialogTitle>
          <DialogDescription>
            From {t.from_location_name}, sent {formatDateTime(t.created_at)} by {t.created_by}. Enter what actually arrived.
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-3">Item</TableHead>
                <TableHead className="text-right">Sent</TableHead>
                <TableHead className="pr-3 text-right">Arrived</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {lines.map((l) => (
                <TableRow key={l.variant_id}>
                  <TableCell className="pl-3">
                    <div className="font-medium">{itemLabel(l)}</div>
                    <div className="text-xs text-muted-foreground">{l.sku}</div>
                  </TableCell>
                  <TableCell className="text-right tabular">{formatQty(l.qty_milli)}</TableCell>
                  <TableCell className="w-32 pr-3">
                    <Input
                      aria-label={`${itemLabel(l)} arrived`}
                      value={got[l.variant_id] ?? ""}
                      onChange={(e) => setGot((g) => ({ ...g, [l.variant_id]: e.target.value }))}
                      onFocus={(e) => e.target.select()}
                      className={cn("h-9 text-right tabular", (isNaN(parsed[l.variant_id]) || parsed[l.variant_id] > l.qty_milli) && "border-destructive")}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        {short > 0 && (
          <p className="flex items-center gap-2 text-sm text-amber-700 dark:text-amber-400">
            <AlertTriangle className="size-4" /> {formatQty(short)} short. It is recorded as missing on the transfer.
          </p>
        )}
        <Input placeholder="Note (e.g. one carton damaged)" value={note} onChange={(e) => setNote(e.target.value)} />
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button onClick={save} disabled={busy || bad}>
            {busy && <Loader2 className="animate-spin" />} Book in at {t.to_location_name}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function DeliveryDialog({ o: { order: o }, onClose }: { o: ExpectedDelivery; onClose: () => void }) {
  const qc = useQueryClient()
  const lines = o.lines ?? []
  const [got, setGot] = useState<Record<string, string>>(() =>
    Object.fromEntries(lines.map((l) => [l.id, formatQty(Math.max(0, l.qty_ordered_milli - l.qty_received_milli))])),
  )
  const [ref, setRef] = useState("")
  const [note, setNote] = useState("")
  const [close, setClose] = useState(false)
  const [busy, setBusy] = useState(false)
  const parsed = Object.fromEntries(Object.entries(got).map(([k, v]) => [k, parseQty(v)]))
  const bad = lines.some((l) => isNaN(parsed[l.id]) || parsed[l.id] < 0)
  const any = lines.some((l) => parsed[l.id] > 0)

  async function save() {
    setBusy(true)
    try {
      await api.receiveDeliveryHere(o.id, { supplier_ref: ref, note, close_po: close, lines: lines.map((l) => ({ po_line_id: l.id, qty_milli: parsed[l.id] || 0 })) })
      toast.success(`Delivery for ${o.po_no} booked in`)
      qc.invalidateQueries({ queryKey: ["incoming"] })
      qc.invalidateQueries({ queryKey: ["bo"] })
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(v) => !v && !busy && onClose()}>
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Receive delivery for {o.po_no}</DialogTitle>
          <DialogDescription>From {o.supplier_name}. Enter what arrived; costs are the order's prices.</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-3">Item</TableHead>
                <TableHead className="text-right">Still due</TableHead>
                <TableHead className="pr-3 text-right">Arrived</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {lines.map((l) => (
                <TableRow key={l.id}>
                  <TableCell className="pl-3">
                    <div className="font-medium">{itemLabel(l)}</div>
                    <div className="text-xs text-muted-foreground">{l.sku}</div>
                  </TableCell>
                  <TableCell className="text-right tabular">{formatQty(Math.max(0, l.qty_ordered_milli - l.qty_received_milli))}</TableCell>
                  <TableCell className="w-32 pr-3">
                    <Input
                      aria-label={`${itemLabel(l)} arrived`}
                      value={got[l.id] ?? ""}
                      onChange={(e) => setGot((g) => ({ ...g, [l.id]: e.target.value }))}
                      onFocus={(e) => e.target.select()}
                      className={cn("h-9 text-right tabular", (isNaN(parsed[l.id]) || parsed[l.id] < 0) && "border-destructive")}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <div className="grid grid-cols-2 gap-2">
          <Input placeholder="Supplier's invoice / waybill no." value={ref} onChange={(e) => setRef(e.target.value)} aria-label="Supplier's reference" />
          <Input placeholder="Note" value={note} onChange={(e) => setNote(e.target.value)} aria-label="Note" />
        </div>
        <label className="flex items-center gap-2 text-sm">
          <Switch checked={close} onCheckedChange={setClose} /> Nothing more is coming on this order
        </label>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button onClick={save} disabled={busy || bad || !any}>
            {busy && <Loader2 className="animate-spin" />} Book in
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Send stock from this till's location to another (works offline). */
export function SendStockButton() {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant="outline" className="h-10" onClick={() => setOpen(true)}>
        <Send /> Send stock from here
      </Button>
      {open && <SendStockDialog onClose={() => setOpen(false)} />}
    </>
  )
}

function SendStockDialog({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient()
  const locations = useQuery({ queryKey: ["till-locations"], queryFn: api.tillLocations })
  const [to, setTo] = useState("")
  const [q, setQ] = useState("")
  const [lines, setLines] = useState<{ item: Item; qty: string }[]>([])
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  const search = useQuery({ queryKey: ["send-search", q], queryFn: () => api.searchItems(q), enabled: q.trim().length > 1 })
  const parsed = lines.map((l) => parseQty(l.qty))
  const bad = parsed.some((n) => isNaN(n) || n <= 0)
  const dest = (locations.data ?? []).find((l: TillLocation) => l.id === to)

  function add(item: Item) {
    setQ("")
    if (!item.track_stock) {
      toast.error(`${item.name} doesn't track stock`)
      return
    }
    if (!lines.some((l) => l.item.variant_id === item.variant_id)) setLines((ls) => [...ls, { item, qty: "1" }])
  }

  async function save() {
    setBusy(true)
    try {
      await api.sendTransferHere({ to_location_id: to, note, lines: lines.map((l, i) => ({ variant_id: l.item.variant_id, qty_milli: parsed[i] })) })
      toast.success(`Stock sent to ${dest?.name}. It shows as in transit until they receive it.`)
      qc.invalidateQueries({ queryKey: ["incoming"] })
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(v) => !v && !busy && onClose()}>
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Send stock from here</DialogTitle>
          <DialogDescription>Works without the hub: it's sent when the connection returns, and the other location receives it.</DialogDescription>
        </DialogHeader>
        <div className="space-y-1.5">
          <Label>Send to</Label>
          <Select value={to} onValueChange={setTo}>
            <SelectTrigger className="w-full" aria-label="Send to">
              <SelectValue placeholder="Choose a location" />
            </SelectTrigger>
            <SelectContent>
              {(locations.data ?? []).map((l) => (
                <SelectItem key={l.id} value={l.id}>
                  {l.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="relative">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-9"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && search.data?.[0]) add(search.data[0])
            }}
            placeholder="Scan or search items"
            aria-label="Add an item"
          />
          {q.trim().length > 1 && (search.data?.length ?? 0) > 0 && (
            <ul role="listbox" aria-label="Matching items" className="absolute top-full right-0 left-0 z-20 mt-1 max-h-60 overflow-y-auto rounded-lg border bg-popover p-1 shadow-lg">
              {search.data!.slice(0, 10).map((it) => (
                <li key={it.variant_id}>
                  <button className="flex w-full justify-between rounded-md px-3 py-1.5 text-left text-sm hover:bg-muted focus-visible:bg-muted focus-visible:outline-none" onClick={() => add(it)}>
                    <span>
                      {it.name}
                      {it.variant_name && <span className="text-muted-foreground"> · {it.variant_name}</span>}
                    </span>
                    <span className="text-muted-foreground tabular">{formatQty(it.stock_milli)} here</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
        <div className="min-h-0 flex-1 divide-y overflow-y-auto rounded-lg border">
          {lines.length === 0 && <p className="p-3 text-sm text-muted-foreground">No items yet.</p>}
          {lines.map((l, i) => (
            <div key={l.item.variant_id} className="flex items-center gap-3 p-2 pl-3 text-sm">
              <div className="flex-1">
                {l.item.name}
                {l.item.variant_name && <span className="text-muted-foreground"> · {l.item.variant_name}</span>}
              </div>
              <Input
                aria-label={`Quantity of ${l.item.name}`}
                className={cn("h-9 w-24 text-right tabular", (isNaN(parsed[i]) || parsed[i] <= 0) && "border-destructive")}
                value={l.qty}
                onChange={(e) => setLines((ls) => ls.map((x, j) => (j === i ? { ...x, qty: e.target.value } : x)))}
              />
              <Button variant="ghost" size="icon-sm" aria-label={`Remove ${l.item.name}`} onClick={() => setLines((ls) => ls.filter((_, j) => j !== i))}>
                <Trash2 />
              </Button>
            </div>
          ))}
        </div>
        <Input placeholder="Note (e.g. driver's name)" value={note} onChange={(e) => setNote(e.target.value)} aria-label="Note" />
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button onClick={save} disabled={busy || !to || !lines.length || bad}>
            {busy ? <Loader2 className="animate-spin" /> : <Send />} Send{dest ? ` to ${dest.name}` : ""}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
