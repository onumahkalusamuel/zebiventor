import { useEffect, useMemo, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { CheckCircle2, CloudOff, Loader2, Minus, Plus, Printer, ReceiptText, RotateCcw, ScanLine, Search, TriangleAlert } from "lucide-react"
import { toast } from "sonner"
import { useApproval } from "@/components/approval"
import { ShiftGate } from "@/components/shift-gate"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { api, errorMessage } from "@/lib/api"
import { formatDateTime, formatQty } from "@/lib/format"
import { useMoney } from "@/lib/state"
import type { ReturnableLine, ReturnableSale, ReturnResult } from "@/lib/types"
import { cn } from "@/lib/utils"

const reasons = ["Damaged", "Wrong item", "Changed mind", "Duplicate copy", "Faulty / defective"]

const mulDivRound = (a: number, b: number, c: number) => (c === 0 ? 0 : Math.floor((a * b + Math.floor(c / 2)) / c))

/** Mirrors terminal.RecordReturn: proportional refund, last units take the remainder. */
function refundFor(l: ReturnableLine, q: number) {
  if (q <= 0) return 0
  if (q === l.returnable_milli) return l.paid_minor - mulDivRound(l.paid_minor, l.returned_milli, l.qty_milli)
  return mulDivRound(l.paid_minor, q, l.qty_milli)
}

export function Returns() {
  return (
    <ShiftGate>
      <ReturnsScreen />
    </ShiftGate>
  )
}

function ReturnsScreen() {
  const { fmt } = useMoney()
  const requestApproval = useApproval()
  const methods = useQuery({ queryKey: ["payment-methods"], queryFn: api.paymentMethods })
  const [receipt, setReceipt] = useState("")
  const [sale, setSale] = useState<ReturnableSale | null>(null)
  const [looking, setLooking] = useState(false)
  const [qty, setQty] = useState<Record<string, number>>({})
  const [restock, setRestock] = useState<Record<string, boolean>>({})
  const [reason, setReason] = useState("")
  const [methodId, setMethodId] = useState("")
  const [reference, setReference] = useState("")
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState<ReturnResult | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => inputRef.current?.focus(), [])

  async function lookup(e?: React.FormEvent) {
    e?.preventDefault()
    if (!receipt.trim()) return
    setLooking(true)
    try {
      const s = await api.lookupForReturn(receipt)
      setSale(s)
      setQty({})
      setRestock(Object.fromEntries(s.lines.map((l) => [l.id, l.track_stock])))
      setReason("")
      setReference("")
      // Default refund method: how they paid, if a single method; else cash.
      const paidWith = s.sale.payments?.length === 1 ? s.sale.payments[0].method_id : ""
      const cash = methods.data?.find((m) => m.kind === "cash")?.id ?? ""
      setMethodId(methods.data?.some((m) => m.id === paidWith) ? paidWith : cash)
    } catch (err) {
      setSale(null)
      toast.error(errorMessage(err))
    } finally {
      setLooking(false)
    }
  }

  const total = useMemo(() => (sale ? sale.lines.reduce((s, l) => s + refundFor(l, qty[l.id] ?? 0), 0) : 0), [sale, qty])
  const method = methods.data?.find((m) => m.id === methodId)
  const canCredit = !!sale?.sale.customer_id
  const ready = total > 0 && reason.trim() && method && !(method.requires_reference && !reference.trim())

  async function complete() {
    if (!sale || !ready) return
    const tok = await requestApproval(`Refund ${fmt(total)} on receipt ${sale.sale.receipt_no}`, "refund")
    if (tok === null) return
    setBusy(true)
    try {
      const res = await api.completeReturn({
        receipt_no: sale.sale.receipt_no,
        reason,
        approval_token: tok,
        lines: sale.lines.filter((l) => (qty[l.id] ?? 0) > 0).map((l) => ({ sale_line_id: l.id, qty_milli: qty[l.id], restock: !!restock[l.id] })),
        refunds: [{ method_id: methodId, amount_minor: total, reference: reference.trim() }],
      })
      setDone(res)
      if (res.print_error) toast.warning("Refund saved, but the slip didn’t print", { description: res.print_error })
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  function reset() {
    setDone(null)
    setSale(null)
    setReceipt("")
    setTimeout(() => inputRef.current?.focus(), 0)
  }

  if (done) return <ReturnDone result={done} onNext={reset} />

  return (
    <div className="grid h-full grid-cols-[1fr_400px]">
      <section className="flex min-h-0 flex-col">
        <div className="border-b bg-card p-4">
          <form onSubmit={lookup} className="relative">
            <Search className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-muted-foreground" />
            <Input
              ref={inputRef}
              value={receipt}
              onChange={(e) => setReceipt(e.target.value)}
              placeholder="Scan the QR code on the receipt, or type the receipt number (e.g. T01-000042)"
              className="h-14 rounded-xl pr-32 pl-12 text-lg md:text-lg"
            />
            <Button type="submit" className="absolute top-2 right-2 h-10" disabled={looking || !receipt.trim()}>
              {looking ? <Loader2 className="animate-spin" /> : <ScanLine />} Find sale
            </Button>
          </form>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-6">
          {!sale ? (
            <div className="flex flex-col items-center gap-3 py-24 text-center text-muted-foreground">
              <RotateCcw className="size-12 opacity-30" />
              <div>
                <div className="font-medium text-foreground">Find the original sale</div>
                <div className="text-sm">Receipts from this till work offline; other tills’ receipts need the hub.</div>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex items-start justify-between rounded-xl border bg-card p-4">
                <div>
                  <div className="flex items-center gap-2 text-lg font-semibold">
                    <ReceiptText className="size-5 text-primary" />
                    {sale.sale.receipt_no}
                    <Badge variant="secondary">{sale.source === "hub" ? "From another till" : "This till"}</Badge>
                  </div>
                  <div className="mt-1 text-sm text-muted-foreground">
                    {formatDateTime(sale.sale.created_at)} · served by {sale.sale.user_name}
                    {sale.sale.customer_name && ` · ${sale.sale.customer_name}`}
                  </div>
                  <div className="mt-1 text-sm text-muted-foreground">Paid by {(sale.sale.payments ?? []).map((p) => `${p.method_name} ${fmt(p.amount_minor)}`).join(", ")}</div>
                </div>
                <div className="text-right">
                  <div className="text-xs text-muted-foreground">Sale total</div>
                  <div className="text-xl font-semibold tabular">{fmt(sale.sale.total_minor)}</div>
                </div>
              </div>
              {sale.partial && (
                <div className="flex items-center gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
                  <CloudOff className="size-4 text-amber-700" /> Hub offline: returns made on other tills for this receipt can’t be checked right now.
                </div>
              )}
              <div className="overflow-hidden rounded-xl border bg-card">
                {sale.lines.map((l) => {
                  const q = qty[l.id] ?? 0
                  const set = (v: number) => setQty((m) => ({ ...m, [l.id]: Math.min(Math.max(0, v), l.returnable_milli) }))
                  return (
                    <div key={l.id} className={cn("flex items-center gap-4 border-b p-4 last:border-b-0", l.returnable_milli === 0 && "opacity-50")}>
                      <div className="min-w-0 flex-1">
                        <div className="truncate font-medium">
                          {l.product_name}
                          {l.variant_name && <span className="text-muted-foreground"> · {l.variant_name}</span>}
                        </div>
                        <div className="text-xs text-muted-foreground tabular">
                          Bought {formatQty(l.qty_milli)} · paid {fmt(l.paid_minor)}
                          {l.returned_milli > 0 && ` · ${formatQty(l.returned_milli)} already returned`}
                        </div>
                      </div>
                      {l.track_stock && q > 0 && (
                        <label className="flex items-center gap-2 text-xs text-muted-foreground">
                          <Switch checked={!!restock[l.id]} onCheckedChange={(v) => setRestock((m) => ({ ...m, [l.id]: v }))} />
                          Back to stock
                        </label>
                      )}
                      <div className="flex items-center gap-1.5">
                        <Button variant="outline" size="icon-sm" disabled={q <= 0} onClick={() => set(q - 1000)}>
                          <Minus />
                        </Button>
                        <div className="w-10 text-center font-semibold tabular">{formatQty(q)}</div>
                        <Button variant="outline" size="icon-sm" disabled={q >= l.returnable_milli} onClick={() => set(q + 1000)}>
                          <Plus />
                        </Button>
                      </div>
                      <div className="w-28 text-right font-semibold tabular">{q > 0 ? fmt(refundFor(l, q)) : "–"}</div>
                    </div>
                  )
                })}
              </div>
              {sale.lines.every((l) => l.returnable_milli === 0) && (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <TriangleAlert className="size-4" /> Everything on this receipt has already been returned.
                </div>
              )}
            </div>
          )}
        </div>
      </section>

      <aside className="flex min-h-0 flex-col border-l bg-card">
        <div className="flex h-14 items-center border-b px-5 font-semibold">Refund</div>
        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto p-5">
          <div className="space-y-2">
            <Label>Reason</Label>
            <div className="flex flex-wrap gap-1.5">
              {reasons.map((r) => (
                <button
                  key={r}
                  onClick={() => setReason(r)}
                  className={cn("rounded-full border px-3 py-1 text-sm transition-colors", reason === r ? "border-primary bg-accent text-accent-foreground" : "hover:bg-muted")}
                >
                  {r}
                </button>
              ))}
            </div>
            <Input value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Or describe the reason" />
          </div>
          <div className="space-y-2">
            <Label>Refund by</Label>
            <div className="grid grid-cols-2 gap-2">
              {(methods.data ?? [])
                .filter((m) => m.kind !== "credit" || canCredit)
                .map((m) => (
                  <button
                    key={m.id}
                    onClick={() => setMethodId(m.id)}
                    className={cn("rounded-lg border p-2.5 text-sm font-medium transition-colors", methodId === m.id ? "border-primary bg-accent text-accent-foreground" : "hover:bg-muted")}
                  >
                    {m.kind === "credit" ? "Customer account" : m.name}
                  </button>
                ))}
            </div>
            {method && method.kind !== "cash" && method.kind !== "credit" && (
              <Input value={reference} onChange={(e) => setReference(e.target.value)} placeholder={method.requires_reference ? "Reference (required)" : "Reference (optional)"} />
            )}
          </div>
        </div>
        <div className="space-y-3 border-t bg-muted/30 p-5">
          <div className="flex items-baseline justify-between">
            <span className="text-sm font-medium text-muted-foreground">Refund total</span>
            <span className="text-3xl font-semibold tracking-tight tabular">{fmt(total)}</span>
          </div>
          <Button className="h-14 w-full text-lg" disabled={!ready || busy} onClick={complete}>
            {busy ? <Loader2 className="animate-spin" /> : <RotateCcw className="size-5" />}
            Refund {fmt(total)}
          </Button>
        </div>
      </aside>
    </div>
  )
}

function ReturnDone({ result, onNext }: { result: ReturnResult; onNext: () => void }) {
  const { fmt } = useMoney()
  const preview = useQuery({ queryKey: ["return-preview", result.return.id], queryFn: () => api.returnPreview(result.return.id, false) })
  const [printing, setPrinting] = useState(false)
  return (
    <div className="grid h-full grid-cols-[1fr_420px]">
      <div className="flex flex-col items-center justify-center gap-5 p-10 text-center">
        <div className="grid size-16 place-items-center rounded-full bg-emerald-500/12">
          <CheckCircle2 className="size-9 text-emerald-600" />
        </div>
        <div>
          <h2 className="text-2xl font-semibold">Refund complete</h2>
          <p className="mt-1 text-muted-foreground">
            {result.return.return_no} against {result.return.sale_receipt_no}
          </p>
        </div>
        <div className="rounded-2xl border-2 border-primary/30 bg-accent/50 px-10 py-5">
          <div className="text-xs font-medium tracking-wide text-accent-foreground uppercase">Give back</div>
          <div className="text-5xl font-semibold tracking-tight text-accent-foreground tabular">{fmt(result.return.total_minor)}</div>
        </div>
        <div className="flex w-full max-w-sm gap-2">
          <Button
            variant="outline"
            className="h-12 flex-1"
            disabled={printing}
            onClick={async () => {
              setPrinting(true)
              try {
                await api.printReturn(result.return.id, result.printed)
                toast.success("Refund slip sent to printer")
              } catch (e) {
                toast.error(errorMessage(e))
              } finally {
                setPrinting(false)
              }
            }}
          >
            {printing ? <Loader2 className="animate-spin" /> : <Printer />} {result.printed ? "Print again" : "Print slip"}
          </Button>
          <Button autoFocus className="h-12 flex-1 text-base" onClick={onNext}>
            Next return
          </Button>
        </div>
      </div>
      <div className="overflow-y-auto bg-muted p-5">
        {preview.data && <img src={preview.data} alt="Refund slip" className="mx-auto w-full bg-white shadow-md [image-rendering:pixelated]" />}
      </div>
    </div>
  )
}
