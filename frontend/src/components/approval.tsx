import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react"
import { Loader2, ShieldCheck } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, errorMessage } from "@/lib/api"
import { atLeast } from "@/lib/format"
import { useLoadedState } from "@/lib/state"

/**
 * requestApproval(reason) resolves to:
 *  - ""      the signed-in user is a manager (self-approved), or the store
 *            does not require approval for this action,
 *  - token   a manager approved on this till (single use, 2 minutes),
 *  - null    cancelled.
 */
type Request = (reason: string, action?: ApprovalAction) => Promise<string | null>

/** Actions whose approval rule can be switched off in Settings. */
export type ApprovalAction = "discount" | "price_override" | "refund" | "paid_out" | "no_sale"

const Ctx = createContext<Request>(async () => null)

export function useApproval() {
  return useContext(Ctx)
}

export function ApprovalProvider({ children }: { children: ReactNode }) {
  const state = useLoadedState()
  const [reason, setReason] = useState<string | null>(null)
  const [username, setUsername] = useState("")
  const [secret, setSecret] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const resolver = useRef<((v: string | null) => void) | null>(null)
  const userRef = useRef<HTMLInputElement>(null)

  // Another dialog closing can hand focus back to its trigger after this
  // one opens; take it back so a manager's typing lands in the form.
  useEffect(() => {
    if (reason === null) return
    const t = setTimeout(() => userRef.current?.focus(), 120)
    return () => clearTimeout(t)
  }, [reason])

  const request = useCallback<Request>(
    (why, action) => {
      if (atLeast(state.user?.role, "manager")) return Promise.resolve("")
      if (action && state.settings["approval_" + action] === "0") return Promise.resolve("")
      resolver.current?.(null) // a newer request supersedes a pending one
      setReason(why)
      setUsername("")
      setSecret("")
      setError("")
      return new Promise((resolve) => (resolver.current = resolve))
    },
    [state.user?.role, state.settings],
  )

  function finish(v: string | null) {
    resolver.current?.(v)
    resolver.current = null
    setReason(null)
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError("")
    try {
      const r = await api.approve(username, secret)
      finish(r.token)
    } catch (err) {
      setError(errorMessage(err))
      setSecret("")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Ctx.Provider value={request}>
      {children}
      <Dialog open={reason !== null} onOpenChange={(o) => !o && finish(null)}>
        <DialogContent className="sm:max-w-sm">
          <form onSubmit={submit} className="space-y-4">
            <DialogHeader>
              <div className="mb-1 grid size-10 place-items-center rounded-full bg-amber-500/15">
                <ShieldCheck className="size-5 text-amber-700 dark:text-amber-400" />
              </div>
              <DialogTitle>Manager approval</DialogTitle>
              <DialogDescription>{reason}</DialogDescription>
            </DialogHeader>
            <div className="space-y-1.5">
              <Label>Manager username</Label>
              <Input ref={userRef} autoFocus value={username} onChange={(e) => setUsername(e.target.value)} className="h-11" />
            </div>
            <div className="space-y-1.5">
              <Label>Password or PIN</Label>
              <Input type="password" value={secret} onChange={(e) => setSecret(e.target.value)} className="h-11" />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => finish(null)}>
                Cancel
              </Button>
              <Button type="submit" disabled={!username || !secret || busy}>
                {busy && <Loader2 className="animate-spin" />} Approve
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Ctx.Provider>
  )
}
