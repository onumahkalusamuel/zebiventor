import { useEffect, useMemo, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { ArrowDownToLine, ArrowUpFromLine, Loader2, Lock, Printer, ReceiptText } from "lucide-react"
import { toast } from "sonner"
import { useApproval } from "@/components/approval"
import { Page, PageHeader, StatCard } from "@/components/page"
import { OpenShiftCard, useShift } from "@/components/shift-gate"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api, errorMessage } from "@/lib/api"
import { atLeast, formatDateTime, formatQty, parseMoney } from "@/lib/format"
import { useLoadedState, useMoney } from "@/lib/state"
import type { Shift as ShiftT } from "@/lib/types"
import { cn } from "@/lib/utils"

export function Shift() {
  const shift = useShift()
  const [viewing, setViewing] = useState<string | null>(null)
  return (
    <Page>
      <PageHeader title="Shift" description="Cash drawer, takings and end-of-day report for this till." />
      <div className="space-y-8 px-8 pb-8">
        {shift.isLoading ? null : shift.data ? <CurrentShift shift={shift.data} onView={setViewing} /> : <div className="h-[420px]"><OpenShiftCard /></div>}
        <History onView={setViewing} />
      </div>
      <ReportSheet id={viewing} onClose={() => setViewing(null)} />
    </Page>
  )
}

function CurrentShift({ shift, onView }: { shift: ShiftT; onView: (id: string) => void }) {
  const state = useLoadedState()
  const { fmt } = useMoney()
  const s = shift.summary!
  const isManager = atLeast(state.user?.role, "manager")
  const movements = useQuery({ queryKey: ["movements", shift.id], queryFn: () => api.shiftMovements(shift.id) })
  const [move, setMove] = useState<null | "paid_in" | "paid_out">(null)
  const [closing, setClosing] = useState(false)

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between rounded-xl border bg-card p-4">
        <div className="flex items-center gap-3">
          <span className="relative flex size-2.5">
            <span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-60" />
            <span className="relative inline-flex size-2.5 rounded-full bg-emerald-500" />
          </span>
          <div>
            <div className="font-medium">Shift open since {formatDateTime(shift.opened_at)}</div>
            <div className="text-sm text-muted-foreground">
              Opened by {shift.opened_by_name} with a float of {fmt(shift.opening_float_minor)}
            </div>
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setMove("paid_in")}>
            <ArrowDownToLine /> Paid in
          </Button>
          <Button variant="outline" onClick={() => setMove("paid_out")}>
            <ArrowUpFromLine /> Paid out
          </Button>
          <Button variant="outline" onClick={() => onView(shift.id)}>
            <ReceiptText /> X report
          </Button>
          <Button onClick={() => setClosing(true)}>
            <Lock /> Close shift
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-4 gap-3">
        <StatCard label="Sales" value={s.sales_count} sub={`${formatQty(s.items_milli)} items`} />
        <StatCard label="Net takings" value={fmt(s.net_sales_minor - s.returns_minor)} sub={s.returns_count ? `after ${s.returns_count} refund(s)` : "no refunds"} />
        {s.account_payments_count ? (
          <StatCard label="Paid on accounts" value={fmt(s.account_payments_minor)} sub={`${s.account_payments_count} payment(s) · ${fmt(s.discount_minor)} discounts`} />
        ) : (
          <StatCard label="Discounts given" value={fmt(s.discount_minor)} />
        )}
        {isManager ? (
          <StatCard label="Cash expected in drawer" value={fmt(s.expected_cash_minor)} sub="float + cash sales & account payments − refunds ± paid in/out" />
        ) : (
          <StatCard label="Cash in drawer" value="Counted at close" sub="Blind count for cashiers" />
        )}
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="overflow-hidden rounded-xl border bg-card">
          <div className="border-b px-4 py-3 text-sm font-medium">By payment method</div>
          <Table>
            <TableBody>
              {(s.by_method ?? []).map((m) => (
                <TableRow key={m.method_name}>
                  <TableCell className="pl-4">{m.method_name}</TableCell>
                  <TableCell className="text-right text-muted-foreground tabular">
                    {[m.refunds_minor ? `−${fmt(m.refunds_minor)} refunds` : "", m.payments_minor ? `+${fmt(m.payments_minor)} on account` : ""].filter(Boolean).join(" · ")}
                  </TableCell>
                  <TableCell className="pr-4 text-right font-semibold tabular">{fmt(m.net_minor)}</TableCell>
                </TableRow>
              ))}
              {!s.by_method?.length && (
                <TableRow>
                  <TableCell className="py-6 text-center text-muted-foreground">No payments yet</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
        <div className="overflow-hidden rounded-xl border bg-card">
          <div className="border-b px-4 py-3 text-sm font-medium">Paid in / out</div>
          <Table>
            <TableBody>
              {(movements.data ?? []).map((m) => (
                <TableRow key={m.id}>
                  <TableCell className="pl-4">
                    <div>{m.reason}</div>
                    <div className="text-xs text-muted-foreground">
                      {m.user_name} · {formatDateTime(m.created_at)}
                    </div>
                  </TableCell>
                  <TableCell className={cn("pr-4 text-right font-semibold tabular", m.kind === "paid_out" ? "text-red-600" : "text-emerald-700")}>
                    {m.kind === "paid_out" ? "−" : "+"}
                    {fmt(m.amount_minor)}
                  </TableCell>
                </TableRow>
              ))}
              {movements.data?.length === 0 && (
                <TableRow>
                  <TableCell className="py-6 text-center text-muted-foreground">None this shift</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </div>
      <MoveDialog kind={move} onClose={() => setMove(null)} />
      {closing && <CloseDialog shift={shift} onClose={() => setClosing(false)} onClosed={onView} />}
    </div>
  )
}

function MoveDialog({ kind, onClose }: { kind: null | "paid_in" | "paid_out"; onClose: () => void }) {
  const qc = useQueryClient()
  const { currency, fmt } = useMoney()
  const requestApproval = useApproval()
  const [amount, setAmount] = useState("")
  const [reason, setReason] = useState("")
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    setAmount("")
    setReason("")
  }, [kind])
  const minor = parseMoney(amount, currency)

  async function save(e: React.FormEvent) {
    e.preventDefault()
    if (!kind) return
    let tok = ""
    if (kind === "paid_out") {
      const t = await requestApproval(`Take ${fmt(minor)} out of the drawer: ${reason}`, "paid_out")
      if (t === null) return
      tok = t
    }
    setBusy(true)
    try {
      await api.cashMovement(kind, minor, reason, tok)
      toast.success(kind === "paid_in" ? "Cash added to drawer" : "Cash taken out of drawer")
      qc.invalidateQueries()
      onClose()
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={!!kind} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <form onSubmit={save} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{kind === "paid_in" ? "Paid in" : "Paid out"}</DialogTitle>
            <DialogDescription>{kind === "paid_in" ? "Cash put into the drawer, e.g. change from the bank." : "Cash taken out, e.g. petty cash or a bank drop. Needs a manager."}</DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label>Amount ({currency.symbol})</Label>
            <Input autoFocus inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} className="h-12 text-center text-xl font-semibold tabular" />
          </div>
          <div className="space-y-1.5">
            <Label>Reason</Label>
            <Input value={reason} onChange={(e) => setReason(e.target.value)} placeholder={kind === "paid_in" ? "Change from bank" : "Fuel for generator"} />
          </div>
          <DialogFooter>
            <Button type="submit" className="w-full" disabled={busy || !(minor > 0) || !reason.trim()}>
              {busy && <Loader2 className="animate-spin" />} Record {fmt(isNaN(minor) ? 0 : minor)}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

const ngnDenoms = [1000, 500, 200, 100, 50, 20, 10, 5]

function CloseDialog({ shift, onClose, onClosed }: { shift: ShiftT; onClose: () => void; onClosed: (id: string) => void }) {
  const qc = useQueryClient()
  const state = useLoadedState()
  const { currency, fmt } = useMoney()
  const isManager = atLeast(state.user?.role, "manager")
  const unit = 10 ** currency.digits
  const useDenoms = currency.code === "NGN"
  const [counts, setCounts] = useState<Record<number, string>>({})
  const [coins, setCoins] = useState("")
  const [direct, setDirect] = useState("")
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)

  const counted = useMemo(() => {
    if (!useDenoms) return parseMoney(direct, currency)
    const notes = ngnDenoms.reduce((s, d) => s + d * unit * (parseInt(counts[d] || "0") || 0), 0)
    return notes + (parseMoney(coins, currency) || 0)
  }, [counts, coins, direct, useDenoms, unit, currency])
  const variance = counted - (shift.summary?.expected_cash_minor ?? 0)

  async function close() {
    setBusy(true)
    try {
      const res = await api.closeShift(counted, note)
      if (res.print_error) toast.warning("Shift closed, but the Z report didn’t print", { description: res.print_error })
      else toast.success("Shift closed")
      qc.invalidateQueries()
      onClose()
      onClosed(res.shift.id)
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Close shift</DialogTitle>
          <DialogDescription>Count all the cash in the drawer. The Z report prints automatically.</DialogDescription>
        </DialogHeader>
        {useDenoms ? (
          <div className="grid grid-cols-2 gap-x-6 gap-y-2">
            {ngnDenoms.map((d) => (
              <div key={d} className="flex items-center gap-3">
                <div className="w-20 text-sm font-medium tabular">{fmt(d * unit)}</div>
                <span className="text-muted-foreground">×</span>
                <Input
                  inputMode="numeric"
                  value={counts[d] ?? ""}
                  onChange={(e) => setCounts((c) => ({ ...c, [d]: e.target.value.replace(/\D/g, "") }))}
                  className="h-9 w-20 text-right tabular"
                  placeholder="0"
                />
              </div>
            ))}
            <div className="col-span-2 flex items-center gap-3">
              <div className="w-20 text-sm font-medium">Coins / other</div>
              <Input inputMode="decimal" value={coins} onChange={(e) => setCoins(e.target.value)} className="h-9 w-32 text-right tabular" placeholder="0.00" />
            </div>
          </div>
        ) : (
          <div className="space-y-1.5">
            <Label>Cash counted ({currency.symbol})</Label>
            <Input autoFocus inputMode="decimal" value={direct} onChange={(e) => setDirect(e.target.value)} className="h-12 text-center text-xl font-semibold tabular" />
          </div>
        )}
        <div className="rounded-xl bg-muted p-4">
          <div className="flex items-baseline justify-between">
            <span className="text-sm text-muted-foreground">Counted</span>
            <span className="text-2xl font-semibold tabular">{fmt(isNaN(counted) ? 0 : counted)}</span>
          </div>
          {isManager && !isNaN(counted) && (
            <div className="mt-1 flex items-baseline justify-between text-sm">
              <span className="text-muted-foreground">Expected {fmt(shift.summary?.expected_cash_minor ?? 0)}</span>
              <span className={cn("font-semibold tabular", variance < 0 ? "text-red-600" : variance > 0 ? "text-amber-600" : "text-emerald-700")}>
                {variance === 0 ? "Balanced" : `${variance > 0 ? "Over" : "Short"} ${fmt(Math.abs(variance))}`}
              </span>
            </div>
          )}
        </div>
        <div className="space-y-1.5">
          <Label>Note (optional)</Label>
          <Input value={note} onChange={(e) => setNote(e.target.value)} placeholder="Anything the manager should know" />
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={close} disabled={busy || isNaN(counted)}>
            {busy ? <Loader2 className="animate-spin" /> : <Lock />} Close shift and print Z report
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function History({ onView }: { onView: (id: string) => void }) {
  const { fmt } = useMoney()
  const shifts = useQuery({ queryKey: ["shifts"], queryFn: api.listShifts })
  const closed = (shifts.data ?? []).filter((s) => s.status === "closed")
  if (!closed.length) return null
  return (
    <div className="space-y-3">
      <h2 className="font-medium">Previous shifts</h2>
      <div className="overflow-hidden rounded-xl border bg-card">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/40">
              <TableHead className="pl-4">Opened</TableHead>
              <TableHead>Closed</TableHead>
              <TableHead>By</TableHead>
              <TableHead className="text-right">Sales</TableHead>
              <TableHead className="text-right">Net takings</TableHead>
              <TableHead className="pr-4 text-right">Cash variance</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {closed.map((s) => {
              const v = s.summary?.variance_minor ?? 0
              return (
                <TableRow key={s.id} className="cursor-pointer" onClick={() => onView(s.id)}>
                  <TableCell className="pl-4">{formatDateTime(s.opened_at)}</TableCell>
                  <TableCell>{formatDateTime(s.closed_at)}</TableCell>
                  <TableCell className="text-muted-foreground">{s.closed_by_name}</TableCell>
                  <TableCell className="text-right tabular">{s.summary?.sales_count ?? 0}</TableCell>
                  <TableCell className="text-right font-medium tabular">{fmt((s.summary?.net_sales_minor ?? 0) - (s.summary?.returns_minor ?? 0))}</TableCell>
                  <TableCell className="pr-4 text-right">
                    {v === 0 ? (
                      <Badge variant="secondary">Balanced</Badge>
                    ) : (
                      <span className={cn("font-semibold tabular", v < 0 ? "text-red-600" : "text-amber-600")}>
                        {v > 0 ? "+" : "−"}
                        {fmt(Math.abs(v))}
                      </span>
                    )}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

function ReportSheet({ id, onClose }: { id: string | null; onClose: () => void }) {
  const preview = useQuery({ queryKey: ["shift-preview", id], queryFn: () => api.shiftPreview(id!), enabled: !!id })
  const [printing, setPrinting] = useState(false)
  return (
    <Sheet open={!!id} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-[480px] gap-0 sm:max-w-[480px]">
        <SheetHeader className="border-b">
          <SheetTitle>Shift report</SheetTitle>
          <SheetDescription>Exactly as it prints on the receipt printer.</SheetDescription>
        </SheetHeader>
        <div className="flex-1 overflow-y-auto bg-muted p-6">
          {preview.data ? <img src={preview.data} alt="" className="mx-auto w-full max-w-[360px] bg-white shadow-md [image-rendering:pixelated]" /> : <Loader2 className="mx-auto mt-20 size-5 animate-spin" />}
        </div>
        <div className="border-t p-4">
          <Button
            className="h-11 w-full"
            disabled={printing}
            onClick={async () => {
              setPrinting(true)
              try {
                await api.printShift(id!)
                toast.success("Report sent to printer")
              } catch (e) {
                toast.error(errorMessage(e))
              } finally {
                setPrinting(false)
              }
            }}
          >
            {printing ? <Loader2 className="animate-spin" /> : <Printer />} Print report
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
