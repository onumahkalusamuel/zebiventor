import { useEffect, useMemo, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { ArrowRightLeft, Banknote, CheckCircle2, CreditCard, Loader2, Printer, Smartphone, Ticket, Wallet, X, type LucideIcon } from "lucide-react"
import { toast } from "sonner"
import { useApproval } from "@/components/approval"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { api, errorMessage } from "@/lib/api"
import { parseMoney } from "@/lib/format"
import { useMoney } from "@/lib/state"
import { STORE_CREDIT, type Customer, type PaymentMethod, type SaleResult } from "@/lib/types"
import { cn } from "@/lib/utils"
import { unitPrice, type CartLine } from "./sell"

const kindIcon: Record<string, LucideIcon> = {
  cash: Banknote,
  card: CreditCard,
  transfer: ArrowRightLeft,
  mobile_money: Smartphone,
  credit: Wallet,
  voucher: Ticket,
  other: Wallet,
}

interface Tender {
  key: number
  method: PaymentMethod
  amount: string
  reference: string
}

export function PayDialog({
  open,
  onOpenChange,
  lines,
  discount,
  total,
  customer,
  approvalToken,
  needsApproval,
  onNeedCustomer,
  onDone,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  lines: CartLine[]
  discount: number
  total: number
  customer: Customer | null
  approvalToken: string
  needsApproval: boolean
  onNeedCustomer: () => void
  onDone: () => void
}) {
  const { fmt, currency } = useMoney()
  const requestApproval = useApproval()
  const methods = useQuery({ queryKey: ["payment-methods"], queryFn: api.paymentMethods })
  const [tenders, setTenders] = useState<Tender[]>([])
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<SaleResult | null>(null)
  const keyRef = useRef(0)

  useEffect(() => {
    if (open) {
      setTenders([])
      setResult(null)
    }
  }, [open])

  // A customer with store credit (a balance below zero) can spend it.
  const credit = customer && customer.balance_minor < 0 ? -customer.balance_minor : 0
  const allMethods: PaymentMethod[] = [
    ...(credit > 0
      ? [{ id: STORE_CREDIT, name: `Store credit (${fmt(credit)})`, kind: "credit", requires_reference: false, opens_drawer: false, sort_order: 0, active: true } as PaymentMethod]
      : []),
    ...(methods.data ?? []),
  ]

  const parsed = tenders.map((t) => ({ ...t, minor: parseMoney(t.amount, currency) }))
  const paid = parsed.reduce((s, t) => s + (isNaN(t.minor) ? 0 : t.minor), 0)
  const cash = parsed.filter((t) => t.method.kind === "cash").reduce((s, t) => s + (isNaN(t.minor) ? 0 : t.minor), 0)
  const remaining = Math.max(0, total - paid)
  const change = Math.max(0, paid - total)
  const invalidAmount = parsed.some((t) => isNaN(t.minor) || t.minor < 0)
  const missingRef = parsed.some((t) => t.method.requires_reference && t.minor > 0 && !t.reference.trim())
  const nonCashOver = change > cash
  const ready = paid >= total && !invalidAmount && !missingRef && !nonCashOver

  function addTender(m: PaymentMethod) {
    if (m.kind === "credit" && !customer) {
      toast.info("Choose the customer first to put this sale on their account")
      onOpenChange(false)
      onNeedCustomer()
      return
    }
    const want = m.id === STORE_CREDIT ? Math.min(remaining, credit) : remaining
    const amount = want > 0 ? (want / 10 ** currency.digits).toFixed(currency.digits) : ""
    setTenders((ts) => [...ts, { key: ++keyRef.current, method: m, amount, reference: "" }])
    setTimeout(() => {
      const inputs = document.querySelectorAll<HTMLInputElement>("[data-tender-amount]")
      inputs[inputs.length - 1]?.select()
    }, 30)
  }

  // Cash needed after any non-cash tenders, then common rounded-up notes.
  const nonCash = paid - cash
  const cashTarget = Math.max(0, total - nonCash)
  const quickCash = useMemo(() => {
    const unit = 10 ** currency.digits
    const set = new Set<number>([cashTarget])
    for (const step of [500, 1000, 2000, 5000, 10000]) {
      set.add(Math.ceil(cashTarget / (step * unit)) * step * unit)
    }
    return [...set].filter((v) => v > 0).sort((a, b) => a - b).slice(0, 5)
  }, [cashTarget, currency.digits])

  function setCash(minor: number) {
    const cashMethod = methods.data?.find((m) => m.kind === "cash")
    if (!cashMethod) return
    const amount = (minor / 10 ** currency.digits).toFixed(currency.digits)
    setTenders((ts) => {
      const i = ts.findIndex((t) => t.method.kind === "cash")
      if (i >= 0) return ts.map((t, j) => (j === i ? { ...t, amount } : t))
      return [...ts, { key: ++keyRef.current, method: cashMethod, amount, reference: "" }]
    })
  }

  async function complete() {
    if (!ready || busy) return
    setBusy(true)
    const submit = (token: string) =>
      api.completeSale({
        lines: lines.map((l) => ({
          variant_id: l.item.variant_id,
          qty_milli: l.qty_milli,
          ...(l.price_override !== undefined ? { price_override: unitPrice(l) } : {}),
        })),
        payments: parsed.filter((t) => t.minor > 0).map((t) => ({ method_id: t.method.id, amount_minor: t.minor, reference: t.reference.trim() })),
        discount_minor: discount,
        customer_id: customer?.id ?? "",
        note: "",
        approval_token: token,
      })
    try {
      let res: SaleResult
      try {
        res = await submit(approvalToken)
      } catch (e) {
        // The earlier approval expired (2 min) or was never given: ask again.
        if (!needsApproval || !/approval required/i.test(errorMessage(e))) throw e
        const tok = await requestApproval("Approve the discount / price changes on this sale")
        if (tok === null) return
        res = await submit(tok)
      }
      setResult(res)
      if (res.print_error) toast.warning("Sale saved, but the receipt didn’t print", { description: res.print_error })
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  function finish() {
    onDone()
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (result ? finish() : onOpenChange(o))}>
      <DialogContent className="max-w-[880px] gap-0 overflow-hidden p-0 sm:max-w-[880px]" showCloseButton={false}>
        {result ? (
          <SaleDone result={result} onNew={finish} />
        ) : (
          <div className="grid grid-cols-[1fr_340px]">
            <div className="space-y-5 p-6">
              <div>
                <DialogTitle className="text-lg">Take payment</DialogTitle>
                <DialogDescription>Choose how the customer is paying. Split across several methods if needed.</DialogDescription>
              </div>
              <div className="grid grid-cols-3 gap-2">
                {allMethods.map((m) => {
                  const Icon = kindIcon[m.kind] ?? Wallet
                  return (
                    <button
                      key={m.id}
                      onClick={() => addTender(m)}
                      className="flex h-20 flex-col items-center justify-center gap-1.5 rounded-xl border bg-card text-sm font-medium transition-all hover:border-primary/50 hover:bg-accent/40 active:scale-[0.98]"
                    >
                      <Icon className="size-5 text-primary" />
                      {m.name}
                    </button>
                  )
                })}
              </div>

              {tenders.length > 0 && (
                <div className="space-y-2">
                  {tenders.map((t, i) => {
                    const Icon = kindIcon[t.method.kind] ?? Wallet
                    return (
                      <div key={t.key} className="flex items-center gap-2 rounded-lg border bg-muted/30 p-2">
                        <div className="flex w-36 items-center gap-2 pl-1 text-sm font-medium">
                          <Icon className="size-4 text-muted-foreground" />
                          <span className="truncate">{t.method.name}</span>
                        </div>
                        <Input
                          data-tender-amount
                          aria-label={`${t.method.name} amount`}
                          inputMode="decimal"
                          value={t.amount}
                          onChange={(e) => setTenders((ts) => ts.map((x, j) => (j === i ? { ...x, amount: e.target.value } : x)))}
                          onKeyDown={(e) => e.key === "Enter" && complete()}
                          className="h-10 w-40 text-right text-base font-semibold tabular"
                        />
                        {(t.method.requires_reference || t.method.kind !== "cash") && (
                          <Input
                            placeholder={t.method.requires_reference ? "Reference (required)" : "Reference (optional)"}
                            value={t.reference}
                            onChange={(e) => setTenders((ts) => ts.map((x, j) => (j === i ? { ...x, reference: e.target.value } : x)))}
                            onKeyDown={(e) => e.key === "Enter" && complete()}
                            className={cn("h-10 flex-1", t.method.requires_reference && !t.reference.trim() && "border-amber-500/60")}
                          />
                        )}
                        <Button variant="ghost" size="icon" className="ml-auto" aria-label={`Remove ${t.method.name} payment`} onClick={() => setTenders((ts) => ts.filter((_, j) => j !== i))}>
                          <X />
                        </Button>
                      </div>
                    )
                  })}
                </div>
              )}

              {methods.data?.some((m) => m.kind === "cash") && (
                <div className="space-y-2">
                  <div className="text-xs font-medium tracking-wide text-muted-foreground uppercase">Quick cash</div>
                  <div className="flex flex-wrap gap-2">
                    {quickCash.map((v, i) => (
                      <Button key={v} variant="outline" className="h-10 tabular" onClick={() => setCash(v)}>
                        {i === 0 ? "Exact " : ""}
                        {fmt(v)}
                      </Button>
                    ))}
                  </div>
                </div>
              )}
            </div>

            <div className="flex flex-col bg-sidebar p-6 text-sidebar-foreground">
              <div className="text-sm">Total due</div>
              <div className="text-4xl font-semibold tracking-tight text-sidebar-accent-foreground tabular">{fmt(total)}</div>
              <div className="mt-6 space-y-2 text-sm">
                <div className="flex justify-between">
                  <span>Paid</span>
                  <span className="tabular text-sidebar-accent-foreground">{fmt(paid)}</span>
                </div>
                <div className="flex justify-between">
                  <span>Remaining</span>
                  <span className={cn("tabular", remaining > 0 ? "text-amber-300" : "text-sidebar-accent-foreground")}>{fmt(remaining)}</span>
                </div>
              </div>
              <div className="mt-6 rounded-xl bg-sidebar-accent p-4">
                <div className="text-xs tracking-wide uppercase">Change</div>
                <div className={cn("text-3xl font-semibold tabular", change > 0 ? "text-sidebar-primary" : "text-sidebar-accent-foreground/50")}>{fmt(change)}</div>
              </div>
              <div className="mt-3 min-h-5 text-xs text-amber-300">
                {missingRef ? "Enter the payment reference." : nonCashOver ? "Only cash can be more than the amount due." : invalidAmount ? "Check the amounts." : ""}
              </div>
              <div className="mt-auto space-y-2 pt-6">
                <Button className="h-14 w-full bg-sidebar-primary text-lg text-sidebar-primary-foreground hover:bg-sidebar-primary/90" disabled={!ready || busy} onClick={complete}>
                  {busy ? <Loader2 className="animate-spin" /> : <CheckCircle2 className="size-5" />}
                  Complete sale
                </Button>
                <Button variant="ghost" className="w-full text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground" onClick={() => onOpenChange(false)}>
                  Back to cart
                </Button>
              </div>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

function SaleDone({ result, onNew }: { result: SaleResult; onNew: () => void }) {
  const { fmt } = useMoney()
  const [preview, setPreview] = useState<string>("")
  const [printing, setPrinting] = useState(false)
  const newRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    api.receiptPreview(result.sale.id, false).then(setPreview).catch(() => {})
    setTimeout(() => newRef.current?.focus(), 50)
  }, [result.sale.id])

  async function print() {
    setPrinting(true)
    try {
      await api.printSale(result.sale.id, result.printed)
      toast.success("Receipt sent to printer")
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setPrinting(false)
    }
  }

  return (
    <div className="grid grid-cols-[1fr_340px]">
      <div className="flex flex-col items-center justify-center gap-5 p-10 text-center">
        <div className="grid size-16 place-items-center rounded-full bg-emerald-500/12">
          <CheckCircle2 className="size-9 text-emerald-600" />
        </div>
        <div>
          <DialogTitle className="text-2xl">Sale complete</DialogTitle>
          <DialogDescription className="mt-1">
            Receipt <span className="font-medium text-foreground">{result.sale.receipt_no}</span> · {fmt(result.sale.total_minor)}
          </DialogDescription>
        </div>
        {result.sale.change_minor > 0 && (
          <div className="rounded-2xl border-2 border-primary/30 bg-accent/50 px-10 py-5">
            <div className="text-xs font-medium tracking-wide text-accent-foreground uppercase">Give change</div>
            <div className="text-5xl font-semibold tracking-tight text-accent-foreground tabular">{fmt(result.sale.change_minor)}</div>
          </div>
        )}
        <div className="flex w-full max-w-sm gap-2 pt-2">
          <Button variant="outline" className="h-12 flex-1" onClick={print} disabled={printing}>
            {printing ? <Loader2 className="animate-spin" /> : <Printer />}
            {result.printed ? "Print again" : "Print receipt"}
          </Button>
          <Button ref={newRef} className="h-12 flex-1 text-base" onClick={onNew}>
            New sale <kbd className="ml-1 rounded bg-primary-foreground/15 px-1.5 text-[11px] font-normal">Enter</kbd>
          </Button>
        </div>
      </div>
      <div className="max-h-[78vh] overflow-y-auto bg-muted p-5">
        {preview ? (
          <img src={preview} alt="Receipt preview" className="mx-auto w-full rounded-sm bg-white shadow-md [image-rendering:pixelated]" />
        ) : (
          <div className="grid h-64 place-items-center">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          </div>
        )}
      </div>
    </div>
  )
}
