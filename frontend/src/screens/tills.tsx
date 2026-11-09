import { useEffect, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { KeyRound, Loader2, Lock, MonitorSmartphone, ShieldOff, TriangleAlert } from "lucide-react"
import { toast } from "sonner"
import { Page, PageHeader } from "@/components/page"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api, errorMessage } from "@/lib/api"
import { can, compareVersions, timeAgo } from "@/lib/format"
import { useAppState, useLoadedState } from "@/lib/state"
import { Switch } from "@/components/ui/switch"
import type { PairingCode, TerminalInfo } from "@/lib/types"

export function Tills() {
  const state = useLoadedState()
  const qc = useQueryClient()
  const terminals = useQuery({ queryKey: ["terminals"], queryFn: api.listTerminals, refetchInterval: 20_000 })
  const [pairOpen, setPairOpen] = useState(false)
  const [revoke, setRevoke] = useState<TerminalInfo | null>(null)
  const isAdmin = can(state.user?.role, "admin")

  return (
    <Page>
      <PageHeader
        title="Tills"
        description="Every computer selling for this store. Each till has its own receipt prefix."
        actions={
          <Button className="h-10" onClick={() => setPairOpen(true)}>
            <KeyRound /> Pair a new till
          </Button>
        }
      />
      <div className="space-y-4 px-8 pb-8">
        {isAdmin && terminals.data && <EncryptionPanel tills={terminals.data} />}
        {terminals.error ? (
          <div className="flex items-center gap-3 rounded-xl border border-amber-500/40 bg-amber-500/10 p-4 text-sm">
            <TriangleAlert className="size-5 text-amber-600" />
            {errorMessage(terminals.error)}
          </div>
        ) : (
          <div className="overflow-hidden rounded-xl border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40">
                  <TableHead className="pl-4">Till</TableHead>
                  <TableHead>Prefix</TableHead>
                  <TableHead>Location</TableHead>
                  <TableHead>Last seen</TableHead>
                  <TableHead>Address</TableHead>
                  <TableHead>Version</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="pr-4" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {terminals.isLoading && (
                  <TableRow>
                    <TableCell colSpan={8} className="py-10 text-center">
                      <Loader2 className="mx-auto size-5 animate-spin text-muted-foreground" />
                    </TableCell>
                  </TableRow>
                )}
                {(terminals.data ?? []).map((t) => {
                  const online = t.active && Date.now() - t.last_seen_at < 90_000
                  return (
                    <TableRow key={t.id}>
                      <TableCell className="pl-4 font-medium">
                        <div className="flex items-center gap-2">
                          <MonitorSmartphone className="size-4 text-muted-foreground" />
                          {t.name}
                          {t.id === state.pairing.terminal_id && <Badge variant="secondary">This till</Badge>}
                        </div>
                      </TableCell>
                      <TableCell className="tabular">{t.receipt_prefix}</TableCell>
                      <TableCell className="text-muted-foreground">{t.location_name}</TableCell>
                      <TableCell className="text-muted-foreground">{timeAgo(t.last_seen_at)}</TableCell>
                      <TableCell className="text-muted-foreground tabular">{t.last_ip}</TableCell>
                      <TableCell className="tabular">
                        {t.app_version || "—"}
                        {t.active && !t.encrypted && t.last_seen_at > 0 && (
                          <Badge variant="outline" className="ml-2 border-amber-500/50 text-amber-700 dark:text-amber-400">
                            Not encrypted
                          </Badge>
                        )}
                        {t.active && state.status.hub_version && t.app_version && compareVersions(t.app_version, state.status.hub_version) < 0 && (
                          <Badge variant="outline" className="ml-2 border-amber-500/50 text-amber-700 dark:text-amber-400" title={`The hub is on ${state.status.hub_version}`}>
                            Older than hub
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        {!t.active ? (
                          <Badge variant="outline">Revoked</Badge>
                        ) : online ? (
                          <span className="inline-flex items-center gap-1.5 text-sm text-emerald-700 dark:text-emerald-400">
                            <span className="size-2 rounded-full bg-emerald-500" /> Online
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
                            <span className="size-2 rounded-full bg-muted-foreground/40" /> Offline
                          </span>
                        )}
                      </TableCell>
                      <TableCell className="pr-4 text-right">
                        {isAdmin && t.active && t.id !== state.pairing.terminal_id && (
                          <Button variant="ghost" size="sm" className="text-destructive" onClick={() => setRevoke(t)}>
                            <ShieldOff /> Revoke
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
      {pairOpen && <PairDialog onClose={() => (setPairOpen(false), qc.invalidateQueries({ queryKey: ["terminals"] }))} />}
      <Dialog open={!!revoke} onOpenChange={(o) => !o && setRevoke(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Revoke {revoke?.name}?</DialogTitle>
            <DialogDescription>
              The till stops syncing immediately. Sales already sent are kept. Any sales it made while offline and has not yet sent will stay on that computer until it is paired again.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setRevoke(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={async () => {
                try {
                  await api.revokeTerminal(revoke!.id)
                  toast.success("Till revoked")
                  qc.invalidateQueries({ queryKey: ["terminals"] })
                } catch (e) {
                  toast.error(errorMessage(e))
                }
                setRevoke(null)
              }}
            >
              Revoke till
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Page>
  )
}

function PairDialog({ onClose }: { onClose: () => void }) {
  const state = useLoadedState()
  const [name, setName] = useState("")
  const [code, setCode] = useState<PairingCode | null>(null)
  const [busy, setBusy] = useState(false)
  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [])

  async function create() {
    setBusy(true)
    try {
      setCode(await api.createPairingCode(name))
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  const left = code ? Math.max(0, Math.round((code.expires_at - now) / 1000)) : 0
  const hubUrl = state.mode === "host" && state.hub_addresses?.length ? state.hub_addresses.map((a) => `${a}`).join("  ·  ") : state.status.hub_url.replace(/^https?:\/\//, "")

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Pair a new till</DialogTitle>
          <DialogDescription>On the new computer, install Zebiventor and choose “Join an existing store”.</DialogDescription>
        </DialogHeader>
        {!code ? (
          <div className="space-y-4">
            <div className="space-y-1.5">
              <Label>Name for the new till (optional)</Label>
              <Input autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Counter 2" />
            </div>
            <Button className="h-11 w-full" onClick={create} disabled={busy}>
              {busy ? <Loader2 className="animate-spin" /> : <KeyRound />} Generate pairing code
            </Button>
          </div>
        ) : (
          <div className="space-y-5 text-center">
            <div className="flex justify-center gap-2">
              {code.code.split("").map((d, i) => (
                <div key={i} className="grid h-16 w-12 place-items-center rounded-xl border-2 bg-muted/40 text-3xl font-semibold tabular">
                  {d}
                </div>
              ))}
            </div>
            <div className="text-sm text-muted-foreground">
              {left > 0 ? (
                <>
                  Valid for <span className="font-medium text-foreground tabular">{Math.floor(left / 60)}:{String(left % 60).padStart(2, "0")}</span> · single use
                </>
              ) : (
                <span className="text-destructive">Expired. Generate a new code.</span>
              )}
            </div>
            <div className="rounded-lg bg-muted p-3 text-left text-sm">
              <div className="text-xs text-muted-foreground">If the new till doesn’t find the hub automatically, enter:</div>
              <div className="mt-1 font-medium tabular">{hubUrl}</div>
              {state.status.security_code && (
                <div className="mt-2 text-xs text-muted-foreground">
                  Security code <b className="text-foreground tabular">{state.status.security_code}</b>: the new till shows the same code when it finds this hub.
                </div>
              )}
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

/** Encryption of till ↔ hub traffic, and the switch to refuse plain connections. */
function EncryptionPanel({ tills }: { tills: TerminalInfo[] }) {
  const state = useLoadedState()
  const { refresh } = useAppState()
  const [busy, setBusy] = useState(false)
  const required = state.settings.require_encryption === "1"
  const plain = tills.filter((t) => t.active && !t.encrypted && t.last_seen_at > 0)
  async function toggle(on: boolean) {
    setBusy(true)
    try {
      await api.saveStoreSettings({ require_encryption: on ? "1" : "0" })
      toast.success(on ? "The hub now refuses unencrypted connections" : "Unencrypted connections allowed again")
      setTimeout(refresh, 800)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section className="flex items-center gap-4 rounded-xl border bg-card p-4">
      <Lock className="size-5 shrink-0 text-muted-foreground" />
      <div className="flex-1 text-sm">
        <div className="font-medium">Encrypted connections{state.status.security_code && <span className="font-normal text-muted-foreground"> · security code {state.status.security_code}</span>}</div>
        <div className="text-muted-foreground">
          {plain.length
            ? `${plain.map((t) => t.name).join(", ")} ${plain.length === 1 ? "is" : "are"} still connecting without encryption (an older app version). Update ${plain.length === 1 ? "it" : "them"} before refusing unencrypted connections.`
            : "Every till talks to the hub over an encrypted connection."}
        </div>
      </div>
      <label className="flex items-center gap-2 text-sm">
        Refuse unencrypted
        <Switch checked={required} disabled={busy || (!required && plain.length > 0)} onCheckedChange={toggle} aria-label="Refuse unencrypted connections" />
      </label>
    </section>
  )
}
