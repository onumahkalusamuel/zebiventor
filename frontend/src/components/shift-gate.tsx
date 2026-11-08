import { useState, type ReactNode } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Loader2, LockOpen, Wallet } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, errorMessage } from "@/lib/api"
import { parseMoney } from "@/lib/format"
import { useMoney } from "@/lib/state"

export function useShift() {
  return useQuery({ queryKey: ["shift"], queryFn: api.currentShift, refetchInterval: 30_000 })
}

/** Renders children only while a shift is open; otherwise offers to open one. */
export function ShiftGate({ children }: { children: ReactNode }) {
  const shift = useShift()
  if (shift.isLoading) {
    return (
      <div className="grid h-full place-items-center">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      </div>
    )
  }
  if (!shift.data) return <OpenShiftCard />
  return <>{children}</>
}

export function OpenShiftCard() {
  const qc = useQueryClient()
  const { currency, fmt } = useMoney()
  const [v, setV] = useState("")
  const [busy, setBusy] = useState(false)
  const amount = parseMoney(v, currency)

  async function open(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.openShift(amount || 0)
      toast.success("Shift opened")
      qc.invalidateQueries()
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid h-full place-items-center p-8">
      <form onSubmit={open} className="w-full max-w-md space-y-6 rounded-2xl border bg-card p-8 shadow-sm">
        <div className="flex items-center gap-3">
          <div className="grid size-11 place-items-center rounded-xl bg-accent text-accent-foreground">
            <Wallet className="size-5" />
          </div>
          <div>
            <h2 className="text-lg font-semibold">Open a shift</h2>
            <p className="text-sm text-muted-foreground">Count the cash in the drawer before the first sale.</p>
          </div>
        </div>
        <div className="space-y-1.5">
          <Label>Opening float ({currency.symbol})</Label>
          <Input autoFocus inputMode="decimal" value={v} onChange={(e) => setV(e.target.value)} placeholder="0.00" className="h-14 text-center text-2xl font-semibold tabular" />
          <p className="text-xs text-muted-foreground">At closing, the drawer is checked against this float plus cash sales, minus refunds and paid-outs.</p>
        </div>
        <Button type="submit" className="h-12 w-full text-base" disabled={busy || isNaN(amount)}>
          {busy ? <Loader2 className="animate-spin" /> : <LockOpen />}
          Open shift{amount > 0 ? ` with ${fmt(amount)}` : ""}
        </Button>
      </form>
    </div>
  )
}
