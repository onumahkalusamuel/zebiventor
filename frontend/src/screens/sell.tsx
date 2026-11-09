import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import {
  Archive,
  ArchiveRestore,
  BadgePercent,
  Barcode,
  Inbox,
  Minus,
  PackageOpen,
  Plus,
  ScanLine,
  Search,
  ShoppingBag,
  Trash2,
  UserRound,
  X,
} from "lucide-react"
import { toast } from "sonner"
import { useApproval } from "@/components/approval"
import { CustomerPicker } from "@/components/customer-picker"
import { ShiftGate } from "@/components/shift-gate"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { api, errorMessage } from "@/lib/api"
import { formatDateTime, formatQty, parseMoney, parseQty } from "@/lib/format"
import { computeTotals, lineGross, type TaxMode } from "@/lib/pricing"
import { useLoadedState, useMoney } from "@/lib/state"
import type { Customer, Item } from "@/lib/types"
import { cn } from "@/lib/utils"
import { PayDialog } from "./pay-dialog"

export interface CartLine {
  item: Item
  qty_milli: number
  price_override?: number
}

export const unitPrice = (l: CartLine) => l.price_override ?? l.item.price_minor

interface CartState {
  lines: CartLine[]
  discount: number
  customer: Customer | null
  approvalToken: string // covers the discount / overrides currently in the cart
}

const emptyCart: CartState = { lines: [], discount: 0, customer: null, approvalToken: "" }

function useDebounced<T>(value: T, ms: number) {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

export function Sell() {
  return (
    <ShiftGate>
      <SellScreen />
    </ShiftGate>
  )
}

function SellScreen() {
  const state = useLoadedState()
  const qc = useQueryClient()
  const requestApproval = useApproval()
  const { fmt, currency } = useMoney()
  const mode = (state.settings.tax_mode || "none") as TaxMode
  const taxName = state.settings.tax_name || "Tax"

  const [cart, setCart] = useState<CartState>(emptyCart)
  const [selected, setSelected] = useState<string | null>(null)
  const [query, setQuery] = useState("")
  const [dialog, setDialog] = useState<null | "pay" | "discount" | "customer" | "held" | "hold">(null)
  const [lineEdit, setLineEdit] = useState<CartLine | null>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const debounced = useDebounced(query, 120)

  const results = useQuery({ queryKey: ["items", debounced], queryFn: () => api.searchItems(debounced) })
  const held = useQuery({ queryKey: ["held"], queryFn: api.heldSales })

  const totals = useMemo(
    () => computeTotals(cart.lines.map((l) => ({ qty_milli: l.qty_milli, unit_minor: unitPrice(l), tax_rate_bp: l.item.tax_rate_bp })), cart.discount, mode),
    [cart.lines, cart.discount, mode],
  )
  const itemCount = cart.lines.reduce((s, l) => s + l.qty_milli, 0)

  const add = useCallback((item: Item, qty = item.pack_qty_milli || 1000) => {
    setCart((c) => {
      const i = c.lines.findIndex((l) => l.item.variant_id === item.variant_id && l.price_override === undefined)
      if (i >= 0) {
        const lines = [...c.lines]
        lines[i] = { ...lines[i], qty_milli: lines[i].qty_milli + qty }
        return { ...c, lines }
      }
      return { ...c, lines: [...c.lines, { item, qty_milli: qty }] }
    })
    setSelected(item.variant_id)
  }, [])

  // Never steal focus from an open dialog (e.g. a manager approval prompt).
  const focusSearch = () =>
    setTimeout(() => {
      if (!document.querySelector("[role=dialog]")) searchRef.current?.focus()
    }, 50)

  async function submitSearch() {
    const q = query.trim()
    if (!q) return
    try {
      const hit = await api.lookupCode(q)
      if (hit) {
        add(hit)
        setQuery("")
        return
      }
    } catch (e) {
      toast.error(errorMessage(e))
      return
    }
    const list = results.data ?? []
    if (list.length === 1 && debounced === query) {
      add(list[0])
      setQuery("")
    } else if (list.length === 0) {
      toast.warning(`Nothing matches “${q}”`)
    }
  }

  const updateLine = (idx: number, patch: Partial<CartLine> | null) =>
    setCart((c) => ({
      ...c,
      lines: patch === null || (patch.qty_milli !== undefined && patch.qty_milli <= 0) ? c.lines.filter((_, i) => i !== idx) : c.lines.map((l, i) => (i === idx ? { ...l, ...patch } : l)),
    }))

  function clear() {
    setCart(emptyCart)
    setSelected(null)
    focusSearch()
  }

  async function applyDiscount(minor: number) {
    if (minor > 0) {
      const tok = await requestApproval(`Give a ${fmt(minor)} discount`, "discount")
      if (tok === null) return
      setCart((c) => ({ ...c, discount: minor, approvalToken: tok || c.approvalToken }))
    } else {
      setCart((c) => ({ ...c, discount: 0 }))
    }
  }

  async function hold(label: string) {
    try {
      await api.holdSale({ label, payload: JSON.stringify(cart), item_count: itemCount, total_minor: totals.total })
      toast.success("Sale held", { description: label || undefined })
      qc.invalidateQueries({ queryKey: ["held"] })
      clear()
    } catch (e) {
      toast.error(errorMessage(e))
    }
  }

  async function recall(id: string) {
    if (cart.lines.length) {
      toast.warning("Hold or finish the current sale first")
      return
    }
    try {
      const h = await api.recallHeldSale(id)
      const restored = JSON.parse(h.payload) as CartState
      // Approvals are single-use and short-lived: ask again if needed at payment.
      setCart({ ...emptyCart, ...restored, approvalToken: "" })
      qc.invalidateQueries({ queryKey: ["held"] })
      setDialog(null)
      focusSearch()
    } catch (e) {
      toast.error(errorMessage(e))
    }
  }

  async function noSale() {
    const tok = await requestApproval("Open the cash drawer without a sale", "no_sale")
    if (tok === null) return
    try {
      await api.openDrawer(tok)
    } catch (e) {
      toast.error(errorMessage(e))
    }
  }

  // Keyboard: typing anywhere goes to search (scanners act as keyboards);
  // F12 pay, F8 hold, F9 customer, Delete removes the selected line.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (dialog || lineEdit || document.querySelector("[role=dialog]")) return
      const t = e.target as HTMLElement
      const inField = t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable
      if (e.key === "F12" || (e.key === "Enter" && e.ctrlKey)) {
        e.preventDefault()
        if (cart.lines.length) setDialog("pay")
        return
      }
      if (e.key === "F8") {
        e.preventDefault()
        if (cart.lines.length) setDialog("hold")
        else if (held.data?.length) setDialog("held")
        return
      }
      if (e.key === "F9") {
        e.preventDefault()
        setDialog("customer")
        return
      }
      if (!inField && e.key === "Delete" && selected) {
        const idx = cart.lines.findIndex((l) => l.item.variant_id === selected)
        if (idx >= 0) updateLine(idx, null)
        return
      }
      if (!inField && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault()
        setQuery((q) => q + e.key)
        searchRef.current?.focus()
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [cart.lines, dialog, lineEdit, selected, held.data])

  useEffect(() => {
    focusSearch()
  }, [])

  const inCart = (variantId: string) => cart.lines.filter((l) => l.item.variant_id === variantId).reduce((s, l) => s + l.qty_milli, 0)

  return (
    <div className="grid h-full grid-cols-[1fr_460px]">
      {/* Catalogue side */}
      <section className="flex min-h-0 flex-col">
        <div className="flex gap-2 border-b bg-card p-4">
          <form
            onSubmit={(e) => {
              e.preventDefault()
              submitSearch()
            }}
            className="relative flex-1"
          >
            <Search className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-muted-foreground" />
            <Input
              ref={searchRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === "Escape" && setQuery("")}
              placeholder="Scan a barcode, or search by title, author, ISBN, SKU…"
              className="h-14 rounded-xl pr-28 pl-12 text-lg md:text-lg"
            />
            <div className="pointer-events-none absolute top-1/2 right-4 flex -translate-y-1/2 items-center gap-1.5 text-xs text-muted-foreground">
              <ScanLine className="size-4" /> Ready to scan
            </div>
          </form>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="outline" className="relative h-14 w-14 rounded-xl" onClick={() => setDialog("held")} disabled={!held.data?.length}>
                <ArchiveRestore className="size-5" />
                {!!held.data?.length && (
                  <span className="absolute -top-1.5 -right-1.5 grid size-5 place-items-center rounded-full bg-primary text-[11px] font-semibold text-primary-foreground">
                    {held.data.length}
                  </span>
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent>Held sales (F8)</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="outline" className="h-14 w-14 rounded-xl" onClick={noSale}>
                <Inbox className="size-5" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>No sale: open drawer</TooltipContent>
          </Tooltip>
        </div>
        <ScrollArea className="min-h-0 flex-1">
          <div className="grid grid-cols-[repeat(auto-fill,minmax(210px,1fr))] gap-3 p-4">
            {(results.data ?? []).map((it) => {
              const left = it.stock_milli - inCart(it.variant_id)
              const author = it.attributes?.author || it.attributes?.brand || it.attributes?.generic_name
              return (
                <button
                  key={it.variant_id}
                  onClick={() => {
                    add(it, 1000)
                    focusSearch()
                  }}
                  className="group flex min-h-[124px] flex-col rounded-xl border bg-card p-3.5 text-left transition-all hover:-translate-y-px hover:border-primary/40 hover:shadow-md active:translate-y-0"
                >
                  <div className="line-clamp-2 font-medium leading-snug">
                    {it.name}
                    {it.variant_name && <span className="text-muted-foreground"> · {it.variant_name}</span>}
                  </div>
                  {author && <div className="mt-0.5 truncate text-xs text-muted-foreground">{author}</div>}
                  <div className="mt-auto flex items-end justify-between pt-3">
                    <div className="text-base font-semibold tabular">{fmt(it.price_minor)}</div>
                    {it.track_stock && (
                      <span
                        className={cn(
                          "rounded-md px-1.5 py-0.5 text-[11px] font-medium tabular",
                          left <= 0 ? "bg-red-500/12 text-red-700 dark:text-red-300" : left <= 5000 ? "bg-amber-500/15 text-amber-800 dark:text-amber-300" : "bg-muted text-muted-foreground",
                        )}
                      >
                        {left <= 0 ? "Out of stock" : `${formatQty(left)} left`}
                      </span>
                    )}
                  </div>
                </button>
              )
            })}
          </div>
          {results.data && results.data.length === 0 && (
            <div className="flex flex-col items-center gap-2 py-24 text-muted-foreground">
              <PackageOpen className="size-10 opacity-40" />
              {debounced ? `No products match “${debounced}”` : "No products yet. Add them under Products."}
            </div>
          )}
        </ScrollArea>
      </section>

      {/* Cart side */}
      <aside className="flex min-h-0 flex-col border-l bg-card">
        <div className="flex h-14 items-center justify-between gap-2 border-b px-4">
          <div className="flex items-center gap-2 font-semibold">
            <ShoppingBag className="size-4.5 text-primary" />
            Current sale
            {cart.lines.length > 0 && <span className="rounded-full bg-accent px-2 text-xs text-accent-foreground tabular">{formatQty(itemCount)}</span>}
          </div>
          <div className="flex items-center gap-1">
            {cart.lines.length > 0 && (
              <>
                <Button variant="ghost" size="sm" onClick={() => setDialog("hold")}>
                  <Archive /> Hold
                </Button>
                <Button variant="ghost" size="sm" onClick={clear}>
                  <X /> Clear
                </Button>
              </>
            )}
          </div>
        </div>

        <button
          onClick={() => setDialog("customer")}
          className="flex items-center gap-3 border-b px-4 py-2.5 text-left text-sm transition-colors hover:bg-muted/60"
        >
          <div className={cn("grid size-8 place-items-center rounded-full", cart.customer ? "bg-accent text-accent-foreground" : "bg-muted text-muted-foreground")}>
            <UserRound className="size-4" />
          </div>
          {cart.customer ? (
            <div className="min-w-0 flex-1">
              <div className="truncate font-medium">{cart.customer.name}</div>
              <div className="truncate text-xs text-muted-foreground">
                {cart.customer.phone || "No phone"}
                {cart.customer.balance_minor > 0 && ` · owes ${fmt(cart.customer.balance_minor)}`}
              </div>
            </div>
          ) : (
            <span className="flex-1 text-muted-foreground">Add customer (F9)</span>
          )}
          {cart.customer && (
            <X
              className="size-4 text-muted-foreground hover:text-foreground"
              onClick={(e) => {
                e.stopPropagation()
                setCart((c) => ({ ...c, customer: null }))
              }}
            />
          )}
        </button>

        <ScrollArea className="min-h-0 flex-1">
          {cart.lines.length === 0 ? (
            <div className="flex flex-col items-center gap-3 px-8 py-20 text-center text-muted-foreground">
              <Barcode className="size-12 opacity-30" />
              <div>
                <div className="font-medium text-foreground">Scan an item to begin</div>
                <div className="text-sm">or tap a product on the left</div>
              </div>
            </div>
          ) : (
            <ul className="divide-y">
              {cart.lines.map((l, idx) => {
                const over = l.item.track_stock && inCart(l.item.variant_id) > l.item.stock_milli
                const overridden = l.price_override !== undefined && l.price_override !== l.item.price_minor
                return (
                  <li
                    key={l.item.variant_id + idx}
                    onClick={() => setSelected(l.item.variant_id)}
                    className={cn("group px-4 py-3 transition-colors", selected === l.item.variant_id ? "bg-accent/60" : "hover:bg-muted/50")}
                  >
                    <div className="flex items-start gap-3">
                      <button className="min-w-0 flex-1 text-left" onClick={() => setLineEdit(l)}>
                        <div className="truncate font-medium">{l.item.name}</div>
                        <div className="text-xs text-muted-foreground tabular">
                          {l.item.variant_name && `${l.item.variant_name} · `}
                          {overridden && <span className="mr-1 line-through">{fmt(l.item.price_minor)}</span>}
                          <span className={cn(overridden && "font-medium text-amber-700 dark:text-amber-400")}>{fmt(unitPrice(l))}</span>
                          {l.item.unit !== "pc" && ` / ${l.item.unit}`}
                          {over && <span className="ml-1.5 text-amber-700 dark:text-amber-400">· only {formatQty(Math.max(0, l.item.stock_milli))} in stock</span>}
                        </div>
                      </button>
                      <div className="text-right font-semibold tabular">{fmt(lineGross(l.qty_milli, unitPrice(l)))}</div>
                    </div>
                    <div className="mt-2 flex items-center gap-1.5">
                      <Button aria-label={`One less ${l.item.name}`} variant="outline" size="icon-sm" onClick={(e) => (e.stopPropagation(), updateLine(idx, { qty_milli: l.qty_milli - 1000 }))}>
                        <Minus />
                      </Button>
                      <button onClick={(e) => (e.stopPropagation(), setLineEdit(l))} className="h-7 min-w-12 rounded-md border px-2 text-sm font-semibold tabular hover:bg-muted">
                        {formatQty(l.qty_milli)}
                      </button>
                      <Button aria-label={`One more ${l.item.name}`} variant="outline" size="icon-sm" onClick={(e) => (e.stopPropagation(), updateLine(idx, { qty_milli: l.qty_milli + 1000 }))}>
                        <Plus />
                      </Button>
                      <Button aria-label={`Remove ${l.item.name}`}
                        variant="ghost"
                        size="icon-sm"
                        className="ml-auto text-muted-foreground opacity-0 group-hover:opacity-100 focus-visible:opacity-100 hover:text-destructive"
                        onClick={(e) => (e.stopPropagation(), updateLine(idx, null))}
                      >
                        <Trash2 />
                      </Button>
                    </div>
                  </li>
                )
              })}
            </ul>
          )}
        </ScrollArea>

        <div className="space-y-3 border-t bg-muted/30 p-4">
          <div className="space-y-1.5 text-sm">
            <Row label="Subtotal" value={fmt(totals.subtotal)} />
            {cart.discount > 0 && <Row label="Discount" value={"−" + fmt(cart.discount)} className="text-emerald-700 dark:text-emerald-400" />}
            {mode === "exclusive" && totals.tax > 0 && <Row label={taxName} value={fmt(totals.tax)} />}
            {mode === "inclusive" && totals.tax > 0 && <Row label={`${taxName} (included)`} value={fmt(totals.tax)} className="text-muted-foreground" />}
          </div>
          <div className="flex items-baseline justify-between">
            <span className="text-sm font-medium text-muted-foreground">Total</span>
            <span className="text-3xl font-semibold tracking-tight tabular">{fmt(totals.total)}</span>
          </div>
          <div className="flex gap-2">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" className="h-14 px-4" disabled={!cart.lines.length} onClick={() => setDialog("discount")}>
                  <BadgePercent className="size-5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Discount</TooltipContent>
            </Tooltip>
            <Button className="h-14 flex-1 text-lg" disabled={!cart.lines.length} onClick={() => setDialog("pay")}>
              Charge {fmt(totals.total)}
              <kbd className="ml-2 rounded bg-primary-foreground/15 px-1.5 py-0.5 text-[11px] font-normal">F12</kbd>
            </Button>
          </div>
        </div>
      </aside>

      <PayDialog
        open={dialog === "pay"}
        onOpenChange={(o) => {
          setDialog(o ? "pay" : null)
          if (!o) focusSearch()
        }}
        lines={cart.lines}
        discount={cart.discount}
        total={totals.total}
        customer={cart.customer}
        approvalToken={cart.approvalToken}
        needsApproval={cart.discount > 0 || cart.lines.some((l) => l.price_override !== undefined)}
        onNeedCustomer={() => setDialog("customer")}
        onDone={clear}
      />
      <DiscountDialog
        open={dialog === "discount"}
        subtotal={totals.subtotal}
        current={cart.discount}
        currency={currency}
        onClose={() => (setDialog(null), focusSearch())}
        onApply={applyDiscount}
      />
      <CustomerPicker
        open={dialog === "customer"}
        onClose={() => (setDialog(null), focusSearch())}
        onPick={(customer) => setCart((c) => ({ ...c, customer }))}
      />
      <LineDialog
        line={lineEdit}
        onClose={() => (setLineEdit(null), focusSearch())}
        onApply={async (qty, price) => {
          const idx = cart.lines.indexOf(lineEdit!)
          if (idx < 0) return
          const patch: Partial<CartLine> = { qty_milli: qty }
          if (price !== undefined && price !== unitPrice(lineEdit!)) {
            const tok = await requestApproval(`Change the price of ${lineEdit!.item.name} to ${fmt(price)}`, "price_override")
            if (tok === null) return
            patch.price_override = price === lineEdit!.item.price_minor ? undefined : price
            setCart((c) => ({ ...c, approvalToken: tok || c.approvalToken }))
          }
          updateLine(idx, patch)
        }}
      />
      <HoldDialog open={dialog === "hold"} onClose={() => (setDialog(null), focusSearch())} onHold={hold} defaultLabel={cart.customer?.name ?? ""} />
      <Dialog open={dialog === "held"} onOpenChange={(o) => !o && (setDialog(null), focusSearch())}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Held sales</DialogTitle>
            <DialogDescription>Parked carts on this till. Recall one to continue it.</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            {(held.data ?? []).map((h) => (
              <button key={h.id} onClick={() => recall(h.id)} className="flex w-full items-center gap-3 rounded-lg border p-3 text-left hover:border-primary/50 hover:bg-accent/40">
                <ArchiveRestore className="size-4 text-primary" />
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium">{h.label || "Unnamed sale"}</div>
                  <div className="text-xs text-muted-foreground">
                    {formatQty(h.item_count)} items · {h.user_name} · {formatDateTime(h.created_at)}
                  </div>
                </div>
                <div className="font-semibold tabular">{fmt(h.total_minor)}</div>
              </button>
            ))}
            {held.data?.length === 0 && <p className="py-6 text-center text-sm text-muted-foreground">Nothing on hold.</p>}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function Row({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <div className={cn("flex justify-between", className)}>
      <span className="text-muted-foreground">{label}</span>
      <span className="tabular">{value}</span>
    </div>
  )
}

function HoldDialog({ open, onClose, onHold, defaultLabel }: { open: boolean; onClose: () => void; onHold: (label: string) => void; defaultLabel: string }) {
  const [label, setLabel] = useState("")
  useEffect(() => {
    if (open) setLabel(defaultLabel)
  }, [open, defaultLabel])
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Hold this sale</DialogTitle>
          <DialogDescription>Serve the next customer and recall this cart later.</DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            onHold(label)
            onClose()
          }}
          className="space-y-4"
        >
          <div className="space-y-1.5">
            <Label>Label (optional)</Label>
            <Input autoFocus value={label} onChange={(e) => setLabel(e.target.value)} placeholder="e.g. Man in green shirt" />
          </div>
          <DialogFooter>
            <Button type="submit" className="w-full">
              <Archive /> Hold sale
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function LineDialog({ line, onClose, onApply }: { line: CartLine | null; onClose: () => void; onApply: (q: number, priceMinor?: number) => void }) {
  const { currency, fmt } = useMoney()
  const [qty, setQty] = useState("")
  const [price, setPrice] = useState("")
  useEffect(() => {
    if (line) {
      setQty(formatQty(line.qty_milli))
      setPrice((unitPrice(line) / 10 ** currency.digits).toFixed(currency.digits))
    }
  }, [line, currency.digits])
  if (!line) return null
  const q = parseQty(qty)
  const p = parseMoney(price, currency)
  const bad = isNaN(q) || (!line.item.allow_fraction && q % 1000 !== 0) || isNaN(p) || p < 0
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{line.item.name}</DialogTitle>
          <DialogDescription>
            Catalogue price {fmt(line.item.price_minor)}
            {line.item.allow_fraction ? ` per ${line.item.unit}; decimals allowed` : ""}
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (bad) return
            onApply(q, p)
            onClose()
          }}
          className="space-y-4"
        >
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Quantity</Label>
              <Input autoFocus inputMode="decimal" className="h-12 text-center text-xl font-semibold tabular" value={qty} onChange={(e) => setQty(e.target.value)} onFocus={(e) => e.target.select()} />
            </div>
            <div className="space-y-1.5">
              <Label>Unit price ({currency.symbol})</Label>
              <Input inputMode="decimal" className="h-12 text-center text-xl font-semibold tabular" value={price} onChange={(e) => setPrice(e.target.value)} onFocus={(e) => e.target.select()} />
            </div>
          </div>
          {!isNaN(p) && p !== line.item.price_minor && <p className="text-xs text-amber-700 dark:text-amber-400">Changing the price needs a manager.</p>}
          <DialogFooter>
            <Button type="submit" className="h-11 w-full" disabled={bad}>
              Update line
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function DiscountDialog({
  open,
  subtotal,
  current,
  currency,
  onClose,
  onApply,
}: {
  open: boolean
  subtotal: number
  current: number
  currency: { symbol: string; digits: number; code: string }
  onClose: () => void
  onApply: (minor: number) => void
}) {
  const { fmt } = useMoney()
  const [mode, setMode] = useState<"amount" | "percent">("percent")
  const [v, setV] = useState("")
  useEffect(() => {
    if (open) setV("")
  }, [open])
  const amount = mode === "percent" ? Math.round((subtotal * (Number(v) || 0)) / 100) : parseMoney(v, currency)
  const bad = isNaN(amount) || amount < 0 || amount > subtotal
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Discount</DialogTitle>
          <DialogDescription>Applied to the whole sale; cashiers need a manager. Current: {fmt(current)}</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-2 gap-1 rounded-lg bg-muted p-1">
          {(["percent", "amount"] as const).map((m) => (
            <button key={m} onClick={() => setMode(m)} className={cn("rounded-md py-1.5 text-sm font-medium", mode === m ? "bg-card shadow-sm" : "text-muted-foreground")}>
              {m === "percent" ? "Percent %" : `Amount ${currency.symbol}`}
            </button>
          ))}
        </div>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (!bad) {
              onClose()
              onApply(amount)
            }
          }}
          className="space-y-4"
        >
          <Input autoFocus inputMode="decimal" className="h-14 text-center text-2xl font-semibold tabular" value={v} onChange={(e) => setV(e.target.value)} placeholder="0" />
          <div className="text-center text-sm text-muted-foreground">
            Discount {fmt(isNaN(amount) ? 0 : amount)} · subtotal after {fmt(Math.max(0, subtotal - (isNaN(amount) ? 0 : amount)))}
          </div>
          <DialogFooter className="gap-2 sm:justify-between">
            <Button type="button" variant="ghost" onClick={() => (onClose(), onApply(0))}>
              Remove discount
            </Button>
            <Button type="submit" disabled={bad}>
              Apply
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
