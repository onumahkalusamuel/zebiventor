import { useState } from "react"
import { Loader2, LogIn } from "lucide-react"
import { toast } from "sonner"
import { LogoMark } from "@/components/brand"
import { SyncPill } from "@/components/sync-pill"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, errorMessage } from "@/lib/api"
import { useLoadedState, useAppState } from "@/lib/state"

export function Login() {
  const state = useLoadedState()
  const { refresh } = useAppState()
  const [username, setUsername] = useState("")
  const [secret, setSecret] = useState("")
  const [busy, setBusy] = useState(false)
  const storeName = state.settings.store_name || state.pairing.store_name

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.login(username, secret)
      await refresh()
    } catch (err) {
      toast.error(errorMessage(err))
      setSecret("")
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="relative grid h-full place-items-center bg-sidebar">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(60%_50%_at_50%_0%,color-mix(in_oklch,var(--sidebar-primary)_18%,transparent),transparent)]" />
      <div className="absolute top-4 right-4">
        <SyncPill status={state.status} />
      </div>
      <form onSubmit={submit} className="relative w-full max-w-sm space-y-6 rounded-2xl border bg-card p-8 shadow-2xl">
        <div className="flex flex-col items-center gap-3 text-center">
          <LogoMark className="size-11" />
          <div>
            <h1 className="text-xl font-semibold tracking-tight">{storeName}</h1>
            <p className="text-sm text-muted-foreground">
              {state.pairing.terminal_name} · {state.pairing.receipt_prefix}
            </p>
          </div>
        </div>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="u">Username</Label>
            <Input id="u" autoFocus autoComplete="username" className="h-11" value={username} onChange={(e) => setUsername(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="p">Password or PIN</Label>
            <Input id="p" type="password" autoComplete="current-password" className="h-11" value={secret} onChange={(e) => setSecret(e.target.value)} />
          </div>
        </div>
        <Button type="submit" className="h-11 w-full text-base" disabled={!username || !secret || busy}>
          {busy ? <Loader2 className="animate-spin" /> : <LogIn />}
          Sign in
        </Button>
        {!state.status.online && (
          <p className="text-center text-xs text-muted-foreground">Hub offline: signing in with the details saved on this till.</p>
        )}
      </form>
      <p className="absolute bottom-4 text-xs text-sidebar-foreground/60">Zebiventor {state.version}</p>
    </div>
  )
}
