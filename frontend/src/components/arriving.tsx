// Stock on its way to this till's location. Receiving works offline: it is
// queued like a sale and reaches the hub when the connection returns.
import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { AlertTriangle, CloudOff, Loader2, PackageCheck, Truck } from "lucide-react"
import { toast } from "sonner"
import { itemLabel } from "@/components/backoffice"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api, errorMessage } from "@/lib/api"
import { formatDateTime, formatQty, parseQty } from "@/lib/format"
import { useLoadedState } from "@/lib/state"
import type { IncomingTransfer } from "@/lib/types"
import { cn } from "@/lib/utils"

export function ArrivingHere() {
  const state = useLoadedState()
  const list = useQuery({ queryKey: ["incoming", state.status.rev], queryFn: api.incomingTransfers, refetchInterval: 30_000 })
  const [open, setOpen] = useState<IncomingTransfer | null>(null)
  const items = list.data ?? []
  if (!items.length) return null
  return (
    <section className="mx-8 mb-4 rounded-xl border bg-card">
      <div className="flex items-center gap-2 border-b px-4 py-3">
        <Truck className="size-4 text-muted-foreground" />
        <h3 className="text-sm font-medium">On its way here</h3>
        <span className="text-xs text-muted-foreground">Count what arrives and book it in, even if the hub can't be reached.</span>
      </div>
      <div className="divide-y">
        {items.map((t) => (
          <div key={t.id} className="flex items-center gap-4 px-4 py-2.5 text-sm">
            <div className="w-28 font-medium tabular">{t.transfer_no}</div>
            <div className="flex-1">
              from {t.from_location_name} · {formatQty(t.items_milli)} items · sent {formatDateTime(t.created_at)}
              {t.note && <span className="text-muted-foreground"> · {t.note}</span>}
            </div>
            {t.received_here ? (
              <Badge variant="secondary">
                <CloudOff /> Received, waiting to sync
              </Badge>
            ) : (
              <Button size="sm" onClick={() => setOpen(t)}>
                <PackageCheck /> Receive
              </Button>
            )}
          </div>
        ))}
      </div>
      {open && <ReceiveDialog t={open} onClose={() => setOpen(null)} />}
    </section>
  )
}

function ReceiveDialog({ t, onClose }: { t: IncomingTransfer; onClose: () => void }) {
  const qc = useQueryClient()
  const [got, setGot] = useState<Record<string, string>>(() => Object.fromEntries((t.lines ?? []).map((l) => [l.variant_id, formatQty(l.qty_milli)])))
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  const lines = t.lines ?? []
  const parsed = Object.fromEntries(Object.entries(got).map(([k, v]) => [k, parseQty(v)]))
  const bad = lines.some((l) => isNaN(parsed[l.variant_id]) || parsed[l.variant_id] < 0 || parsed[l.variant_id] > l.qty_milli)
  const short = lines.reduce((s, l) => s + Math.max(0, l.qty_milli - (parsed[l.variant_id] ?? l.qty_milli)), 0)

  async function save() {
    setBusy(true)
    try {
      await api.receiveTransferHere(t.id, { note, lines: lines.map((l) => ({ variant_id: l.variant_id, received_qty_milli: parsed[l.variant_id] })) })
      toast.success(`${t.transfer_no} booked in at ${t.to_location_name}`)
      qc.invalidateQueries({ queryKey: ["incoming"] })
      qc.invalidateQueries({ queryKey: ["bo"] })
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && !busy && onClose()}>
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Receive {t.transfer_no}</DialogTitle>
          <DialogDescription>
            From {t.from_location_name}, sent {formatDateTime(t.created_at)} by {t.created_by}. Enter what actually arrived.
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-3">Item</TableHead>
                <TableHead className="text-right">Sent</TableHead>
                <TableHead className="pr-3 text-right">Arrived</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {lines.map((l) => (
                <TableRow key={l.variant_id}>
                  <TableCell className="pl-3">
                    <div className="font-medium">{itemLabel(l)}</div>
                    <div className="text-xs text-muted-foreground">{l.sku}</div>
                  </TableCell>
                  <TableCell className="text-right tabular">{formatQty(l.qty_milli)}</TableCell>
                  <TableCell className="w-32 pr-3">
                    <Input
                      aria-label={`${itemLabel(l)} arrived`}
                      value={got[l.variant_id] ?? ""}
                      onChange={(e) => setGot((g) => ({ ...g, [l.variant_id]: e.target.value }))}
                      onFocus={(e) => e.target.select()}
                      className={cn("h-9 text-right tabular", (isNaN(parsed[l.variant_id]) || parsed[l.variant_id] > l.qty_milli) && "border-destructive")}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        {short > 0 && (
          <p className="flex items-center gap-2 text-sm text-amber-700 dark:text-amber-400">
            <AlertTriangle className="size-4" /> {formatQty(short)} short. It is recorded as missing on the transfer.
          </p>
        )}
        <Input placeholder="Note (e.g. one carton damaged)" value={note} onChange={(e) => setNote(e.target.value)} />
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button onClick={save} disabled={busy || bad}>
            {busy && <Loader2 className="animate-spin" />} Book in at {t.to_location_name}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
