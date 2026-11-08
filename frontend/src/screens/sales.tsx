import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { CloudCheck, CloudUpload, Loader2, Printer, Receipt, Search } from "lucide-react"
import { toast } from "sonner"
import { EmptyState, Page, PageHeader, StatCard } from "@/components/page"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api, errorMessage } from "@/lib/api"
import { formatDateTime, formatQty } from "@/lib/format"
import { useShift } from "@/components/shift-gate"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import { useMoney } from "@/lib/state"

export function Sales() {
  const { fmt } = useMoney()
  const [q, setQ] = useState("")
  const [openSale, setOpenSale] = useState<{ id: string; kind: "sale" | "return" | "payment" } | null>(null)
  const sales = useQuery({ queryKey: ["sales", q], queryFn: () => api.recentSales(q) })
  const shift = useShift()
  const sum = shift.data?.summary

  return (
    <Page>
      <PageHeader title="Sales" description="Sales, refunds and payments on account taken at this till. Figures are for the current shift." />
      <div className="space-y-6 px-8 pb-8">
        <div className="grid grid-cols-4 gap-3">
          <StatCard
            label="Shift takings"
            value={fmt((sum?.net_sales_minor ?? 0) - (sum?.returns_minor ?? 0) + (sum?.account_payments_minor ?? 0))}
            sub={shift.data ? `${sum?.sales_count ?? 0} sales${sum?.account_payments_count ? ` · ${sum.account_payments_count} on account` : ""}` : "No shift open"}
          />
          {(sum?.by_method ?? []).slice(0, 2).map((m) => (
            <StatCard key={m.method_name} label={m.method_name} value={fmt(m.net_minor)} />
          ))}
          <StatCard label="Refunds" value={fmt(sum?.returns_minor ?? 0)} sub={`${sum?.returns_count ?? 0} this shift`} />
        </div>

        <div className="relative max-w-sm">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Receipt number or customer…" className="h-10 pl-9" />
        </div>

        {sales.data && sales.data.length === 0 ? (
          <EmptyState icon={<Receipt />} title="No sales yet" body="Completed sales from this till appear here." />
        ) : (
          <div className="overflow-hidden rounded-xl border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40">
                  <TableHead className="pl-4">Receipt</TableHead>
                  <TableHead>Time</TableHead>
                  <TableHead>Cashier</TableHead>
                  <TableHead>Paid by</TableHead>
                  <TableHead className="text-right">Items</TableHead>
                  <TableHead className="text-right">Total</TableHead>
                  <TableHead className="w-10 pr-4" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {(sales.data ?? []).map((s) => (
                  <TableRow key={s.id} className="cursor-pointer" onClick={() => setOpenSale({ id: s.id, kind: s.kind })}>
                    <TableCell className="pl-4 font-medium tabular">
                      {s.receipt_no}
                      {s.kind === "return" && <Badge variant="outline" className="ml-2 border-red-500/40 text-red-700 dark:text-red-400">Refund</Badge>}
                      {s.kind === "payment" && <Badge variant="outline" className="ml-2 border-emerald-500/40 text-emerald-700 dark:text-emerald-400">On account</Badge>}
                    </TableCell>
                    <TableCell className="text-muted-foreground">{formatDateTime(s.created_at)}</TableCell>
                    <TableCell>
                      {s.user_name}
                      {s.customer && <div className="text-xs text-muted-foreground">{s.customer}</div>}
                    </TableCell>
                    <TableCell className="text-muted-foreground">{s.methods}</TableCell>
                    <TableCell className="text-right tabular">{s.kind === "payment" ? "" : formatQty(s.items)}</TableCell>
                    <TableCell className={cn("text-right font-semibold tabular", s.total_minor < 0 && "text-red-600")}>{fmt(s.total_minor)}</TableCell>
                    <TableCell className="pr-4">
                      {s.synced ? <CloudCheck className="size-4 text-emerald-600" /> : <CloudUpload className="size-4 text-amber-600" />}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
      <SaleSheet target={openSale} onClose={() => setOpenSale(null)} />
    </Page>
  )
}

function SaleSheet({ target, onClose }: { target: { id: string; kind: "sale" | "return" | "payment" } | null; onClose: () => void }) {
  const id = target?.id ?? null
  const isReturn = target?.kind === "return"
  const isPayment = target?.kind === "payment"
  const sale = useQuery({ queryKey: ["sale", id], queryFn: () => api.getSale(id!), enabled: !!id && !isReturn && !isPayment })
  const preview = useQuery({
    queryKey: ["preview", id],
    queryFn: () => (isReturn ? api.returnPreview(id!, true) : isPayment ? api.accountPaymentPreview(id!, true) : api.receiptPreview(id!, true)),
    enabled: !!id,
  })
  const [printing, setPrinting] = useState(false)

  async function reprint() {
    if (!id) return
    setPrinting(true)
    try {
      await (isReturn ? api.printReturn(id, true) : isPayment ? api.printAccountPayment(id) : api.printSale(id, true))
      toast.success("Reprint sent to printer")
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setPrinting(false)
    }
  }

  return (
    <Sheet open={!!id} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-[480px] gap-0 sm:max-w-[480px]">
        <SheetHeader className="border-b">
          <SheetTitle>{isReturn ? "Refund slip" : isPayment ? "Payment on account" : `Receipt ${sale.data?.receipt_no ?? ""}`}</SheetTitle>
          <SheetDescription>
            {isReturn || isPayment
              ? "Reprints are marked as such."
              : sale.data && `${formatDateTime(sale.data.created_at)} · ${sale.data.user_name} · ${sale.data.synced_at ? "synced to hub" : "not yet synced"}`}
          </SheetDescription>
        </SheetHeader>
        <div className="flex-1 overflow-y-auto bg-muted p-6">
          {preview.data ? (
            <img src={preview.data} alt="" className="mx-auto w-full max-w-[360px] bg-white shadow-md [image-rendering:pixelated]" />
          ) : (
            <Loader2 className="mx-auto mt-20 size-5 animate-spin text-muted-foreground" />
          )}
        </div>
        <div className="border-t p-4">
          <Button className="h-11 w-full" onClick={reprint} disabled={printing}>
            {printing ? <Loader2 className="animate-spin" /> : <Printer />}
            {isReturn ? "Reprint refund slip" : isPayment ? "Reprint payment receipt" : "Reprint receipt"}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
