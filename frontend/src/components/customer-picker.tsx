import { useEffect, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Loader2, Phone, Search, UserPlus, UserRound } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, errorMessage } from "@/lib/api"
import { useMoney } from "@/lib/state"
import type { Customer } from "@/lib/types"
import { cn } from "@/lib/utils"

export function CustomerPicker({ open, onClose, onPick }: { open: boolean; onClose: () => void; onPick: (c: Customer | null) => void }) {
  const { fmt } = useMoney()
  const [q, setQ] = useState("")
  const [creating, setCreating] = useState(false)
  const list = useQuery({ queryKey: ["customers", q], queryFn: () => api.searchCustomers(q), enabled: open })
  useEffect(() => {
    if (open) {
      setQ("")
      setCreating(false)
    }
  }, [open])

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{creating ? "New customer" : "Customer"}</DialogTitle>
          <DialogDescription>{creating ? "Saved on this till now and shared with every till." : "Attach a customer to this sale (needed for sales on account)."}</DialogDescription>
        </DialogHeader>
        {creating ? (
          <CustomerForm
            initial={{ name: q.match(/^\d/) ? "" : q, phone: q.match(/^\d/) ? q : "" }}
            onSaved={(c) => {
              onPick(c)
              onClose()
            }}
            onCancel={() => setCreating(false)}
          />
        ) : (
          <div className="space-y-3">
            <div className="relative">
              <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input autoFocus value={q} onChange={(e) => setQ(e.target.value)} placeholder="Name or phone number" className="h-11 pl-9" />
            </div>
            <div className="max-h-80 space-y-1 overflow-y-auto">
              {(list.data ?? []).filter((c) => c.active).map((c) => (
                <button
                  key={c.id}
                  onClick={() => {
                    onPick(c)
                    onClose()
                  }}
                  className="flex w-full items-center gap-3 rounded-lg p-2.5 text-left hover:bg-muted"
                >
                  <div className="grid size-9 place-items-center rounded-full bg-accent text-accent-foreground">
                    <UserRound className="size-4" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-medium">{c.name}</div>
                    <div className="truncate text-xs text-muted-foreground">{[c.phone, c.email].filter(Boolean).join(" · ") || "No contact details"}</div>
                  </div>
                  {c.balance_minor !== 0 && (
                    <div className={cn("text-right text-xs tabular", c.balance_minor > 0 ? "text-amber-700 dark:text-amber-400" : "text-emerald-700")}>
                      owes {fmt(c.balance_minor)}
                    </div>
                  )}
                </button>
              ))}
              {list.data?.length === 0 && <p className="py-6 text-center text-sm text-muted-foreground">No customers found.</p>}
            </div>
            <DialogFooter className="sm:justify-between">
              <Button variant="ghost" onClick={() => (onPick(null), onClose())}>
                No customer
              </Button>
              <Button onClick={() => setCreating(true)}>
                <UserPlus /> New customer
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

export function CustomerForm({
  initial,
  onSaved,
  onCancel,
}: {
  initial: Partial<Customer>
  onSaved: (c: Customer) => void
  onCancel: () => void
}) {
  const qc = useQueryClient()
  const [f, setF] = useState<Partial<Customer>>({ active: true, ...initial })
  const [busy, setBusy] = useState(false)
  const set = (k: keyof Customer, v: string | boolean) => setF((p) => ({ ...p, [k]: v }))

  async function save(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const c = await api.saveCustomer(f)
      toast.success(initial.id ? "Customer updated" : "Customer added")
      qc.invalidateQueries({ queryKey: ["customers"] })
      onSaved(c)
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={save} className="space-y-4">
      <div className="grid grid-cols-2 gap-3">
        <div className="col-span-2 space-y-1.5">
          <Label>Full name</Label>
          <Input autoFocus value={f.name ?? ""} onChange={(e) => set("name", e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <Label>Phone</Label>
          <div className="relative">
            <Phone className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input inputMode="tel" value={f.phone ?? ""} onChange={(e) => set("phone", e.target.value)} className="pl-8" />
          </div>
        </div>
        <div className="space-y-1.5">
          <Label>Email</Label>
          <Input type="email" value={f.email ?? ""} onChange={(e) => set("email", e.target.value)} />
        </div>
        <div className="col-span-2 space-y-1.5">
          <Label>Notes</Label>
          <Input value={f.notes ?? ""} onChange={(e) => set("notes", e.target.value)} placeholder="e.g. school, preferred titles" />
        </div>
      </div>
      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" disabled={busy || !f.name?.trim()}>
          {busy && <Loader2 className="animate-spin" />} Save customer
        </Button>
      </DialogFooter>
    </form>
  )
}
