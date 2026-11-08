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
import { useMoney } from "@/lib/state"

export function Sales() {
  const { fmt } = useMoney()
  const [q, setQ] = useState("")
  const [open, setOpen] = useState<string | null>(null)
  const sales = useQuery({ queryKey: ["sales", q], queryFn: () => api.recentSales(q) })
  const today = useQuery({ queryKey: ["today"], queryFn: api.todayTotals, refetchInterval: 15_000 })

  const methods = Object.entries(today.data?.by_method ?? {}).sort((a, b) => b[1] - a[1])

  return (
    <Page>
      <PageHeader title="Sales" description="Sales made on this till. Totals are for today." />
      <div className="space-y-6 px-8 pb-8">
        <div className="grid grid-cols-4 gap-3">
          <StatCard label="Today’s takings" value={fmt(today.data?.total_minor ?? 0)} sub={`${today.data?.count ?? 0} sales`} />
          {methods.slice(0, 2).map(([name, amt]) => (
            <StatCard key={name} label={name} value={fmt(amt)} />
          ))}
          <StatCard
            label="Waiting to sync"
            value={today.data?.unsynced ?? 0}
            sub={today.data?.unsynced ? "Will send when the hub is reachable" : "All sent to the hub"}
          />
        </div>

        <div className="relative max-w-sm">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Find receipt number…" className="h-10 pl-9" />
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
                  <TableRow key={s.id} className="cursor-pointer" onClick={() => setOpen(s.id)}>
                    <TableCell className="pl-4 font-medium tabular">{s.receipt_no}</TableCell>
                    <TableCell className="text-muted-foreground">{formatDateTime(s.created_at)}</TableCell>
                    <TableCell>{s.user_name}</TableCell>
                    <TableCell className="text-muted-foreground">{s.methods}</TableCell>
                    <TableCell className="text-right tabular">{formatQty(s.items)}</TableCell>
                    <TableCell className="text-right font-semibold tabular">{fmt(s.total_minor)}</TableCell>
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
      <SaleSheet id={open} onClose={() => setOpen(null)} />
    </Page>
  )
}

function SaleSheet({ id, onClose }: { id: string | null; onClose: () => void }) {
  const sale = useQuery({ queryKey: ["sale", id], queryFn: () => api.getSale(id!), enabled: !!id })
  const preview = useQuery({ queryKey: ["preview", id], queryFn: () => api.receiptPreview(id!, true), enabled: !!id })
  const [printing, setPrinting] = useState(false)

  async function reprint() {
    if (!id) return
    setPrinting(true)
    try {
      await api.printSale(id, true)
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
          <SheetTitle>Receipt {sale.data?.receipt_no}</SheetTitle>
          <SheetDescription>
            {sale.data && formatDateTime(sale.data.created_at)} · {sale.data?.user_name} · {sale.data?.synced_at ? "synced to hub" : "not yet synced"}
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
            Reprint receipt
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
