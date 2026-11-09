// Returns without a receipt: pick the items, a manager approves, refund in
// money or as store credit on the customer's account.
import { useEffect, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeft, Loader2, Minus, Plus, RotateCcw, Search, ShieldCheck, Trash2, UserRound, Wallet } from "lucide-react"
import { toast } from "sonner"
import { useApproval } from "@/components/approval"
import { CustomerPicker } from "@/components/customer-picker"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { api, errorMessage } from "@/lib/api"
import { formatQty } from "@/lib/format"
import { useMoney } from "@/lib/state"
import { STORE_CREDIT, type Customer, type Item, type ReturnResult } from "@/lib/types"
import { cn } from "@/lib/utils"

interface Line {
  item: Item
  qty: number // milli
  today: number // refund price per unit, tax included
  unit: number // what we give back per unit
  restock: boolean
}

const reasons = ["Damaged", "Wrong item", "Changed mind", "Gift return", "Faulty / defective"]

export function NoReceiptReturn({ onBack, onDone }: { onBack: () => void; onDone: (r: ReturnResult) => void }) {
  const { fmt, currency } = useMoney()
  const requestApproval = useApproval()
  const methods = useQuery({ queryKey: ["payment-methods"], queryFn: api.paymentMethods })
  const [q, setQ] = useState("")
  const [lines, setLines] = useState<Line[]>([])
  const [customer, setCustomer] = useState<Customer | null>(null)
  const [picking, setPicking] = useState(false)
  const [reason, setReason] = useState("")
  const [methodId, setMethodId] = useState("")
  const [reference, setReference] = useState("")
  const [busy, setBusy] = useState(false)
  const search = useQuery({ queryKey: ["nr-search", q], queryFn: () => api.searchItems(q), enabled: q.trim().length > 1 })
  const inputRef = useRef<HTMLInputElement>(null)
  useEffect(() => inputRef.current?.focus(), [])

  const cash = methods.data?.find((m) => m.kind === "cash")?.id ?? ""
  const method = methodId || cash
  const total = lines.reduce((s, l) => s + Math.round((l.unit * l.qty) / 1000), 0)
  const m = methods.data?.find((x) => x.id === method)
  const needsRef = !!m?.requires_reference && method !== STORE_CREDIT
  const ready = total > 0 && reason.trim() && method && (method !== STORE_CREDIT || customer) && !(needsRef && !reference.trim())

  async function add(item: Item) {
    setQ("")
    inputRef.current?.focus()
    if (lines.some((l) => l.item.variant_id === item.variant_id)) {
      setLines((ls) => ls.map((l) => (l.item.variant_id === item.variant_id ? { ...l, qty: l.qty + 1000 } : l)))
      return
    }
    try {
      const today = await api.noReceiptPrice(item.variant_id)
      setLines((ls) => [...ls, { item, qty: 1000, today, unit: today, restock: item.track_stock }])
    } catch (e) {
      toast.error(errorMessage(e))
    }
  }
  const update = (id: string, f: (l: Line) => Line) => setLines((ls) => ls.map((l) => (l.item.variant_id === id ? f(l) : l)))

  async function complete() {
    if (!ready) return
    const tok = await requestApproval(`Refund ${fmt(total)} without a receipt`)
    if (tok === null) return
    setBusy(true)
    try {
      const res = await api.completeReturnNoReceipt({
        lines: lines.map((l) => ({ variant_id: l.item.variant_id, qty_milli: l.qty, unit_refund_minor: l.unit, restock: l.restock })),
        customer_id: customer?.id ?? "",
        refunds: [{ method_id: method, amount_minor: total, reference: reference.trim() }],
        reason,
        approval_token: tok,
      })
      if (res.print_error) toast.warning("Refund saved, but the slip didn’t print", { description: res.print_error })
      onDone(res)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid h-full grid-cols-[1fr_400px]">
      <section className="flex min-h-0 flex-col">
        <div className="flex items-center gap-3 border-b bg-card p-4">
          <Button variant="ghost" size="icon" onClick={onBack} aria-label="Back to receipt search">
            <ArrowLeft />
          </Button>
          <div className="relative flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-muted-foreground" />
            <Input
              ref={inputRef}
              value={q}
              onChange={(e) => setQ(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && search.data?.[0]) add(search.data[0])
              }}
              aria-label="Find the item being returned"
              placeholder="Scan or search the item being returned"
              className="h-14 rounded-xl pl-12 text-lg md:text-lg"
            />
            {q.trim().length > 1 && (search.data?.length ?? 0) > 0 && (
              <ul role="listbox" aria-label="Matching items" className="absolute top-full right-0 left-0 z-20 mt-1 max-h-80 overflow-y-auto rounded-xl border bg-popover p-1 shadow-lg">
                {search.data!.slice(0, 12).map((it) => (
                  <li key={it.variant_id}>
                    <button className="flex w-full items-center justify-between rounded-lg px-3 py-2 text-left hover:bg-muted focus-visible:bg-muted focus-visible:outline-none" onClick={() => add(it)}>
                      <span>
                        {it.name}
                        {it.variant_name && <span className="text-muted-foreground"> · {it.variant_name}</span>}
                      </span>
                      <span className="tabular text-muted-foreground">{fmt(it.price_minor)}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-6">
          <div className="mb-4 rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
            <b>No receipt.</b> A manager approves every return without a receipt. The refund can't be more than today's price; lower it if the item was bought on offer.
          </div>
          {lines.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-16 text-center text-muted-foreground">
              <RotateCcw className="size-12 opacity-30" />
              <div className="font-medium text-foreground">Add the items being returned</div>
            </div>
          ) : (
            <div className="overflow-hidden rounded-xl border bg-card">
              {lines.map((l) => {
                const id = l.item.variant_id
                const step = l.item.allow_fraction ? 100 : 1000
                return (
                  <div key={id} className="flex items-center gap-4 border-b p-4 last:border-b-0">
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-medium">
                        {l.item.name}
                        {l.item.variant_name && <span className="text-muted-foreground"> · {l.item.variant_name}</span>}
                      </div>
                      <div className="text-xs text-muted-foreground">Today {fmt(l.today)} each</div>
                    </div>
                    {l.item.track_stock && (
                      <label className="flex items-center gap-2 text-xs text-muted-foreground">
                        <Switch checked={l.restock} onCheckedChange={(v) => update(id, (x) => ({ ...x, restock: v }))} />
                        Back to stock
                      </label>
                    )}
                    <div className="flex items-center gap-1.5">
                      <Button variant="outline" size="icon-sm" aria-label={`One less ${l.item.name}`} disabled={l.qty <= step} onClick={() => update(id, (x) => ({ ...x, qty: x.qty - step }))}>
                        <Minus />
                      </Button>
                      <div className="w-12 text-center font-semibold tabular" aria-label="Quantity">
                        {formatQty(l.qty)}
                      </div>
                      <Button variant="outline" size="icon-sm" aria-label={`One more ${l.item.name}`} onClick={() => update(id, (x) => ({ ...x, qty: x.qty + step }))}>
                        <Plus />
                      </Button>
                    </div>
                    <div className="w-32">
                      <Input
                        aria-label={`Refund per unit for ${l.item.name}`}
                        className={cn("h-9 text-right tabular", l.unit > l.today && "border-destructive")}
                        defaultValue={(l.unit / 10 ** currency.digits).toFixed(currency.digits)}
                        onChange={(e) => {
                          const v = Math.round(parseFloat(e.target.value.replace(/,/g, "")) * 10 ** currency.digits)
                          update(id, (x) => ({ ...x, unit: isFinite(v) && v >= 0 ? Math.min(v, x.today) : x.unit }))
                        }}
                      />
                    </div>
                    <Button variant="ghost" size="icon-sm" aria-label={`Remove ${l.item.name}`} onClick={() => setLines((ls) => ls.filter((x) => x.item.variant_id !== id))}>
                      <Trash2 />
                    </Button>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </section>

      <aside className="flex min-h-0 flex-col border-l bg-card">
        <div className="flex h-14 items-center gap-2 border-b px-5 font-semibold">
          <ShieldCheck className="size-4 text-muted-foreground" /> Refund without receipt
        </div>
        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto p-5">
          <div className="space-y-2">
            <Label>Customer</Label>
            <Button variant="outline" className="w-full justify-start" onClick={() => setPicking(true)}>
              <UserRound /> {customer ? customer.name : "Add customer (needed for store credit)"}
            </Button>
          </div>
          <div className="space-y-2">
            <Label>Reason</Label>
            <div className="flex flex-wrap gap-1.5">
              {reasons.map((r) => (
                <button
                  key={r}
                  onClick={() => setReason(r)}
                  aria-pressed={reason === r}
                  className={cn("rounded-full border px-3 py-1 text-sm transition-colors", reason === r ? "border-primary bg-accent text-accent-foreground" : "hover:bg-muted")}
                >
                  {r}
                </button>
              ))}
            </div>
            <Input value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Or describe the reason" aria-label="Reason" />
          </div>
          <div className="space-y-2">
            <Label>Refund by</Label>
            <div className="grid grid-cols-2 gap-2">
              <button
                onClick={() => setMethodId(STORE_CREDIT)}
                aria-pressed={method === STORE_CREDIT}
                className={cn("flex items-center justify-center gap-1.5 rounded-lg border p-2.5 text-sm font-medium transition-colors", method === STORE_CREDIT ? "border-primary bg-accent text-accent-foreground" : "hover:bg-muted")}
              >
                <Wallet className="size-4" /> Store credit
              </button>
              {(methods.data ?? [])
                .filter((x) => x.kind !== "credit")
                .map((x) => (
                  <button
                    key={x.id}
                    onClick={() => setMethodId(x.id)}
                    aria-pressed={method === x.id}
                    className={cn("rounded-lg border p-2.5 text-sm font-medium transition-colors", method === x.id ? "border-primary bg-accent text-accent-foreground" : "hover:bg-muted")}
                  >
                    {x.name}
                  </button>
                ))}
            </div>
            {method === STORE_CREDIT && <p className="text-xs text-muted-foreground">Kept on the customer's account and taken off their next purchase.</p>}
            {needsRef && <Input value={reference} onChange={(e) => setReference(e.target.value)} placeholder="Reference (required)" aria-label="Reference" />}
          </div>
        </div>
        <div className="space-y-3 border-t bg-muted/30 p-5">
          <div className="flex items-baseline justify-between">
            <span className="text-sm font-medium text-muted-foreground">Refund total</span>
            <span className="text-3xl font-semibold tracking-tight tabular">{fmt(total)}</span>
          </div>
          <Button className="h-14 w-full text-lg" disabled={!ready || busy} onClick={complete}>
            {busy ? <Loader2 className="animate-spin" /> : <RotateCcw className="size-5" />}
            {method === STORE_CREDIT ? `Give ${fmt(total)} store credit` : `Refund ${fmt(total)}`}
          </Button>
        </div>
      </aside>
      <CustomerPicker
        open={picking}
        onClose={() => setPicking(false)}
        onPick={(c) => {
          setCustomer(c)
          setPicking(false)
        }}
      />
    </div>
  )
}
