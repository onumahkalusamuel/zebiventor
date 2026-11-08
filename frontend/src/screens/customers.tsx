import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Banknote, Loader2, Pencil, Scale, Search, UserPlus, UsersRound } from "lucide-react"
import { toast } from "sonner"
import { CustomerForm } from "@/components/customer-picker"
import { ExportButton, QueryError } from "@/components/backoffice"
import { EmptyState, Page, PageHeader, StatCard } from "@/components/page"
import { useShift } from "@/components/shift-gate"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api, errorMessage } from "@/lib/api"
import { atLeast, formatDateTime, parseMoney, timeAgo } from "@/lib/format"
import { csvMoney, hub, toCSV } from "@/lib/hub"
import { useLoadedState, useMoney } from "@/lib/state"
import type { Customer, CustomerLedgerEntry } from "@/lib/types"
import { cn } from "@/lib/utils"

export function Customers() {
  const { fmt } = useMoney()
  const [q, setQ] = useState("")
  const [owingOnly, setOwingOnly] = useState(false)
  const [editing, setEditing] = useState<Customer | "new" | null>(null)
  const [paying, setPaying] = useState<Customer | null>(null)
  const [statement, setStatement] = useState<Customer | null>(null)
  const list = useQuery({ queryKey: ["customers", q], queryFn: () => api.searchCustomers(q) })
  const all = list.data ?? []
  const owing = all.filter((c) => c.balance_minor > 0)
  const totalOwed = owing.reduce((s, c) => s + c.balance_minor, 0)
  const shown = owingOnly ? owing.sort((a, b) => b.balance_minor - a.balance_minor) : all

  return (
    <Page>
      <PageHeader
        title="Customers"
        description="Shared by every till. Add customers and take payments on account even while offline."
        actions={
          <Button className="h-10" onClick={() => setEditing("new")}>
            <UserPlus /> New customer
          </Button>
        }
      />
      <div className="space-y-5 px-8 pb-8">
        <div className="grid grid-cols-3 gap-3">
          <StatCard label="Customers" value={all.length} sub={q ? "matching your search" : undefined} />
          <button className="text-left" onClick={() => setOwingOnly((v) => !v)}>
            <StatCard label="Owing on account" value={owing.length} sub={owingOnly ? "showing only these · click to show all" : "click to list them"} className={cn(owingOnly && "border-primary")} />
          </button>
          <StatCard label="Total owed" value={fmt(totalOwed)} />
        </div>
        <div className="relative max-w-md">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Name, phone or email" className="h-10 pl-9" />
        </div>
        {shown.length === 0 && list.data ? (
          <EmptyState icon={<UsersRound />} title={q || owingOnly ? "No matching customers" : "No customers yet"} body="Add customers at the till or here." />
        ) : (
          <div className="overflow-hidden rounded-xl border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40">
                  <TableHead className="pl-4">Name</TableHead>
                  <TableHead>Phone</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Updated</TableHead>
                  <TableHead className="text-right">Balance owed</TableHead>
                  <TableHead className="w-32 pr-4" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {shown.map((c) => (
                  <TableRow key={c.id} className={cn("cursor-pointer", !c.active && "opacity-50")} onClick={() => setStatement(c)}>
                    <TableCell className="pl-4 font-medium">
                      {c.name}
                      {!c.active && <Badge variant="outline" className="ml-2">Inactive</Badge>}
                    </TableCell>
                    <TableCell className="text-muted-foreground tabular">{c.phone}</TableCell>
                    <TableCell className="text-muted-foreground">{c.email}</TableCell>
                    <TableCell className="text-muted-foreground">{timeAgo(c.updated_at)}</TableCell>
                    <TableCell className={cn("text-right font-semibold tabular", c.balance_minor > 0 && "text-amber-700 dark:text-amber-400", c.balance_minor < 0 && "text-emerald-700 dark:text-emerald-400")}>
                      {c.balance_minor > 0 ? fmt(c.balance_minor) : c.balance_minor < 0 ? `${fmt(-c.balance_minor)} credit` : "–"}
                    </TableCell>
                    <TableCell className="pr-4 text-right">
                      {c.balance_minor > 0 && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={(e) => {
                            e.stopPropagation()
                            setPaying(c)
                          }}
                        >
                          <Banknote /> Take payment
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
      <Dialog open={!!editing} onOpenChange={(o) => !o && setEditing(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{editing === "new" ? "New customer" : "Edit customer"}</DialogTitle>
            <DialogDescription>Changes reach every till automatically.</DialogDescription>
          </DialogHeader>
          {editing && <CustomerForm initial={editing === "new" ? {} : editing} onSaved={() => setEditing(null)} onCancel={() => setEditing(null)} />}
        </DialogContent>
      </Dialog>
      {paying && <PaymentDialog customer={paying} onClose={() => setPaying(null)} />}
      <CustomerSheet
        customer={statement}
        onClose={() => setStatement(null)}
        onEdit={(c) => {
          setStatement(null)
          setEditing(c)
        }}
        onPay={(c) => setPaying(c)}
      />
    </Page>
  )
}

function PaymentDialog({ customer, onClose }: { customer: Customer; onClose: () => void }) {
  const { fmt, currency } = useMoney()
  const qc = useQueryClient()
  const shift = useShift()
  const methods = useQuery({ queryKey: ["methods"], queryFn: api.paymentMethods })
  const usable = (methods.data ?? []).filter((m) => m.active && m.kind !== "credit")
  const [methodId, setMethodId] = useState("")
  const [amount, setAmount] = useState(() => (customer.balance_minor / 10 ** currency.digits).toFixed(currency.digits))
  const [reference, setReference] = useState("")
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  const method = usable.find((m) => m.id === (methodId || usable[0]?.id))
  const minor = parseMoney(amount, currency)
  const valid = method && !isNaN(minor) && minor > 0 && (!method.requires_reference || reference.trim())

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!method) return
    setBusy(true)
    try {
      const r = await api.receiveAccountPayment({ customer_id: customer.id, method_id: method.id, amount_minor: minor, reference, note })
      toast.success(`${fmt(minor)} received from ${customer.name} (${r.payment.payment_no})`)
      if (r.print_error) toast.warning("Receipt not printed: " + r.print_error)
      qc.invalidateQueries({ queryKey: ["customers"] })
      qc.invalidateQueries({ queryKey: ["shift"] })
      qc.invalidateQueries({ queryKey: ["sales"] })
      onClose()
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Payment from {customer.name}</DialogTitle>
            <DialogDescription>
              Owes {fmt(customer.balance_minor)}. The payment counts in this shift's takings and prints a receipt.
            </DialogDescription>
          </DialogHeader>
          {!shift.data && !shift.isLoading ? (
            <p className="rounded-lg bg-amber-500/10 px-3 py-2 text-sm text-amber-800 dark:text-amber-300">Open a shift on the Shift page before taking payments.</p>
          ) : (
            <>
              <div className="space-y-1.5">
                <Label>Amount</Label>
                <Input autoFocus value={amount} onChange={(e) => setAmount(e.target.value)} className="h-12 text-xl tabular" />
                {!isNaN(minor) && minor > customer.balance_minor && (
                  <p className="text-xs text-muted-foreground">{fmt(minor - customer.balance_minor)} more than owed; it stays on the account as credit.</p>
                )}
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label>Paid by</Label>
                  <Select value={method?.id ?? ""} onValueChange={setMethodId}>
                    <SelectTrigger className="h-10 w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {usable.map((m) => (
                        <SelectItem key={m.id} value={m.id}>
                          {m.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label>Reference{method?.requires_reference ? "" : " (optional)"}</Label>
                  <Input value={reference} onChange={(e) => setReference(e.target.value)} className="h-10" />
                </div>
              </div>
              <div className="space-y-1.5">
                <Label>Note (optional)</Label>
                <Input value={note} onChange={(e) => setNote(e.target.value)} className="h-10" />
              </div>
            </>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!valid || busy || !shift.data}>
              {busy ? <Loader2 className="animate-spin" /> : <Banknote />} Receive {!isNaN(minor) && minor > 0 ? fmt(minor) : ""}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

const kindLabel: Record<CustomerLedgerEntry["kind"], string> = {
  opening: "Opening balance",
  sale: "Sale on account",
  return: "Refund to account",
  payment: "Payment received",
  adjustment: "Adjustment",
}

function CustomerSheet({ customer, onClose, onEdit, onPay }: { customer: Customer | null; onClose: () => void; onEdit: (c: Customer) => void; onPay: (c: Customer) => void }) {
  const state = useLoadedState()
  const { fmt, currency } = useMoney()
  const isManager = atLeast(state.user?.role, "manager")
  const canStatement = isManager && state.has_session
  const st = useQuery({ queryKey: ["bo", "statement", customer?.id], queryFn: () => hub.statement(customer!.id), enabled: !!customer && canStatement })
  const [adjusting, setAdjusting] = useState(false)
  const c = st.data?.customer ?? customer
  const entries = [...(st.data?.entries ?? [])].reverse()

  return (
    <Sheet open={!!customer} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-[620px] gap-0 sm:max-w-[620px]">
        <SheetHeader className="border-b">
          <SheetTitle>{c?.name}</SheetTitle>
          <SheetDescription>{[c?.phone, c?.email].filter(Boolean).join(" · ") || "No contact details"}</SheetDescription>
        </SheetHeader>
        {c && (
          <div className="flex items-end justify-between gap-3 border-b px-5 py-4">
            <div>
              <div className="text-xs font-medium tracking-wide text-muted-foreground uppercase">{c.balance_minor < 0 ? "In credit" : "Owes"}</div>
              <div className={cn("text-3xl font-semibold tabular", c.balance_minor > 0 && "text-amber-700 dark:text-amber-400")}>{fmt(Math.abs(c.balance_minor))}</div>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => onEdit(c)}>
                <Pencil /> Edit
              </Button>
              {state.user?.role === "admin" && state.has_session && (
                <Button variant="outline" onClick={() => setAdjusting(true)}>
                  <Scale /> Adjust
                </Button>
              )}
              <Button onClick={() => onPay(c)}>
                <Banknote /> Take payment
              </Button>
            </div>
          </div>
        )}
        <div className="flex-1 overflow-y-auto">
          {!canStatement ? (
            <p className="p-5 text-sm text-muted-foreground">
              {isManager ? "The statement comes from the hub; it will show once this till is connected." : "Managers can see the full statement of account."}
            </p>
          ) : (
            <>
              <QueryError error={st.error} />
              {st.isLoading && <Loader2 className="mx-auto mt-10 size-5 animate-spin text-muted-foreground" />}
              {st.data && entries.length === 0 && <p className="p-5 text-sm text-muted-foreground">No sales on account yet.</p>}
              {entries.length > 0 && (
                <Table>
                  <TableHeader>
                    <TableRow className="bg-muted/40">
                      <TableHead className="pl-5">Date</TableHead>
                      <TableHead>Entry</TableHead>
                      <TableHead className="text-right">Amount</TableHead>
                      <TableHead className="pr-5 text-right">Balance</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {entries.map((e) => (
                      <TableRow key={e.id}>
                        <TableCell className="pl-5 whitespace-nowrap text-muted-foreground">{formatDateTime(e.at)}</TableCell>
                        <TableCell>
                          <div>
                            {kindLabel[e.kind]} {e.ref_no && <span className="text-muted-foreground tabular">{e.ref_no}</span>}
                          </div>
                          {(e.note || e.user_name) && <div className="text-xs text-muted-foreground">{[e.note, e.user_name].filter(Boolean).join(" · ")}</div>}
                        </TableCell>
                        <TableCell className={cn("text-right tabular", e.amount_minor < 0 && "text-emerald-700 dark:text-emerald-400")}>{fmt(e.amount_minor)}</TableCell>
                        <TableCell className="pr-5 text-right font-medium tabular">{fmt(e.balance_minor)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </>
          )}
        </div>
        {canStatement && st.data && entries.length > 0 && (
          <div className="border-t p-4">
            <ExportButton
              label="Export statement"
              name={`statement-${(c?.name ?? "customer").replace(/\W+/g, "-").toLowerCase()}.csv`}
              build={() =>
                toCSV(
                  ["Date", "Entry", "Reference", "Note", "Amount", "Balance"],
                  (st.data?.entries ?? []).map((e) => [new Date(e.at).toLocaleString(), kindLabel[e.kind], e.ref_no, e.note, csvMoney(e.amount_minor, currency.digits), csvMoney(e.balance_minor, currency.digits)]),
                )
              }
            />
          </div>
        )}
        {adjusting && c && <AdjustDialog customer={c} onClose={() => setAdjusting(false)} />}
      </SheetContent>
    </Sheet>
  )
}

function AdjustDialog({ customer, onClose }: { customer: Customer; onClose: () => void }) {
  const qc = useQueryClient()
  const { fmt, currency } = useMoney()
  const [direction, setDirection] = useState<"up" | "down">("up")
  const [amount, setAmount] = useState("")
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  const minor = parseMoney(amount, currency)
  async function save(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await hub.adjustBalance(customer.id, direction === "up" ? minor : -minor, note)
      toast.success("Balance adjusted")
      qc.invalidateQueries({ queryKey: ["bo", "statement", customer.id] })
      setTimeout(() => qc.invalidateQueries({ queryKey: ["customers"] }), 800)
      onClose()
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={save} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Adjust {customer.name}'s balance</DialogTitle>
            <DialogDescription>For an opening balance brought over from another system, or a write-off. Recorded on the statement with your name.</DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label>Amount</Label>
            <Input autoFocus value={amount} onChange={(e) => setAmount(e.target.value)} className="h-10 tabular" />
          </div>
          <Select value={direction} onValueChange={(v) => setDirection(v as "up" | "down")}>
            <SelectTrigger className="h-10 w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="up">Increase what they owe (e.g. opening balance)</SelectItem>
              <SelectItem value="down">Reduce what they owe (e.g. write-off)</SelectItem>
            </SelectContent>
          </Select>
          <div className="space-y-1.5">
            <Label>Reason</Label>
            <Input value={note} onChange={(e) => setNote(e.target.value)} className="h-10" />
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy || isNaN(minor) || minor <= 0 || !note.trim()}>
              {busy && <Loader2 className="animate-spin" />} {direction === "up" ? "Add" : "Deduct"} {!isNaN(minor) && minor > 0 ? fmt(minor) : ""}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

