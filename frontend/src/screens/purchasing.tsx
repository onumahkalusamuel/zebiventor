import { useMemo, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Ban, CheckCheck, ClipboardList, Loader2, PackageCheck, PackagePlus, Pencil, Plus, Send, Trash2, Truck } from "lucide-react"
import { toast } from "sonner"
import { Chips, HubRequired, ItemPicker, LocationSelect, QueryError, itemLabel, toDateInput } from "@/components/backoffice"
import { EmptyState, Page, PageHeader } from "@/components/page"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Switch } from "@/components/ui/switch"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { errorMessage } from "@/lib/api"
import { formatDateTime, formatQty, parseMoney, parseQty } from "@/lib/format"
import { hub } from "@/lib/hub"
import { useLoadedState, useMoney } from "@/lib/state"
import type { GoodsReceipt, POStatus, PurchaseOrder, StockReportRow, Supplier } from "@/lib/types"
import { cn } from "@/lib/utils"

type Tab = "orders" | "deliveries" | "suppliers"

export function Purchasing() {
  const [tab, setTab] = useState<Tab>("orders")
  const [editing, setEditing] = useState<PurchaseOrder | "new" | null>(null)
  const [receiving, setReceiving] = useState<PurchaseOrder | "direct" | null>(null)
  const [supplier, setSupplier] = useState<Supplier | "new" | null>(null)
  return (
    <Page>
      <PageHeader
        title="Purchasing"
        description="Order from suppliers and book deliveries into stock. Item costs follow what you actually paid."
        actions={
          <>
            <Button variant="outline" className="h-10" onClick={() => setReceiving("direct")}>
              <PackageCheck /> Receive without an order
            </Button>
            <Button className="h-10" onClick={() => setEditing("new")}>
              <PackagePlus /> New order
            </Button>
          </>
        }
      />
      <HubRequired>
        <div className="space-y-4 px-8 pb-8">
          <Chips
            value={tab}
            onChange={setTab}
            options={[
              { value: "orders", label: "Orders" },
              { value: "deliveries", label: "Deliveries" },
              { value: "suppliers", label: "Suppliers" },
            ]}
          />
          {tab === "orders" && <OrdersTab onEdit={setEditing} onReceive={setReceiving} />}
          {tab === "deliveries" && <DeliveriesTab />}
          {tab === "suppliers" && <SuppliersTab onEdit={setSupplier} />}
        </div>
      </HubRequired>
      {editing && <OrderEditor order={editing === "new" ? null : editing} onClose={() => setEditing(null)} onNewSupplier={() => setSupplier("new")} />}
      {receiving && <ReceiveDialog order={receiving === "direct" ? null : receiving} onClose={() => setReceiving(null)} />}
      {supplier && <SupplierDialog supplier={supplier === "new" ? null : supplier} onClose={() => setSupplier(null)} />}
    </Page>
  )
}

const statusStyle: Record<POStatus, { label: string; className: string }> = {
  draft: { label: "Draft", className: "border-border text-muted-foreground" },
  ordered: { label: "Ordered", className: "border-sky-500/40 text-sky-700 dark:text-sky-400" },
  partial: { label: "Part received", className: "border-amber-500/40 text-amber-700 dark:text-amber-400" },
  received: { label: "Received", className: "border-emerald-500/40 text-emerald-700 dark:text-emerald-400" },
  cancelled: { label: "Cancelled", className: "border-border text-muted-foreground line-through" },
}

export function POBadge({ status }: { status: POStatus }) {
  const s = statusStyle[status]
  return (
    <Badge variant="outline" className={s.className}>
      {s.label}
    </Badge>
  )
}

function useSuppliers() {
  return useQuery({ queryKey: ["bo", "suppliers"], queryFn: hub.suppliers })
}

function OrdersTab({ onEdit, onReceive }: { onEdit: (o: PurchaseOrder) => void; onReceive: (o: PurchaseOrder) => void }) {
  const { fmt } = useMoney()
  const [filter, setFilter] = useState<"open" | "">("open")
  const [openId, setOpenId] = useState<string | null>(null)
  const orders = useQuery({ queryKey: ["bo", "pos", filter], queryFn: () => hub.purchaseOrders(filter) })
  return (
    <>
      <Chips value={filter} onChange={setFilter} options={[{ value: "open", label: "Open" }, { value: "", label: "All orders" }]} />
      <QueryError error={orders.error} />
      {orders.data?.length === 0 ? (
        <EmptyState icon={<ClipboardList />} title={filter ? "No open orders" : "No orders yet"} body="Create an order, place it with the supplier, then receive the goods when they arrive." />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">Order</TableHead>
                <TableHead>Supplier</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Deliver to</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="text-right">Items</TableHead>
                <TableHead className="pr-4 text-right">Value</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(orders.data ?? []).map((o) => (
                <TableRow key={o.id} className="cursor-pointer" onClick={() => setOpenId(o.id)}>
                  <TableCell className="pl-4 font-medium tabular">{o.po_no}</TableCell>
                  <TableCell>{o.supplier_name}</TableCell>
                  <TableCell>
                    <POBadge status={o.status} />
                  </TableCell>
                  <TableCell className="text-muted-foreground">{o.location_name}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatDateTime(o.created_at)}
                    {o.expected_at > 0 && <div className="text-xs">due {new Date(o.expected_at).toLocaleDateString()}</div>}
                  </TableCell>
                  <TableCell className="text-right tabular">{o.line_count}</TableCell>
                  <TableCell className="pr-4 text-right font-medium tabular">{fmt(o.total_minor)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      <OrderSheet
        id={openId}
        onClose={() => setOpenId(null)}
        onEdit={(o) => {
          setOpenId(null)
          onEdit(o)
        }}
        onReceive={(o) => {
          setOpenId(null)
          onReceive(o)
        }}
      />
    </>
  )
}

function OrderSheet({ id, onClose, onEdit, onReceive }: { id: string | null; onClose: () => void; onEdit: (o: PurchaseOrder) => void; onReceive: (o: PurchaseOrder) => void }) {
  const { fmt } = useMoney()
  const qc = useQueryClient()
  const order = useQuery({ queryKey: ["bo", "po", id], queryFn: () => hub.purchaseOrder(id!), enabled: !!id })
  const [busy, setBusy] = useState(false)
  const o = order.data

  async function setStatus(status: POStatus, done: string) {
    if (!o) return
    setBusy(true)
    try {
      await hub.setPurchaseOrderStatus(o.id, status)
      toast.success(done)
      qc.invalidateQueries({ queryKey: ["bo"] })
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  const outstanding = (o?.lines ?? []).reduce((s, l) => s + Math.max(0, l.qty_ordered_milli - l.qty_received_milli), 0)
  return (
    <Sheet open={!!id} onOpenChange={(v) => !v && onClose()}>
      <SheetContent className="w-[640px] gap-0 sm:max-w-[640px]">
        <SheetHeader className="border-b">
          <SheetTitle className="flex items-center gap-2">
            Order {o?.po_no} {o && <POBadge status={o.status} />}
          </SheetTitle>
          <SheetDescription>
            {o && `${o.supplier_name} · deliver to ${o.location_name} · created ${formatDateTime(o.created_at)} by ${o.created_by}`}
          </SheetDescription>
        </SheetHeader>
        <div className="flex-1 space-y-5 overflow-y-auto p-5">
          <QueryError error={order.error} />
          {!o && !order.error && <Loader2 className="mx-auto mt-10 size-5 animate-spin text-muted-foreground" />}
          {o && (
            <>
              {o.note && <p className="rounded-lg bg-muted px-3 py-2 text-sm">{o.note}</p>}
              <div className="overflow-hidden rounded-lg border">
                <Table>
                  <TableHeader>
                    <TableRow className="bg-muted/40">
                      <TableHead className="pl-3">Item</TableHead>
                      <TableHead className="text-right">Ordered</TableHead>
                      <TableHead className="text-right">Received</TableHead>
                      <TableHead className="text-right">Unit cost</TableHead>
                      <TableHead className="pr-3 text-right">Total</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(o.lines ?? []).map((l) => (
                      <TableRow key={l.id}>
                        <TableCell className="pl-3">
                          <div className="font-medium">{itemLabel(l)}</div>
                          <div className="text-xs text-muted-foreground">{l.sku}</div>
                        </TableCell>
                        <TableCell className="text-right tabular">{formatQty(l.qty_ordered_milli)}</TableCell>
                        <TableCell className={cn("text-right tabular", l.qty_received_milli >= l.qty_ordered_milli ? "text-emerald-700 dark:text-emerald-400" : l.qty_received_milli > 0 && "text-amber-700 dark:text-amber-400")}>
                          {formatQty(l.qty_received_milli)}
                        </TableCell>
                        <TableCell className="text-right tabular">{fmt(l.unit_cost_minor)}</TableCell>
                        <TableCell className="pr-3 text-right tabular">{fmt(Math.round((l.qty_ordered_milli * l.unit_cost_minor) / 1000))}</TableCell>
                      </TableRow>
                    ))}
                    <TableRow>
                      <TableCell colSpan={4} className="pl-3 font-medium">
                        Order value
                      </TableCell>
                      <TableCell className="pr-3 text-right font-semibold tabular">{fmt(o.total_minor)}</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>
              {(o.receipts ?? []).length > 0 && (
                <div className="space-y-2">
                  <div className="text-sm font-medium">Deliveries</div>
                  {o.receipts!.map((g) => (
                    <div key={g.id} className="flex items-center justify-between rounded-lg border px-3 py-2 text-sm">
                      <div>
                        <span className="font-medium tabular">{g.grn_no}</span>
                        {g.supplier_ref && <span className="text-muted-foreground"> · supplier ref {g.supplier_ref}</span>}
                        <div className="text-xs text-muted-foreground">
                          {formatDateTime(g.created_at)} · {g.received_by}
                        </div>
                      </div>
                      <div className="font-medium tabular">{fmt(g.total_minor)}</div>
                    </div>
                  ))}
                </div>
              )}
            </>
          )}
        </div>
        {o && (
          <div className="flex flex-wrap gap-2 border-t p-4">
            {o.status === "draft" && (
              <>
                <Button variant="outline" onClick={() => onEdit(o)} disabled={busy}>
                  <Pencil /> Edit
                </Button>
                <Button variant="ghost" onClick={() => setStatus("cancelled", "Order cancelled")} disabled={busy}>
                  <Ban /> Cancel order
                </Button>
                <Button className="ml-auto" onClick={() => setStatus("ordered", `${o.po_no} placed with ${o.supplier_name}`)} disabled={busy}>
                  <Send /> Place order
                </Button>
              </>
            )}
            {(o.status === "ordered" || o.status === "partial") && (
              <>
                {o.status === "ordered" && (
                  <Button variant="ghost" onClick={() => setStatus("cancelled", "Order cancelled")} disabled={busy}>
                    <Ban /> Cancel order
                  </Button>
                )}
                {o.status === "partial" && (
                  <Button variant="ghost" onClick={() => setStatus("received", "Order closed; the rest won't be delivered")} disabled={busy} title={`${formatQty(outstanding)} still outstanding`}>
                    <CheckCheck /> Close (rest not coming)
                  </Button>
                )}
                <Button className="ml-auto" onClick={() => onReceive(o)} disabled={busy}>
                  <Truck /> Receive delivery
                </Button>
              </>
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}

interface EditLine {
  variant_id: string
  label: string
  sku: string
  qty: string
  cost: string
  on_hand: number
  reorder: number
}

function OrderEditor({ order, onClose, onNewSupplier }: { order: PurchaseOrder | null; onClose: () => void; onNewSupplier: () => void }) {
  const state = useLoadedState()
  const qc = useQueryClient()
  const { fmt, currency } = useMoney()
  const suppliers = useSuppliers()
  const full = useQuery({ queryKey: ["bo", "po", order?.id], queryFn: () => hub.purchaseOrder(order!.id), enabled: !!order })
  const [supplierId, setSupplierId] = useState(order?.supplier_id ?? "")
  const [location, setLocation] = useState(order?.location_id || state.pairing.location_id)
  const [expected, setExpected] = useState(order?.expected_at ? toDateInput(order.expected_at) : "")
  const [note, setNote] = useState(order?.note ?? "")
  const [lines, setLines] = useState<EditLine[] | null>(order ? null : [])
  const [busy, setBusy] = useState(false)
  const toMoney = (minor: number) => (minor / 10 ** currency.digits).toFixed(currency.digits)

  // Load the draft's lines once.
  if (order && lines === null && full.data) {
    setLines(
      (full.data.lines ?? []).map((l) => ({
        variant_id: l.variant_id, label: itemLabel(l), sku: l.sku, qty: formatQty(l.qty_ordered_milli), cost: toMoney(l.unit_cost_minor), on_hand: 0, reorder: 0,
      })),
    )
  }
  const rows = useMemo(() => lines ?? [], [lines])
  const exclude = useMemo(() => new Set(rows.map((l) => l.variant_id)), [rows])

  function add(r: StockReportRow, qtyMilli = 1000) {
    setLines((ls) => [...(ls ?? []), { variant_id: r.variant_id, label: itemLabel(r), sku: r.sku, qty: formatQty(qtyMilli), cost: toMoney(r.cost_minor), on_hand: r.qty_milli, reorder: r.reorder_level_milli }])
  }

  async function addLowStock() {
    try {
      const rep = await hub.stock({ location, only: "low" })
      const extra = rep.rows.filter((r) => !exclude.has(r.variant_id))
      if (extra.length === 0) {
        toast.info("Nothing is at or below its reorder level here")
        return
      }
      // Order back up to twice the reorder level, in whole units.
      for (const r of extra) add(r, Math.max(1000, Math.ceil((2 * r.reorder_level_milli - Math.max(0, r.qty_milli)) / 1000) * 1000))
      toast.success(`Added ${extra.length} low-stock item${extra.length === 1 ? "" : "s"}`)
    } catch (e) {
      toast.error(errorMessage(e))
    }
  }

  const parsed = rows.map((l) => ({ qty: parseQty(l.qty), cost: parseMoney(l.cost, currency) }))
  const total = parsed.reduce((s, p) => s + (isNaN(p.qty) || isNaN(p.cost) ? 0 : Math.round((p.qty * p.cost) / 1000)), 0)
  const valid = supplierId && rows.length > 0 && parsed.every((p) => !isNaN(p.qty) && !isNaN(p.cost))

  async function save(submit: boolean) {
    setBusy(true)
    try {
      await hub.savePurchaseOrder({
        id: order?.id ?? "", supplier_id: supplierId, location_id: location, note,
        expected_at: expected ? new Date(expected + "T12:00").getTime() : 0,
        lines: rows.map((l, i) => ({ variant_id: l.variant_id, qty_ordered_milli: parsed[i].qty, unit_cost_minor: parsed[i].cost })),
        submit,
      })
      toast.success(submit ? "Order placed" : "Draft saved")
      qc.invalidateQueries({ queryKey: ["bo"] })
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{order ? `Edit ${order.po_no}` : "New purchase order"}</DialogTitle>
          <DialogDescription>Save as a draft to finish later, or place it once it's ready to send.</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-3 gap-3">
          <div className="space-y-1.5">
            <Label>Supplier</Label>
            <div className="flex gap-1.5">
              <Select value={supplierId} onValueChange={setSupplierId}>
                <SelectTrigger className="h-10 w-full">
                  <SelectValue placeholder="Choose a supplier" />
                </SelectTrigger>
                <SelectContent>
                  {(suppliers.data ?? []).filter((s) => s.active || s.id === supplierId).map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button variant="outline" size="icon" className="size-10 shrink-0" title="New supplier" onClick={onNewSupplier}>
                <Plus />
              </Button>
            </div>
          </div>
          <LocationSelect value={location} onChange={setLocation} />
          <div className="space-y-1.5">
            <Label>Expected delivery</Label>
            <Input type="date" value={expected} onChange={(e) => setExpected(e.target.value)} className="h-10" />
          </div>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex-1">
            <ItemPicker location={location} onPick={(r) => add(r)} exclude={exclude} autoFocus={!order} />
          </div>
          <Button variant="outline" className="h-10" onClick={addLowStock}>
            Add low-stock items
          </Button>
        </div>
        <div className="min-h-[160px] flex-1 overflow-y-auto rounded-lg border">
          {rows.length === 0 ? (
            <div className="grid h-full min-h-[160px] place-items-center text-sm text-muted-foreground">{lines === null ? <Loader2 className="size-5 animate-spin" /> : "Scan or search to add items"}</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40">
                  <TableHead className="pl-3">Item</TableHead>
                  <TableHead className="w-28 text-right">Quantity</TableHead>
                  <TableHead className="w-36 text-right">Unit cost</TableHead>
                  <TableHead className="w-32 text-right">Total</TableHead>
                  <TableHead className="w-10 pr-3" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((l, i) => (
                  <TableRow key={l.variant_id}>
                    <TableCell className="pl-3">
                      <div className="font-medium">{l.label}</div>
                      <div className="text-xs text-muted-foreground">
                        {l.sku}
                        {l.reorder > 0 && ` · ${formatQty(l.on_hand)} on hand, reorder at ${formatQty(l.reorder)}`}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Input value={l.qty} onChange={(e) => setLines((ls) => ls!.map((x, j) => (j === i ? { ...x, qty: e.target.value } : x)))} className={cn("h-9 text-right tabular", isNaN(parsed[i].qty) && "border-destructive")} />
                    </TableCell>
                    <TableCell>
                      <Input value={l.cost} onChange={(e) => setLines((ls) => ls!.map((x, j) => (j === i ? { ...x, cost: e.target.value } : x)))} className={cn("h-9 text-right tabular", isNaN(parsed[i].cost) && "border-destructive")} />
                    </TableCell>
                    <TableCell className="text-right tabular">{isNaN(parsed[i].qty) || isNaN(parsed[i].cost) ? "–" : fmt(Math.round((parsed[i].qty * parsed[i].cost) / 1000))}</TableCell>
                    <TableCell className="pr-3">
                      <Button variant="ghost" size="icon-sm" onClick={() => setLines((ls) => ls!.filter((_, j) => j !== i))}>
                        <Trash2 />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
        <div className="grid grid-cols-[1fr_auto] items-end gap-4">
          <div className="space-y-1.5">
            <Label>Note to supplier / internal note</Label>
            <Input value={note} onChange={(e) => setNote(e.target.value)} className="h-10" />
          </div>
          <div className="text-right">
            <div className="text-xs text-muted-foreground uppercase">Order value</div>
            <div className="text-2xl font-semibold tabular">{fmt(total)}</div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Close
          </Button>
          <Button variant="outline" onClick={() => save(false)} disabled={!valid || busy}>
            Save draft
          </Button>
          <Button onClick={() => save(true)} disabled={!valid || busy}>
            {busy ? <Loader2 className="animate-spin" /> : <Send />} Place order
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface RecvLine {
  po_line_id: string
  variant_id: string
  label: string
  sku: string
  outstanding: number | null
  qty: string
  cost: string
  price_minor: number
  new_price: string
}

function ReceiveDialog({ order, onClose }: { order: PurchaseOrder | null; onClose: () => void }) {
  const state = useLoadedState()
  const qc = useQueryClient()
  const { fmt, currency } = useMoney()
  const suppliers = useSuppliers()
  const full = useQuery({ queryKey: ["bo", "po", order?.id], queryFn: () => hub.purchaseOrder(order!.id), enabled: !!order })
  const [supplierId, setSupplierId] = useState(order?.supplier_id ?? "")
  const [location, setLocation] = useState(order?.location_id || state.pairing.location_id)
  const [ref, setRef] = useState("")
  const [note, setNote] = useState("")
  const [closePO, setClosePO] = useState(false)
  const [lines, setLines] = useState<RecvLine[] | null>(order ? null : [])
  const [busy, setBusy] = useState(false)
  const toMoney = (minor: number) => (minor / 10 ** currency.digits).toFixed(currency.digits)

  if (order && lines === null && full.data) {
    setLines(
      (full.data.lines ?? []).map((l) => {
        const out = Math.max(0, l.qty_ordered_milli - l.qty_received_milli)
        return { po_line_id: l.id, variant_id: l.variant_id, label: itemLabel(l), sku: l.sku, outstanding: out, qty: out ? formatQty(out) : "", cost: toMoney(l.unit_cost_minor), price_minor: 0, new_price: "" }
      }),
    )
  }
  const rows = useMemo(() => lines ?? [], [lines])
  const exclude = useMemo(() => new Set(rows.filter((l) => !l.po_line_id).map((l) => l.variant_id)), [rows])
  const parsed = rows.map((l) => ({
    qty: l.qty.trim() === "" || l.qty.trim() === "0" ? 0 : parseQty(l.qty),
    cost: parseMoney(l.cost, currency),
    price: l.new_price.trim() === "" ? 0 : parseMoney(l.new_price, currency),
  }))
  const total = parsed.reduce((s, p) => s + (p.qty > 0 && !isNaN(p.cost) ? Math.round((p.qty * p.cost) / 1000) : 0), 0)
  const anyQty = parsed.some((p) => p.qty > 0)
  const valid = (order || supplierId) && anyQty && parsed.every((p) => !isNaN(p.qty) && !isNaN(p.cost) && !isNaN(p.price))
  const short = rows.some((l, i) => l.outstanding !== null && parsed[i].qty < (l.outstanding ?? 0))
  const update = (i: number, patch: Partial<RecvLine>) => setLines((ls) => ls!.map((x, j) => (j === i ? { ...x, ...patch } : x)))

  async function submit() {
    setBusy(true)
    try {
      await hub.receiveGoods({
        po_id: order?.id ?? "", supplier_id: order ? "" : supplierId, location_id: location, supplier_ref: ref, note, close_po: closePO,
        lines: rows
          .map((l, i) => ({ po_line_id: l.po_line_id, variant_id: l.variant_id, qty_milli: parsed[i].qty, unit_cost_minor: parsed[i].cost, new_price_minor: parsed[i].price }))
          .filter((l) => l.qty_milli > 0),
      })
      toast.success(`Delivery booked into stock (${fmt(total)})`)
      qc.invalidateQueries({ queryKey: ["bo"] })
      qc.invalidateQueries({ queryKey: ["products"] })
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{order ? `Receive delivery for ${order.po_no}` : "Receive goods without an order"}</DialogTitle>
          <DialogDescription>
            Enter what actually arrived and what it cost. Stock goes up straight away; item costs become the average of old and new stock.
          </DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-3 gap-3">
          {order ? (
            <div className="space-y-1.5">
              <Label>Supplier</Label>
              <div className="flex h-10 items-center rounded-md border bg-muted/40 px-3 text-sm">{order.supplier_name}</div>
            </div>
          ) : (
            <div className="space-y-1.5">
              <Label>Supplier</Label>
              <Select value={supplierId} onValueChange={setSupplierId}>
                <SelectTrigger className="h-10 w-full">
                  <SelectValue placeholder="Choose a supplier" />
                </SelectTrigger>
                <SelectContent>
                  {(suppliers.data ?? []).filter((s) => s.active).map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          <LocationSelect value={location} onChange={setLocation} />
          <div className="space-y-1.5">
            <Label>Supplier invoice / delivery note no.</Label>
            <Input value={ref} onChange={(e) => setRef(e.target.value)} className="h-10" />
          </div>
        </div>
        {!order && <ItemPicker location={location} exclude={exclude} autoFocus onPick={(r) => setLines((ls) => [...(ls ?? []), { po_line_id: "", variant_id: r.variant_id, label: itemLabel(r), sku: r.sku, outstanding: null, qty: "1", cost: toMoney(r.cost_minor), price_minor: r.price_minor, new_price: "" }])} />}
        <div className="min-h-[160px] flex-1 overflow-y-auto rounded-lg border">
          {rows.length === 0 ? (
            <div className="grid h-full min-h-[160px] place-items-center text-sm text-muted-foreground">{lines === null ? <Loader2 className="size-5 animate-spin" /> : "Scan or search to add what arrived"}</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40">
                  <TableHead className="pl-3">Item</TableHead>
                  {order && <TableHead className="w-24 text-right">Outstanding</TableHead>}
                  <TableHead className="w-28 text-right">Received</TableHead>
                  <TableHead className="w-32 text-right">Unit cost</TableHead>
                  <TableHead className="w-32 text-right" title="Leave blank to keep the current selling price">
                    New selling price
                  </TableHead>
                  {!order && <TableHead className="w-10 pr-3" />}
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((l, i) => (
                  <TableRow key={l.po_line_id || l.variant_id}>
                    <TableCell className="pl-3">
                      <div className="font-medium">{l.label}</div>
                      <div className="text-xs text-muted-foreground">
                        {l.sku}
                        {l.price_minor > 0 && ` · sells at ${fmt(l.price_minor)}`}
                      </div>
                    </TableCell>
                    {order && <TableCell className="text-right text-muted-foreground tabular">{formatQty(l.outstanding ?? 0)}</TableCell>}
                    <TableCell>
                      <Input value={l.qty} placeholder="0" onChange={(e) => update(i, { qty: e.target.value })} className={cn("h-9 text-right tabular", isNaN(parsed[i].qty) && "border-destructive")} />
                    </TableCell>
                    <TableCell>
                      <Input value={l.cost} onChange={(e) => update(i, { cost: e.target.value })} className={cn("h-9 text-right tabular", isNaN(parsed[i].cost) && "border-destructive")} />
                    </TableCell>
                    <TableCell>
                      <Input value={l.new_price} placeholder="unchanged" onChange={(e) => update(i, { new_price: e.target.value })} className={cn("h-9 text-right tabular", isNaN(parsed[i].price) && "border-destructive")} />
                    </TableCell>
                    {!order && (
                      <TableCell className="pr-3">
                        <Button variant="ghost" size="icon-sm" onClick={() => setLines((ls) => ls!.filter((_, j) => j !== i))}>
                          <Trash2 />
                        </Button>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
        <div className="grid grid-cols-[1fr_auto] items-end gap-4">
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label>Note</Label>
              <Input value={note} onChange={(e) => setNote(e.target.value)} className="h-10" placeholder="e.g. 2 cartons damaged, returned to driver" />
            </div>
            {order && short && (
              <label className="flex items-center gap-2 text-sm">
                <Switch checked={closePO} onCheckedChange={setClosePO} /> The rest isn't coming: close the order after this delivery
              </label>
            )}
          </div>
          <div className="text-right">
            <div className="text-xs text-muted-foreground uppercase">Delivery value</div>
            <div className="text-2xl font-semibold tabular">{fmt(total)}</div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={!valid || busy}>
            {busy ? <Loader2 className="animate-spin" /> : <PackageCheck />} Book into stock
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function DeliveriesTab() {
  const { fmt } = useMoney()
  const [openId, setOpenId] = useState<string | null>(null)
  const list = useQuery({ queryKey: ["bo", "receipts"], queryFn: () => hub.receipts() })
  return (
    <>
      <QueryError error={list.error} />
      {list.data?.length === 0 ? (
        <EmptyState icon={<Truck />} title="No deliveries yet" body="Goods you receive, with or without an order, are listed here." />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">Delivery</TableHead>
                <TableHead>Supplier</TableHead>
                <TableHead>Order</TableHead>
                <TableHead>Supplier ref</TableHead>
                <TableHead>Received</TableHead>
                <TableHead className="pr-4 text-right">Value</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(list.data ?? []).map((g) => (
                <TableRow key={g.id} className="cursor-pointer" onClick={() => setOpenId(g.id)}>
                  <TableCell className="pl-4 font-medium tabular">{g.grn_no}</TableCell>
                  <TableCell>{g.supplier_name || <span className="text-muted-foreground">–</span>}</TableCell>
                  <TableCell className="text-muted-foreground tabular">{g.po_no || "Direct"}</TableCell>
                  <TableCell className="text-muted-foreground">{g.supplier_ref}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatDateTime(g.created_at)} · {g.received_by}
                  </TableCell>
                  <TableCell className="pr-4 text-right font-medium tabular">{fmt(g.total_minor)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      <DeliverySheet id={openId} onClose={() => setOpenId(null)} />
    </>
  )
}

function DeliverySheet({ id, onClose }: { id: string | null; onClose: () => void }) {
  const { fmt } = useMoney()
  const g = useQuery({ queryKey: ["bo", "receipt", id], queryFn: () => hub.receipt(id!), enabled: !!id })
  const d: GoodsReceipt | undefined = g.data
  return (
    <Sheet open={!!id} onOpenChange={(v) => !v && onClose()}>
      <SheetContent className="w-[560px] gap-0 sm:max-w-[560px]">
        <SheetHeader className="border-b">
          <SheetTitle>Delivery {d?.grn_no}</SheetTitle>
          <SheetDescription>{d && `${d.supplier_name || "No supplier"} · ${d.location_name} · ${formatDateTime(d.created_at)} by ${d.received_by}`}</SheetDescription>
        </SheetHeader>
        <div className="flex-1 space-y-4 overflow-y-auto p-5">
          <QueryError error={g.error} />
          {d && (
            <>
              {(d.po_no || d.supplier_ref || d.note) && (
                <div className="space-y-1 text-sm">
                  {d.po_no && <div>Against order {d.po_no}</div>}
                  {d.supplier_ref && <div>Supplier ref {d.supplier_ref}</div>}
                  {d.note && <div className="text-muted-foreground">{d.note}</div>}
                </div>
              )}
              <div className="overflow-hidden rounded-lg border">
                <Table>
                  <TableHeader>
                    <TableRow className="bg-muted/40">
                      <TableHead className="pl-3">Item</TableHead>
                      <TableHead className="text-right">Qty</TableHead>
                      <TableHead className="text-right">Unit cost</TableHead>
                      <TableHead className="pr-3 text-right">Total</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(d.lines ?? []).map((l) => (
                      <TableRow key={l.id}>
                        <TableCell className="pl-3">
                          <div className="font-medium">{itemLabel(l)}</div>
                          <div className="text-xs text-muted-foreground">{l.sku}</div>
                        </TableCell>
                        <TableCell className="text-right tabular">{formatQty(l.qty_milli)}</TableCell>
                        <TableCell className="text-right tabular">{fmt(l.unit_cost_minor)}</TableCell>
                        <TableCell className="pr-3 text-right tabular">{fmt(l.total_minor)}</TableCell>
                      </TableRow>
                    ))}
                    <TableRow>
                      <TableCell colSpan={3} className="pl-3 font-medium">
                        Total
                      </TableCell>
                      <TableCell className="pr-3 text-right font-semibold tabular">{fmt(d.total_minor)}</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}

function SuppliersTab({ onEdit }: { onEdit: (s: Supplier | "new") => void }) {
  const { fmt } = useMoney()
  const list = useSuppliers()
  return (
    <>
      <QueryError error={list.error} />
      {list.data?.length === 0 ? (
        <EmptyState icon={<Truck />} title="No suppliers yet" body="Add the companies you buy from." action={<Button onClick={() => onEdit("new")}>Add a supplier</Button>} />
      ) : (
        <>
          <Button variant="outline" onClick={() => onEdit("new")}>
            <Plus /> New supplier
          </Button>
          <div className="overflow-hidden rounded-xl border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40">
                  <TableHead className="pl-4">Supplier</TableHead>
                  <TableHead>Contact</TableHead>
                  <TableHead>Phone</TableHead>
                  <TableHead className="text-right">Open orders</TableHead>
                  <TableHead className="pr-4 text-right">Bought to date</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(list.data ?? []).map((s) => (
                  <TableRow key={s.id} className={cn("cursor-pointer", !s.active && "opacity-50")} onClick={() => onEdit(s)}>
                    <TableCell className="pl-4 font-medium">
                      {s.name}
                      {!s.active && <Badge variant="outline" className="ml-2">Inactive</Badge>}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {s.contact_name}
                      {s.email && <div className="text-xs">{s.email}</div>}
                    </TableCell>
                    <TableCell className="text-muted-foreground tabular">{s.phone}</TableCell>
                    <TableCell className="text-right tabular">{s.open_orders || "–"}</TableCell>
                    <TableCell className="pr-4 text-right font-medium tabular">{fmt(s.received_minor)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </>
      )}
    </>
  )
}

function SupplierDialog({ supplier, onClose }: { supplier: Supplier | null; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<Partial<Supplier>>(supplier ?? { active: true })
  const [busy, setBusy] = useState(false)
  const set = (k: keyof Supplier) => (e: React.ChangeEvent<HTMLInputElement>) => setF((x) => ({ ...x, [k]: e.target.value }))
  async function save(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await hub.saveSupplier(f)
      toast.success("Supplier saved")
      qc.invalidateQueries({ queryKey: ["bo", "suppliers"] })
      onClose()
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={save} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{supplier ? "Edit supplier" : "New supplier"}</DialogTitle>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label>Company name</Label>
            <Input autoFocus value={f.name ?? ""} onChange={set("name")} className="h-10" />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Contact person</Label>
              <Input value={f.contact_name ?? ""} onChange={set("contact_name")} className="h-10" />
            </div>
            <div className="space-y-1.5">
              <Label>Phone</Label>
              <Input value={f.phone ?? ""} onChange={set("phone")} className="h-10" />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label>Email</Label>
            <Input type="email" value={f.email ?? ""} onChange={set("email")} className="h-10" />
          </div>
          <div className="space-y-1.5">
            <Label>Address</Label>
            <Input value={f.address ?? ""} onChange={set("address")} className="h-10" />
          </div>
          <div className="space-y-1.5">
            <Label>Notes (terms, account number…)</Label>
            <Input value={f.notes ?? ""} onChange={set("notes")} className="h-10" />
          </div>
          {supplier && (
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={!!f.active} onCheckedChange={(v) => setF((x) => ({ ...x, active: v }))} /> Active
            </label>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!f.name?.trim() || busy}>
              {busy && <Loader2 className="animate-spin" />} Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
