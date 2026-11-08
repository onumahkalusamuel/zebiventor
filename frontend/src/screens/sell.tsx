import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { BadgePercent, Barcode, Minus, PackageOpen, Plus, ScanLine, Search, ShoppingBag, Trash2, X } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { api, errorMessage } from "@/lib/api"
import { atLeast, formatQty, parseMoney, parseQty } from "@/lib/format"
import { useLoadedState, useMoney } from "@/lib/state"
import type { Item } from "@/lib/types"
import { cn } from "@/lib/utils"
import { PayDialog } from "./pay-dialog"

export interface CartLine {
  item: Item
  qty_milli: number
}

function lineTotal(l: CartLine) {
  return Math.round((l.qty_milli * l.item.price_minor) / 1000)
}

function useDebounced<T>(value: T, ms: number) {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

export function Sell() {
  const state = useLoadedState()
  const { fmt, currency } = useMoney()
  const [cart, setCart] = useState<CartLine[]>([])
  const [selected, setSelected] = useState<string | null>(null)
  const [query, setQuery] = useState("")
  const [discount, setDiscount] = useState(0)
  const [payOpen, setPayOpen] = useState(false)
  const [discountOpen, setDiscountOpen] = useState(false)
  const [qtyEdit, setQtyEdit] = useState<CartLine | null>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const debounced = useDebounced(query, 120)
  const canDiscount = atLeast(state.user?.role, "manager")

  const results = useQuery({
    queryKey: ["items", debounced],
    queryFn: () => api.searchItems(debounced),
  })

  const subtotal = useMemo(() => cart.reduce((s, l) => s + lineTotal(l), 0), [cart])
  const total = Math.max(0, subtotal - discount)
  const itemCount = cart.reduce((s, l) => s + l.qty_milli, 0)

  const add = useCallback((item: Item, qty = item.pack_qty_milli || 1000) => {
    setCart((c) => {
      const i = c.findIndex((l) => l.item.variant_id === item.variant_id)
      if (i >= 0) {
        const next = [...c]
        next[i] = { ...next[i], qty_milli: next[i].qty_milli + qty }
        return next
      }
      return [...c, { item, qty_milli: qty }]
    })
    setSelected(item.variant_id)
  }, [])

  const focusSearch = () => setTimeout(() => searchRef.current?.focus(), 0)

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

  function setQty(variantId: string, qty: number) {
    setCart((c) => (qty <= 0 ? c.filter((l) => l.item.variant_id !== variantId) : c.map((l) => (l.item.variant_id === variantId ? { ...l, qty_milli: qty } : l))))
  }

  function clear() {
    setCart([])
    setDiscount(0)
    setSelected(null)
    focusSearch()
  }

  // Keyboard: typing anywhere goes to the search box (barcode scanners act
  // as keyboards); F12 = pay; Delete removes the selected line.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (payOpen || discountOpen || qtyEdit) return
      const t = e.target as HTMLElement
      const inField = t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable
      if (e.key === "F12" || (e.key === "Enter" && e.ctrlKey)) {
        e.preventDefault()
        if (cart.length) setPayOpen(true)
        return
      }
      if (!inField && e.key === "Delete" && selected) {
        setQty(selected, 0)
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
  }, [cart.length, payOpen, discountOpen, qtyEdit, selected])

  useEffect(() => {
    focusSearch()
  }, [])

  const cartQty = (variantId: string) => cart.find((l) => l.item.variant_id === variantId)?.qty_milli ?? 0

  return (
    <div className="grid h-full grid-cols-[1fr_440px]">
      {/* Catalogue side */}
      <section className="flex min-h-0 flex-col">
        <div className="border-b bg-card p-4">
          <form
            onSubmit={(e) => {
              e.preventDefault()
              submitSearch()
            }}
            className="relative"
          >
            <Search className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-muted-foreground" />
            <Input
              ref={searchRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === "Escape" && setQuery("")}
              placeholder="Scan a barcode, or search by title, author, ISBN, SKU…"
              className="h-14 rounded-xl pr-24 pl-12 text-lg md:text-lg"
            />
            <div className="pointer-events-none absolute top-1/2 right-4 flex -translate-y-1/2 items-center gap-1.5 text-xs text-muted-foreground">
              <ScanLine className="size-4" /> Ready to scan
            </div>
          </form>
        </div>
        <ScrollArea className="min-h-0 flex-1">
          <div className="grid grid-cols-[repeat(auto-fill,minmax(210px,1fr))] gap-3 p-4">
            {(results.data ?? []).map((it) => {
              const left = it.stock_milli - cartQty(it.variant_id)
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
        <div className="flex h-14 items-center justify-between border-b px-4">
          <div className="flex items-center gap-2 font-semibold">
            <ShoppingBag className="size-4.5 text-primary" />
            Current sale
            {cart.length > 0 && <span className="rounded-full bg-accent px-2 text-xs text-accent-foreground tabular">{formatQty(itemCount)}</span>}
          </div>
          {cart.length > 0 && (
            <Button variant="ghost" size="sm" onClick={clear}>
              <X /> Clear
            </Button>
          )}
        </div>

        <ScrollArea className="min-h-0 flex-1">
          {cart.length === 0 ? (
            <div className="flex flex-col items-center gap-3 px-8 py-24 text-center text-muted-foreground">
              <Barcode className="size-12 opacity-30" />
              <div>
                <div className="font-medium text-foreground">Scan an item to begin</div>
                <div className="text-sm">or tap a product on the left</div>
              </div>
            </div>
          ) : (
            <ul className="divide-y">
              {cart.map((l) => {
                const over = l.item.track_stock && l.qty_milli > l.item.stock_milli
                return (
                  <li
                    key={l.item.variant_id}
                    onClick={() => setSelected(l.item.variant_id)}
                    className={cn("group px-4 py-3 transition-colors", selected === l.item.variant_id ? "bg-accent/60" : "hover:bg-muted/50")}
                  >
                    <div className="flex items-start gap-3">
                      <div className="min-w-0 flex-1">
                        <div className="truncate font-medium">{l.item.name}</div>
                        <div className="text-xs text-muted-foreground tabular">
                          {l.item.variant_name && `${l.item.variant_name} · `}
                          {fmt(l.item.price_minor)}
                          {l.item.unit !== "pc" && ` / ${l.item.unit}`}
                          {over && <span className="ml-1.5 text-amber-700 dark:text-amber-400">· only {formatQty(Math.max(0, l.item.stock_milli))} in stock</span>}
                        </div>
                      </div>
                      <div className="text-right font-semibold tabular">{fmt(lineTotal(l))}</div>
                    </div>
                    <div className="mt-2 flex items-center gap-1.5">
                      <Button variant="outline" size="icon-sm" onClick={(e) => (e.stopPropagation(), setQty(l.item.variant_id, l.qty_milli - 1000))}>
                        <Minus />
                      </Button>
                      <button
                        onClick={(e) => (e.stopPropagation(), setQtyEdit(l))}
                        className="h-7 min-w-12 rounded-md border px-2 text-sm font-semibold tabular hover:bg-muted"
                      >
                        {formatQty(l.qty_milli)}
                      </button>
                      <Button variant="outline" size="icon-sm" onClick={(e) => (e.stopPropagation(), setQty(l.item.variant_id, l.qty_milli + 1000))}>
                        <Plus />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="ml-auto text-muted-foreground opacity-0 group-hover:opacity-100 hover:text-destructive"
                        onClick={(e) => (e.stopPropagation(), setQty(l.item.variant_id, 0))}
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
            <Row label="Subtotal" value={fmt(subtotal)} />
            {discount > 0 && <Row label="Discount" value={"−" + fmt(discount)} className="text-emerald-700 dark:text-emerald-400" />}
          </div>
          <div className="flex items-baseline justify-between">
            <span className="text-sm font-medium text-muted-foreground">Total</span>
            <span className="text-3xl font-semibold tracking-tight tabular">{fmt(total)}</span>
          </div>
          <div className="flex gap-2">
            {canDiscount && (
              <Button variant="outline" className="h-14 px-4" disabled={!cart.length} onClick={() => setDiscountOpen(true)}>
                <BadgePercent className="size-5" />
              </Button>
            )}
            <Button className="h-14 flex-1 text-lg" disabled={!cart.length} onClick={() => setPayOpen(true)}>
              Charge {fmt(total)}
              <kbd className="ml-2 rounded bg-primary-foreground/15 px-1.5 py-0.5 text-[11px] font-normal">F12</kbd>
            </Button>
          </div>
        </div>
      </aside>

      <PayDialog
        open={payOpen}
        onOpenChange={(o) => {
          setPayOpen(o)
          if (!o) focusSearch()
        }}
        cart={cart}
        discount={discount}
        total={total}
        onDone={clear}
      />
      <DiscountDialog
        open={discountOpen}
        subtotal={subtotal}
        current={discount}
        currency={currency}
        onClose={() => (setDiscountOpen(false), focusSearch())}
        onApply={setDiscount}
      />
      <QtyDialog
        line={qtyEdit}
        onClose={() => (setQtyEdit(null), focusSearch())}
        onApply={(q) => qtyEdit && setQty(qtyEdit.item.variant_id, q)}
      />
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

function QtyDialog({ line, onClose, onApply }: { line: CartLine | null; onClose: () => void; onApply: (q: number) => void }) {
  const [v, setV] = useState("")
  useEffect(() => setV(line ? formatQty(line.qty_milli) : ""), [line])
  if (!line) return null
  const q = parseQty(v)
  const bad = isNaN(q) || (!line.item.allow_fraction && q % 1000 !== 0)
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Quantity</DialogTitle>
          <DialogDescription>{line.item.name}{line.item.allow_fraction ? ` (in ${line.item.unit}, decimals allowed)` : ""}</DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (!bad) {
              onApply(q)
              onClose()
            }
          }}
        >
          <Input autoFocus inputMode="decimal" className="h-14 text-center text-2xl font-semibold tabular" value={v} onChange={(e) => setV(e.target.value)} onFocus={(e) => e.target.select()} />
          <DialogFooter className="mt-4">
            <Button type="submit" className="h-11 w-full" disabled={bad}>
              Set quantity
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
          <DialogDescription>Applied to the whole sale. Current: {fmt(current)}</DialogDescription>
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
              onApply(amount)
              onClose()
            }
          }}
          className="space-y-4"
        >
          <Input autoFocus inputMode="decimal" className="h-14 text-center text-2xl font-semibold tabular" value={v} onChange={(e) => setV(e.target.value)} placeholder="0" />
          <div className="text-center text-sm text-muted-foreground">
            Discount {fmt(isNaN(amount) ? 0 : amount)} · new total {fmt(Math.max(0, subtotal - (isNaN(amount) ? 0 : amount)))}
          </div>
          <DialogFooter className="gap-2 sm:justify-between">
            <Button type="button" variant="ghost" onClick={() => (onApply(0), onClose())}>
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
