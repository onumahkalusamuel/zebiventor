// Money owed to suppliers: bills (their invoices), payments, statements.
import { useMemo, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Ban, Banknote, FileText, Loader2, Pencil, Plus, ReceiptText } from "lucide-react"
import { toast } from "sonner"
import { Chips, PrintButton, QueryError, printDocument, toDateInput } from "@/components/backoffice"
import { EmptyState, StatCard } from "@/components/page"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { errorMessage } from "@/lib/api"
import { formatDate, formatDateTime, parseMoney } from "@/lib/format"
import { hub } from "@/lib/hub"
import { useMoney } from "@/lib/state"
import type { GoodsReceipt, Supplier, SupplierBill } from "@/lib/types"
import { cn } from "@/lib/utils"

const DAY = 86_400_000
const PAY_METHODS = ["Bank transfer", "Cash", "Cheque", "Card", "Other"]

function startOfToday() {
  const d = new Date()
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
}

function fromDateInput(s: string) {
  const [y, m, d] = s.split("-").map(Number)
  return new Date(y, (m || 1) - 1, d || 1).getTime()
}

function useSupplierList() {
  return useQuery({ queryKey: ["bo", "suppliers"], queryFn: hub.suppliers })
}

export function BillBadge({ bill }: { bill: SupplierBill }) {
  let label = "Unpaid"
  let cls = "border-sky-500/40 text-sky-700 dark:text-sky-400"
  if (bill.status === "paid") {
    label = "Paid"
    cls = "border-emerald-500/40 text-emerald-700 dark:text-emerald-400"
  } else if (bill.status === "void") {
    label = "Void"
    cls = "text-muted-foreground line-through"
  } else if (bill.due_date < startOfToday()) {
    label = bill.paid_minor > 0 ? "Part paid · overdue" : "Overdue"
    cls = "border-destructive/50 text-destructive"
  } else if (bill.paid_minor > 0) {
    label = "Part paid"
    cls = "border-amber-500/40 text-amber-700 dark:text-amber-400"
  }
  return (
    <Badge variant="outline" className={cls}>
      {label}
    </Badge>
  )
}

function dueText(b: SupplierBill) {
  if (b.status !== "open") return formatDate(b.due_date)
  const days = Math.round((b.due_date - startOfToday()) / DAY)
  if (days < 0) return `${formatDate(b.due_date)} (${-days} day${days === -1 ? "" : "s"} late)`
  if (days === 0) return `${formatDate(b.due_date)} (today)`
  return `${formatDate(b.due_date)} (in ${days} day${days === 1 ? "" : "s"})`
}

// ---------- bills ----------

type BillFilter = "open" | "overdue" | "paid" | ""

export function BillsTab({ onBill, onPay }: { onBill: () => void; onPay: (supplierId?: string, billIds?: string[]) => void }) {
  const { fmt } = useMoney()
  const [filter, setFilter] = useState<BillFilter>("open")
  const [openId, setOpenId] = useState<string | null>(null)
  const list = useQuery({ queryKey: ["bo", "bills", filter], queryFn: () => hub.bills("", filter) })
  const ap = useQuery({ queryKey: ["bo", "payables"], queryFn: hub.payables })
  const t = ap.data?.totals
  const overdue = t ? t.days_1_30_minor + t.days_31_60_minor + t.days_61_90_minor + t.over_90_minor : 0
  return (
    <>
      <div className="grid grid-cols-3 gap-3">
        <StatCard label="Owed to suppliers" value={t ? fmt(t.total_minor) : "–"} />
        <StatCard label="Overdue" value={t ? <span className={cn(overdue > 0 && "text-destructive")}>{fmt(overdue)}</span> : "–"} />
        <StatCard label="Credit with suppliers" value={t ? fmt(t.credit_minor) : "–"} />
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Chips
          value={filter}
          onChange={setFilter}
          options={[
            { value: "open", label: "Unpaid" },
            { value: "overdue", label: "Overdue" },
            { value: "paid", label: "Paid" },
            { value: "", label: "All" },
          ]}
        />
        <div className="ml-auto flex gap-2">
          <Button variant="outline" onClick={onBill}>
            <Plus /> Record a bill
          </Button>
          <Button onClick={() => onPay()}>
            <Banknote /> Pay a supplier
          </Button>
        </div>
      </div>
      <QueryError error={list.error ?? ap.error} />
      {list.data?.length === 0 ? (
        <EmptyState
          icon={<ReceiptText />}
          title={filter === "open" || filter === "overdue" ? "Nothing owed" : "No bills"}
          body="Record each supplier invoice as a bill, ideally from its delivery, so you can see what you owe and when it is due."
        />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">Bill</TableHead>
                <TableHead>Supplier</TableHead>
                <TableHead>Their invoice</TableHead>
                <TableHead>Dated</TableHead>
                <TableHead>Due</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Amount</TableHead>
                <TableHead className="pr-4 text-right">Unpaid</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(list.data ?? []).map((b) => (
                <TableRow key={b.id} className={cn("cursor-pointer", b.status === "void" && "opacity-50")} onClick={() => setOpenId(b.id)}>
                  <TableCell className="pl-4 font-medium tabular">{b.bill_no}</TableCell>
                  <TableCell>{b.supplier_name}</TableCell>
                  <TableCell className="text-muted-foreground">{b.supplier_ref || "–"}</TableCell>
                  <TableCell className="text-muted-foreground">{formatDate(b.bill_date)}</TableCell>
                  <TableCell className={cn("text-muted-foreground", b.status === "open" && b.due_date < startOfToday() && "text-destructive")}>{dueText(b)}</TableCell>
                  <TableCell>
                    <BillBadge bill={b} />
                  </TableCell>
                  <TableCell className="text-right tabular">{fmt(b.amount_minor)}</TableCell>
                  <TableCell className="pr-4 text-right font-medium tabular">{b.status === "open" ? fmt(b.amount_minor - b.paid_minor) : "–"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      <BillSheet id={openId} onClose={() => setOpenId(null)} onPay={onPay} />
    </>
  )
}

function BillSheet({ id, onClose, onPay }: { id: string | null; onClose: () => void; onPay: (supplierId?: string, billIds?: string[]) => void }) {
  const { fmt } = useMoney()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ["bo", "bill", id], queryFn: () => hub.bill(id!), enabled: !!id })
  const [voiding, setVoiding] = useState(false)
  const [reason, setReason] = useState("")
  const [busy, setBusy] = useState(false)
  const b = q.data?.bill
  async function voidBill() {
    if (!b) return
    setBusy(true)
    try {
      await hub.voidBill(b.id, reason)
      toast.success(`${b.bill_no} voided`)
      setVoiding(false)
      qc.invalidateQueries({ queryKey: ["bo"] })
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Sheet
      open={!!id}
      onOpenChange={(o) => {
        if (!o) {
          setVoiding(false)
          onClose()
        }
      }}
    >
      <SheetContent className="w-[520px] gap-0 sm:max-w-[520px]">
        <SheetHeader className="border-b">
          <SheetTitle className="flex items-center gap-2">
            Bill {b?.bill_no} {b && <BillBadge bill={b} />}
          </SheetTitle>
          <SheetDescription>{b && `${b.supplier_name}${b.supplier_ref ? ` · invoice ${b.supplier_ref}` : ""} · recorded by ${b.created_by}`}</SheetDescription>
        </SheetHeader>
        <div className="flex-1 space-y-5 overflow-y-auto p-5">
          <QueryError error={q.error} />
          {b && (
            <>
              <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
                <div>
                  <dt className="text-muted-foreground">Dated</dt>
                  <dd>{formatDate(b.bill_date)}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">Due</dt>
                  <dd>{dueText(b)}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">Amount</dt>
                  <dd className="font-medium tabular">{fmt(b.amount_minor)}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">Still to pay</dt>
                  <dd className="font-medium tabular">{b.status === "open" ? fmt(b.amount_minor - b.paid_minor) : "–"}</dd>
                </div>
                {b.grn_no && (
                  <div>
                    <dt className="text-muted-foreground">For delivery</dt>
                    <dd className="tabular">{b.grn_no}</dd>
                  </div>
                )}
              </dl>
              {b.note && <p className="whitespace-pre-line rounded-lg bg-muted px-3 py-2 text-sm">{b.note}</p>}
              <div className="space-y-2">
                <div className="text-sm font-medium">Payments</div>
                {(q.data?.payments ?? []).length === 0 && <p className="text-sm text-muted-foreground">None yet.</p>}
                {(q.data?.payments ?? []).map((p) => (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => printDocument("payment_advice", p.id)}
                    className="flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-sm hover:bg-muted/50"
                    title="Print payment advice"
                  >
                    <div>
                      <span className="font-medium tabular">{p.payment_no}</span> · {p.method}
                      {p.reference && <span className="text-muted-foreground"> · {p.reference}</span>}
                      <div className="text-xs text-muted-foreground">{formatDate(p.paid_at)}</div>
                    </div>
                    <span className="tabular">{fmt(p.amount_minor)}</span>
                  </button>
                ))}
              </div>
            </>
          )}
        </div>
        {b && b.status === "open" && (
          <div className="space-y-3 border-t p-4">
            {voiding ? (
              <>
                <Input autoFocus placeholder="Why is this bill void? (e.g. entered twice)" value={reason} onChange={(e) => setReason(e.target.value)} className="h-10" />
                <div className="flex justify-end gap-2">
                  <Button variant="ghost" onClick={() => setVoiding(false)}>
                    Back
                  </Button>
                  <Button variant="destructive" onClick={voidBill} disabled={busy || !reason.trim()}>
                    {busy && <Loader2 className="animate-spin" />} Void bill
                  </Button>
                </div>
              </>
            ) : (
              <div className="flex justify-end gap-2">
                {b.paid_minor === 0 && (
                  <Button variant="ghost" onClick={() => setVoiding(true)}>
                    <Ban /> Void
                  </Button>
                )}
                <Button onClick={() => onPay(b.supplier_id, [b.id])}>
                  <Banknote /> Pay this bill
                </Button>
              </div>
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}

/** Records a supplier's invoice; from a delivery, the amount is filled in. */
export function BillDialog({ supplierId, grn, onClose }: { supplierId?: string; grn?: GoodsReceipt; onClose: () => void }) {
  const { fmt, currency } = useMoney()
  const qc = useQueryClient()
  const suppliers = useSupplierList()
  const [supplier, setSupplier] = useState(grn?.supplier_id || supplierId || "")
  const [ref, setRef] = useState(grn?.supplier_ref ?? "")
  const [billDate, setBillDate] = useState(() => toDateInput(Date.now()))
  const terms = suppliers.data?.find((s) => s.id === supplier)?.terms_days ?? 0
  const [due, setDue] = useState<string | null>(null) // null = follow the terms
  const dueValue = due ?? toDateInput(fromDateInput(billDate) + terms * DAY)
  const [amount, setAmount] = useState(grn ? (grn.total_minor / 10 ** currency.digits).toFixed(currency.digits) : "")
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  const amt = parseMoney(amount, currency)
  const valid = supplier && amt > 0 && dueValue >= billDate
  async function save() {
    setBusy(true)
    try {
      await hub.createBill({
        supplier_id: supplier,
        supplier_ref: ref,
        grn_id: grn?.id ?? "",
        bill_date: fromDateInput(billDate),
        due_date: fromDateInput(dueValue),
        amount_minor: amt,
        note,
      })
      toast.success("Bill recorded")
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
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Record a supplier bill</DialogTitle>
          <DialogDescription>
            {grn ? `For delivery ${grn.grn_no}, worth ${fmt(grn.total_minor)}. Change the amount if the invoice differs (e.g. delivery charges).` : "Enter the supplier's invoice. It is added to what you owe them."}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label>Supplier</Label>
            <Select value={supplier} onValueChange={setSupplier} disabled={!!grn?.supplier_id}>
              <SelectTrigger className="h-10 w-full">
                <SelectValue placeholder="Choose a supplier" />
              </SelectTrigger>
              <SelectContent>
                {(suppliers.data ?? [])
                  .filter((s) => s.active || s.id === supplier)
                  .map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.name}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Their invoice number</Label>
              <Input value={ref} onChange={(e) => setRef(e.target.value)} className="h-10" />
            </div>
            <div className="space-y-1.5">
              <Label>Amount</Label>
              <Input
                autoFocus={!grn}
                inputMode="decimal"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                className={cn("h-10 text-right tabular", isNaN(amt) && "border-destructive")}
              />
            </div>
            <div className="space-y-1.5">
              <Label>Invoice date</Label>
              <Input type="date" value={billDate} onChange={(e) => setBillDate(e.target.value)} className="h-10" />
            </div>
            <div className="space-y-1.5">
              <Label>Due {due === null && terms > 0 && <span className="font-normal text-muted-foreground">({terms} days)</span>}</Label>
              <Input type="date" value={dueValue} onChange={(e) => setDue(e.target.value)} className={cn("h-10", dueValue < billDate && "border-destructive")} />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label>Note</Label>
            <Input value={note} onChange={(e) => setNote(e.target.value)} className="h-10" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={save} disabled={!valid || busy}>
            {busy && <Loader2 className="animate-spin" />} Record bill
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---------- payments ----------

/** Pays a supplier. The money goes to the ticked bills first, then the oldest. */
export function PayDialog({ supplierId, billIds, onClose }: { supplierId?: string; billIds?: string[]; onClose: () => void }) {
  const { fmt, currency } = useMoney()
  const qc = useQueryClient()
  const suppliers = useSupplierList()
  const [supplier, setSupplier] = useState(supplierId ?? "")
  const bills = useQuery({ queryKey: ["bo", "bills", "open", supplier], queryFn: () => hub.bills(supplier, "open"), enabled: !!supplier })
  const [picked, setPicked] = useState<string[] | null>(billIds ?? null) // null = none chosen yet
  const [amount, setAmount] = useState<string | null>(null) // null = follow the ticked bills
  const [method, setMethod] = useState(PAY_METHODS[0])
  const [reference, setReference] = useState("")
  const [today] = useState(() => toDateInput(Date.now()))
  const [date, setDate] = useState(today)
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  const open = useMemo(() => bills.data ?? [], [bills.data])
  const ticked = picked ?? []
  const tickedTotal = open.filter((b) => ticked.includes(b.id)).reduce((s, b) => s + b.amount_minor - b.paid_minor, 0)
  const owed = suppliers.data?.find((s) => s.id === supplier)?.balance_minor ?? 0
  const amountText = amount ?? (tickedTotal > 0 ? (tickedTotal / 10 ** currency.digits).toFixed(currency.digits) : "")
  const amt = parseMoney(amountText, currency)
  const valid = supplier && amt > 0 && method
  function toggle(id: string) {
    setPicked((p) => {
      const cur = p ?? []
      return cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id]
    })
  }
  async function save() {
    setBusy(true)
    try {
      const { id } = await hub.paySupplier({
        supplier_id: supplier,
        method,
        reference,
        amount_minor: amt,
        paid_at: date === today ? Date.now() : fromDateInput(date) + 12 * 3_600_000,
        note,
        bill_ids: ticked,
      })
      toast.success(`Payment of ${fmt(amt)} recorded`, { action: { label: "Print advice", onClick: () => printDocument("payment_advice", id) } })
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
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Pay a supplier</DialogTitle>
          <DialogDescription>Tick the bills this payment covers. Anything left over goes to the oldest unpaid bills, then stays as credit.</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto">
          <div className="space-y-1.5">
            <Label>Supplier</Label>
            <Select
              value={supplier}
              onValueChange={(v) => {
                setSupplier(v)
                setPicked(null)
                setAmount(null)
              }}
            >
              <SelectTrigger className="h-10 w-full">
                <SelectValue placeholder="Choose a supplier" />
              </SelectTrigger>
              <SelectContent>
                {(suppliers.data ?? [])
                  .filter((s) => s.active || s.id === supplier)
                  .map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.name} {s.balance_minor !== 0 && <span className="text-muted-foreground">· {s.balance_minor > 0 ? `owed ${fmt(s.balance_minor)}` : `credit ${fmt(-s.balance_minor)}`}</span>}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
          </div>
          {supplier && (
            <div className="overflow-hidden rounded-lg border">
              {open.length === 0 ? (
                <p className="px-3 py-4 text-sm text-muted-foreground">{bills.isLoading ? "Loading…" : "No unpaid bills. A payment now is kept as credit for the next bill."}</p>
              ) : (
                <Table>
                  <TableBody>
                    {open.map((b) => (
                      <TableRow key={b.id} className="cursor-pointer" onClick={() => toggle(b.id)}>
                        <TableCell className="w-8 pl-3">
                          <input type="checkbox" readOnly checked={ticked.includes(b.id)} className="size-4 accent-primary" />
                        </TableCell>
                        <TableCell>
                          <div className="font-medium tabular">
                            {b.bill_no} {b.supplier_ref && <span className="font-normal text-muted-foreground">· {b.supplier_ref}</span>}
                          </div>
                          <div className={cn("text-xs text-muted-foreground", b.due_date < startOfToday() && "text-destructive")}>Due {dueText(b)}</div>
                        </TableCell>
                        <TableCell className="pr-3 text-right tabular">{fmt(b.amount_minor - b.paid_minor)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </div>
          )}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Amount paid</Label>
              <Input inputMode="decimal" value={amountText} onChange={(e) => setAmount(e.target.value)} className={cn("h-10 text-right tabular", isNaN(amt) && "border-destructive")} />
              {supplier && owed > 0 && amount === null && ticked.length === 0 && (
                <button type="button" className="text-xs text-primary hover:underline" onClick={() => setAmount((owed / 10 ** currency.digits).toFixed(currency.digits))}>
                  Pay everything owed ({fmt(owed)})
                </button>
              )}
            </div>
            <div className="space-y-1.5">
              <Label>Paid on</Label>
              <Input type="date" value={date} max={today} onChange={(e) => setDate(e.target.value)} className="h-10" />
            </div>
            <div className="space-y-1.5">
              <Label>How</Label>
              <Select value={method} onValueChange={setMethod}>
                <SelectTrigger className="h-10 w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PAY_METHODS.map((m) => (
                    <SelectItem key={m} value={m}>
                      {m}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Reference</Label>
              <Input value={reference} onChange={(e) => setReference(e.target.value)} placeholder={method === "Cheque" ? "Cheque no." : "Transfer ref."} className="h-10" />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label>Note</Label>
            <Input value={note} onChange={(e) => setNote(e.target.value)} className="h-10" />
          </div>
          {method === "Cash" && <p className="text-xs text-muted-foreground">Paid from a till drawer? Also record it as Paid out on that till so the drawer still balances.</p>}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={save} disabled={!valid || busy}>
            {busy && <Loader2 className="animate-spin" />} Record payment
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function PaymentsTab({ onPay }: { onPay: () => void }) {
  const { fmt } = useMoney()
  const list = useQuery({ queryKey: ["bo", "supplier-payments"], queryFn: () => hub.supplierPayments() })
  const [openId, setOpenId] = useState<string | null>(null)
  const detail = useQuery({ queryKey: ["bo", "supplier-payment", openId], queryFn: () => hub.supplierPayment(openId!), enabled: !!openId })
  const p = detail.data
  const used = (p?.allocations ?? []).reduce((s, a) => s + a.amount_minor, 0)
  return (
    <>
      <Button className="w-fit" onClick={onPay}>
        <Banknote /> Pay a supplier
      </Button>
      <QueryError error={list.error} />
      {list.data?.length === 0 ? (
        <EmptyState icon={<Banknote />} title="No payments yet" />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">Payment</TableHead>
                <TableHead>Supplier</TableHead>
                <TableHead>Paid on</TableHead>
                <TableHead>How</TableHead>
                <TableHead>Reference</TableHead>
                <TableHead className="pr-4 text-right">Amount</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(list.data ?? []).map((x) => (
                <TableRow key={x.id} className="cursor-pointer" onClick={() => setOpenId(x.id)}>
                  <TableCell className="pl-4 font-medium tabular">{x.payment_no}</TableCell>
                  <TableCell>{x.supplier_name}</TableCell>
                  <TableCell className="text-muted-foreground">{formatDate(x.paid_at)}</TableCell>
                  <TableCell className="text-muted-foreground">{x.method}</TableCell>
                  <TableCell className="text-muted-foreground">{x.reference}</TableCell>
                  <TableCell className="pr-4 text-right font-medium tabular">{fmt(x.amount_minor)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      <Sheet open={!!openId} onOpenChange={(o) => !o && setOpenId(null)}>
        <SheetContent className="w-[480px] gap-0 sm:max-w-[480px]">
          <SheetHeader className="border-b">
            <SheetTitle>Payment {p?.payment_no}</SheetTitle>
            <SheetDescription>{p && `${p.supplier_name} · ${p.method}${p.reference ? ` ${p.reference}` : ""} · ${formatDate(p.paid_at)} · by ${p.created_by}`}</SheetDescription>
          </SheetHeader>
          <div className="flex-1 overflow-y-auto">
            <Table>
              <TableBody>
                {(p?.allocations ?? []).map((a) => (
                  <TableRow key={a.bill_id}>
                    <TableCell className="pl-4">
                      <div className="font-medium tabular">{a.bill_no}</div>
                      {a.supplier_ref && <div className="text-xs text-muted-foreground">Invoice {a.supplier_ref}</div>}
                    </TableCell>
                    <TableCell className="pr-4 text-right tabular">{fmt(a.amount_minor)}</TableCell>
                  </TableRow>
                ))}
                {p && p.amount_minor - used > 0 && (
                  <TableRow>
                    <TableCell className="pl-4 text-muted-foreground">Credit (not yet used)</TableCell>
                    <TableCell className="pr-4 text-right tabular">{fmt(p.amount_minor - used)}</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
            {p?.note && <p className="px-4 py-3 text-sm text-muted-foreground">{p.note}</p>}
          </div>
          {p && (
            <div className="flex justify-end border-t p-4">
              <PrintButton kind="payment_advice" id={p.id} label="Payment advice" />
            </div>
          )}
        </SheetContent>
      </Sheet>
    </>
  )
}

// ---------- supplier account ----------

const kindLabel = { opening: "Opening balance", bill: "Bill", payment: "Payment", void: "Bill voided", adjustment: "Adjustment" }

export function SupplierSheet({
  supplier,
  onClose,
  onEdit,
  onBill,
  onPay,
}: {
  supplier: Supplier | null
  onClose: () => void
  onEdit: (s: Supplier) => void
  onBill: (supplierId: string) => void
  onPay: (supplierId: string) => void
}) {
  const { fmt } = useMoney()
  const st = useQuery({ queryKey: ["bo", "supplier-statement", supplier?.id], queryFn: () => hub.supplierStatement(supplier!.id), enabled: !!supplier })
  const s = st.data?.supplier ?? supplier
  const entries = [...(st.data?.entries ?? [])].reverse()
  return (
    <Sheet open={!!supplier} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-[680px] gap-0 sm:max-w-[680px]">
        <SheetHeader className="border-b">
          <SheetTitle>{s?.name}</SheetTitle>
          <SheetDescription>{s && [s.contact_name, s.phone, s.email, s.terms_days ? `pays in ${s.terms_days} days` : "pays on receipt"].filter(Boolean).join(" · ")}</SheetDescription>
        </SheetHeader>
        <div className="flex-1 space-y-5 overflow-y-auto p-5">
          <QueryError error={st.error} />
          {s && (
            <div className="grid grid-cols-3 gap-3">
              <StatCard label={s.balance_minor < 0 ? "Credit with them" : "We owe"} value={fmt(Math.abs(s.balance_minor))} />
              <StatCard label="Overdue" value={<span className={cn(s.overdue_minor > 0 && "text-destructive")}>{fmt(s.overdue_minor)}</span>} />
              <StatCard label="Bought to date" value={fmt(s.received_minor)} />
            </div>
          )}
          <div className="overflow-hidden rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40">
                  <TableHead className="pl-3">Date</TableHead>
                  <TableHead>Entry</TableHead>
                  <TableHead className="text-right">Amount</TableHead>
                  <TableHead className="pr-3 text-right">Balance</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="py-8 text-center text-muted-foreground">
                      {st.isLoading ? "Loading…" : "No bills or payments yet."}
                    </TableCell>
                  </TableRow>
                )}
                {entries.map((e) => (
                  <TableRow key={e.id}>
                    <TableCell className="pl-3 whitespace-nowrap text-muted-foreground">{formatDate(e.at)}</TableCell>
                    <TableCell>
                      <div>
                        {kindLabel[e.kind] ?? e.kind} <span className="tabular text-muted-foreground">{e.ref_no}</span>
                      </div>
                      {e.note && <div className="text-xs text-muted-foreground">{e.note}</div>}
                    </TableCell>
                    <TableCell className={cn("text-right tabular", e.amount_minor < 0 && "text-emerald-700 dark:text-emerald-400")}>{fmt(e.amount_minor)}</TableCell>
                    <TableCell className="pr-3 text-right tabular">{fmt(e.balance_minor)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          {st.data && <p className="text-xs text-muted-foreground">Last change {entries[0] ? formatDateTime(entries[0].at) : "–"}.</p>}
        </div>
        {s && (
          <div className="flex flex-wrap gap-2 border-t p-4">
            <Button variant="outline" onClick={() => onEdit(s)}>
              <Pencil /> Edit
            </Button>
            <PrintButton kind="supplier_statement" id={s.id} label="Statement" />
            <div className="ml-auto flex gap-2">
              <Button variant="outline" onClick={() => onBill(s.id)}>
                <FileText /> Record bill
              </Button>
              <Button onClick={() => onPay(s.id)}>
                <Banknote /> Pay
              </Button>
            </div>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
