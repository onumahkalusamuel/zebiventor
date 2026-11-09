// Settings: shop type, backups & restore, updates, email.
import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { AlertTriangle, ArchiveRestore, Check, CloudDownload, Download, FolderOpen, HardDriveDownload, Loader2, Plus, RefreshCw, Save, Send, Trash2, Upload } from "lucide-react"
import { toast } from "sonner"
import { HubRequired, QueryError } from "@/components/backoffice"
import { Section, SwitchRow } from "@/components/section"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { api, errorMessage } from "@/lib/api"
import { can, formatDateTime, productFields, shopUnits, timeAgo } from "@/lib/format"
import { hub } from "@/lib/hub"
import { useAppState, useLoadedState } from "@/lib/state"
import { useUpdateState } from "@/lib/use-update"
import type { BackupInfo, EmailSettings, ProductField, ShopTemplate } from "@/lib/types"
import { cn } from "@/lib/utils"

function bytes(n: number) {
  if (n >= 1 << 30) return (n / (1 << 30)).toFixed(1) + " GB"
  if (n >= 1 << 20) return (n / (1 << 20)).toFixed(1) + " MB"
  return Math.max(1, Math.round(n / 1024)) + " KB"
}

// ---------- shop type ----------

export function ShopTypeSettings() {
  const state = useLoadedState()
  const { refresh } = useAppState()
  const templates = useQuery({ queryKey: ["templates"], queryFn: api.shopTemplates })
  const current = state.settings.business_type || "general"
  const [picked, setPicked] = useState(current)
  const [withTax, setWithTax] = useState(false)
  const [busy, setBusy] = useState(false)
  const t = templates.data?.find((x) => x.type === picked)

  async function apply() {
    setBusy(true)
    try {
      const res = await hub.applyTemplate(picked, withTax)
      const added = [...(res.categories ?? []), ...(res.payment_methods ?? [])]
      toast.success(added.length ? `Set up as ${t?.label}. Added: ${added.join(", ")}` : `Set up as ${t?.label}`)
      setTimeout(refresh, 800)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <HubRequired>
      <div className="grid max-w-5xl grid-cols-[1fr_22rem] items-start gap-4">
        <Section title="Type of shop" description="Picks the product details, units, categories and payment methods that suit your business. Applying one only adds what's missing; nothing is removed or renamed.">
          <div className="grid grid-cols-2 gap-2">
            {(templates.data ?? []).map((x) => (
              <button
                key={x.type}
                onClick={() => setPicked(x.type)}
                className={cn("rounded-lg border p-3 text-left transition-colors hover:bg-muted/50", picked === x.type && "border-primary ring-2 ring-primary/30")}
              >
                <div className="flex items-center gap-2 text-sm font-medium">
                  {x.label}
                  {x.type === current && <Badge variant="secondary">Current</Badge>}
                </div>
                <div className="mt-0.5 text-xs text-muted-foreground">{x.description}</div>
              </button>
            ))}
          </div>
        </Section>
        {t && <TemplatePreview t={t} withTax={withTax} setWithTax={setWithTax} busy={busy} onApply={apply} />}
      </div>
      <ProductDetailsEditor />
    </HubRequired>
  )
}

function TemplatePreview({ t, withTax, setWithTax, busy, onApply }: { t: ShopTemplate; withTax: boolean; setWithTax: (v: boolean) => void; busy: boolean; onApply: () => void }) {
  const taxLabel = t.tax_mode === "none" ? "no tax" : `${t.tax_name} ${t.default_tax_bp / 100}% ${t.tax_mode === "inclusive" ? "included in prices" : "added at the till"}`
  return (
    <Section title={t.label}>
      <dl className="space-y-3 text-sm">
        <div>
          <dt className="text-xs text-muted-foreground">Product details</dt>
          <dd>{t.fields.map((f) => f.label).join(", ")}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">Units</dt>
          <dd>{t.units.join(", ")}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">Categories</dt>
          <dd>{t.categories.join(", ")}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">Payment methods</dt>
          <dd>{t.payment_methods.map((m) => m.name).join(", ")}</dd>
        </div>
      </dl>
      <div className="mt-3 border-t pt-2">
        <SwitchRow label="Also use its tax settings" hint={taxLabel} checked={withTax} onChange={setWithTax} />
      </div>
      <Button className="mt-3 w-full" onClick={onApply} disabled={busy}>
        {busy ? <Loader2 className="animate-spin" /> : <Check />} Use this set-up
      </Button>
    </Section>
  )
}

function keyFor(label: string, taken: Set<string>) {
  const base = label.toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, "") || "detail"
  let k = base
  for (let i = 2; taken.has(k); i++) k = `${base}_${i}`
  return k
}

function ProductDetailsEditor() {
  const state = useLoadedState()
  const { refresh } = useAppState()
  const [fields, setFields] = useState<ProductField[]>(() => productFields(state.settings))
  const [units, setUnits] = useState(() => shopUnits(state.settings).join(", "))
  const [busy, setBusy] = useState(false)

  async function save() {
    const taken = new Set<string>()
    const clean = fields
      .filter((f) => f.label.trim())
      .map((f) => {
        const key = f.key || keyFor(f.label, taken)
        taken.add(key)
        return { key, label: f.label.trim(), ...(f.placeholder?.trim() ? { placeholder: f.placeholder.trim() } : {}) }
      })
    const unitList = units.split(",").map((u) => u.trim()).filter(Boolean)
    setBusy(true)
    try {
      await api.saveStoreSettings({ product_fields: JSON.stringify(clean), units: JSON.stringify(unitList) })
      toast.success("Saved. Every till will update.")
      setTimeout(refresh, 800)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Section className="mt-4 max-w-5xl" title="Product details and units" description="The extra details asked for on each product (searchable at the till), and the units products can be sold in.">
      <div className="space-y-2">
        <div className="grid grid-cols-[1fr_1fr_auto] gap-2 text-xs text-muted-foreground">
          <span>Detail</span>
          <span>Example shown in the box</span>
          <span className="w-9" />
        </div>
        {fields.map((f, i) => (
          <div key={i} className="grid grid-cols-[1fr_1fr_auto] gap-2">
            <Input value={f.label} placeholder="e.g. Expiry date" onChange={(e) => setFields((p) => p.map((x, j) => (j === i ? { ...x, label: e.target.value } : x)))} />
            <Input value={f.placeholder ?? ""} onChange={(e) => setFields((p) => p.map((x, j) => (j === i ? { ...x, placeholder: e.target.value } : x)))} />
            <Button variant="ghost" size="icon" aria-label={`Remove ${f.label || "detail"}`} onClick={() => setFields((p) => p.filter((_, j) => j !== i))}>
              <Trash2 />
            </Button>
          </div>
        ))}
        <Button variant="outline" size="sm" disabled={fields.length >= 12} onClick={() => setFields((p) => [...p, { key: "", label: "" }])}>
          <Plus /> Add a detail
        </Button>
        <p className="text-xs text-muted-foreground">Removing a detail hides it from the product form; values already saved on products are kept.</p>
      </div>
      <div className="mt-5 max-w-xl space-y-1.5">
        <Label htmlFor="units">Units (separated by commas)</Label>
        <Input id="units" value={units} onChange={(e) => setUnits(e.target.value)} placeholder="pc, kg, pack, carton" />
      </div>
      <Button className="mt-4" onClick={save} disabled={busy}>
        {busy ? <Loader2 className="animate-spin" /> : <Save />} Save details and units
      </Button>
    </Section>
  )
}

// ---------- backups ----------

export function BackupSettings() {
  return (
    <HubRequired>
      <BackupPanel />
    </HubRequired>
  )
}

function BackupPanel() {
  const state = useLoadedState()
  const qc = useQueryClient()
  const isAdmin = can(state.user?.role, "admin")
  const here = state.hub_running
  const st = useQuery({ queryKey: ["bo", "backups"], queryFn: hub.backups })
  const [folder, setFolder] = useState<string | null>(null)
  const [keep, setKeep] = useState<string | null>(null)
  const [busy, setBusy] = useState<"" | "save" | "now">("")
  const [restore, setRestore] = useState<BackupInfo | null>(null)
  const d = st.data

  async function run(kind: "save" | "now", fn: () => Promise<unknown>, ok: string) {
    setBusy(kind)
    try {
      await fn()
      toast.success(ok)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy("")
      qc.invalidateQueries({ queryKey: ["bo", "backups"] })
    }
  }

  async function pickFolder() {
    const f = await api.chooseFolder("Where should backups be copied?")
    if (f) setFolder(f)
  }

  async function openBackup(path?: string) {
    try {
      const info = path ? await api.inspectBackupFile(path) : await api.chooseBackupFile()
      if (info) setRestore(info)
    } catch (e) {
      toast.error(errorMessage(e))
    }
  }

  if (st.error) return <QueryError error={st.error} />
  if (!d) return <Loader2 className="animate-spin text-muted-foreground" />
  const copyFolder = folder ?? d.copy_folder
  const keepN = keep ?? String(d.keep)

  return (
    <div className="max-w-4xl space-y-4">
      {d.warning && (
        <Alert variant="destructive">
          <AlertTriangle />
          <AlertTitle>Backups need attention</AlertTitle>
          <AlertDescription>{d.warning}</AlertDescription>
        </Alert>
      )}
      <Section
        title="Backups"
        description="The hub saves a full copy of the store's records once a day, on the hub PC and (if set) on a second drive, USB stick or network folder."
        actions={
          isAdmin && (
            <Button variant="outline" disabled={!!busy} onClick={() => run("now", hub.backupNow, "Backup made")}>
              {busy === "now" ? <Loader2 className="animate-spin" /> : <HardDriveDownload />} Back up now
            </Button>
          )
        }
      >
        <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
          <dt className="text-muted-foreground">Last backup</dt>
          <dd>{d.last_at ? `${timeAgo(d.last_at)} (${formatDateTime(d.last_at)})` : "Never"}</dd>
          <dt className="text-muted-foreground">On the hub PC</dt>
          <dd className="truncate text-xs" title={d.folder}>
            {d.folder}
          </dd>
          <dt className="text-muted-foreground">Second copy</dt>
          <dd>{d.copy_folder ? (d.copy_last_at ? `${timeAgo(d.copy_last_at)} to ${d.copy_folder}` : `not yet copied to ${d.copy_folder}`) : "Not set"}</dd>
        </dl>
        {isAdmin && (
          <div className="mt-5 grid grid-cols-[1fr_8rem_auto] items-end gap-3 border-t pt-4">
            <div className="space-y-1.5">
              <Label htmlFor="copy-folder">Copy backups to</Label>
              <div className="flex gap-2">
                <Input id="copy-folder" value={copyFolder} onChange={(e) => setFolder(e.target.value)} placeholder={here ? "Choose a drive or folder" : "Folder path on the hub PC, e.g. E:\\ or \\\\server\\backups"} />
                {here && (
                  <Button variant="outline" onClick={pickFolder}>
                    <FolderOpen /> Choose…
                  </Button>
                )}
              </div>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="keep">Keep (days)</Label>
              <Input id="keep" inputMode="numeric" value={keepN} onChange={(e) => setKeep(e.target.value)} className="text-right tabular" />
            </div>
            <Button disabled={!!busy} onClick={() => run("save", () => hub.saveBackupSettings(copyFolder, Number(keepN) || 0), "Backup settings saved")}>
              {busy === "save" ? <Loader2 className="animate-spin" /> : <Save />} Save
            </Button>
            {!here && <p className="col-span-3 text-xs text-muted-foreground">The folder is on the hub PC ({state.status.hub_url}), not this till.</p>}
          </div>
        )}
      </Section>

      <Section
        title="Saved backups"
        description={here ? "Restore replaces all the store's records with a backup. The current records are backed up first." : "Restoring is done on the hub PC."}
        actions={
          isAdmin && here && (
            <Button variant="outline" onClick={() => openBackup()}>
              <ArchiveRestore /> Restore from a file…
            </Button>
          )
        }
      >
        <div className="divide-y text-sm">
          {(d.files ?? []).slice(0, 12).map((f) => (
            <div key={f.path} className="flex items-center gap-3 py-2">
              <div className="w-40 tabular">{formatDateTime(f.at)}</div>
              <Badge variant={f.where === "copy" ? "secondary" : "outline"}>{f.where === "copy" ? "Second copy" : "Hub PC"}</Badge>
              <div className="flex-1 text-xs text-muted-foreground">{bytes(f.size)}</div>
              {isAdmin && here && (
                <Button variant="ghost" size="sm" onClick={() => openBackup(f.path)}>
                  Restore…
                </Button>
              )}
            </div>
          ))}
          {!d.files?.length && <p className="py-2 text-muted-foreground">No backups yet.</p>}
        </div>
      </Section>
      {restore && <RestoreDialog info={restore} onClose={() => setRestore(null)} />}
    </div>
  )
}

function RestoreDialog({ info, onClose }: { info: BackupInfo; onClose: () => void }) {
  const { refresh } = useAppState()
  const qc = useQueryClient()
  const [sure, setSure] = useState(false)
  const [busy, setBusy] = useState(false)

  async function go() {
    setBusy(true)
    try {
      const safety = await api.restoreBackup(info.path)
      toast.success("Restored. Tills will catch up and re-send recent sales.", { description: `The records from before the restore were saved to ${safety}` })
      qc.invalidateQueries()
      refresh()
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && !busy && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Restore from backup</DialogTitle>
          <DialogDescription className="break-all">{info.path}</DialogDescription>
        </DialogHeader>
        {info.problem ? (
          <Alert variant="destructive">
            <AlertTriangle />
            <AlertTitle>This file can't be restored</AlertTitle>
            <AlertDescription>{info.problem}</AlertDescription>
          </Alert>
        ) : (
          <div className="space-y-3 text-sm">
            <dl className="grid grid-cols-2 gap-y-1">
              <dt className="text-muted-foreground">Store</dt>
              <dd>{info.store_name}</dd>
              <dt className="text-muted-foreground">Last activity in it</dt>
              <dd>{formatDateTime(info.taken_at)}</dd>
              <dt className="text-muted-foreground">Products / sales</dt>
              <dd>
                {info.products} / {info.sales}
              </dd>
            </dl>
            {!info.same_store && (
              <Alert variant="destructive">
                <AlertTriangle />
                <AlertTitle>Different store</AlertTitle>
                <AlertDescription>This backup is from another store. Tills paired with this hub will need pairing again.</AlertDescription>
              </Alert>
            )}
            <p className="text-muted-foreground">
              Sales made at tills since then are re-sent automatically. Anything done only in the back office after {formatDateTime(info.taken_at)} (deliveries, price changes, new
              products, bills) will need entering again. Tills keep selling during the restore.
            </p>
            <label className="flex items-center gap-2">
              <input type="checkbox" checked={sure} onChange={(e) => setSure(e.target.checked)} className="size-4" />
              Replace the store's records with this backup
            </label>
          </div>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          {!info.problem && (
            <Button variant="destructive" disabled={!sure || busy} onClick={go}>
              {busy ? <Loader2 className="animate-spin" /> : <ArchiveRestore />} Restore
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---------- updates ----------

export function TillUpdate({ compact }: { compact?: boolean }) {
  const state = useLoadedState()
  const st = useUpdateState()
  const manager = can(state.user?.role, "admin", "manager")
  if (!st) return null
  const pct = st.total ? Math.round((st.done / st.total) * 100) : 0

  async function act(fn: () => Promise<void>) {
    try {
      await fn()
    } catch (e) {
      toast.error(errorMessage(e))
    }
  }

  const button = !manager ? (
    <span className="text-xs text-muted-foreground">Ask a manager to update this till.</span>
  ) : st.downloading ? (
    <Button size="sm" variant="outline" disabled>
      <Loader2 className="animate-spin" /> Downloading {pct}%
    </Button>
  ) : st.ready ? (
    <Button size="sm" onClick={() => act(api.installUpdate)}>
      <Download /> {st.can_install ? "Install and restart" : "Show installer"}
    </Button>
  ) : (
    <Button size="sm" variant={compact ? "default" : "outline"} onClick={() => act(api.downloadUpdate)}>
      <CloudDownload /> Download from hub
    </Button>
  )

  if (compact) return st.available ? button : null
  return (
    <Section title="This till" description="Tills update from the hub over the local network, so only the hub needs internet.">
      <dl className="grid grid-cols-2 gap-y-1 text-sm">
        <dt className="text-muted-foreground">This till</dt>
        <dd>{st.current}</dd>
        <dt className="text-muted-foreground">Hub</dt>
        <dd>{state.status.hub_version || "—"}</dd>
      </dl>
      <div className="mt-4 flex items-center gap-3">
        {st.available ? (
          <>
            <span className="text-sm">
              Version <b>{st.available}</b> is ready at the hub.
            </span>
            {button}
          </>
        ) : (
          <span className="text-sm text-muted-foreground">This till is up to date with the hub.</span>
        )}
      </div>
      {st.notes && <p className="mt-2 whitespace-pre-line text-sm text-muted-foreground">{st.notes}</p>}
      {st.ready && st.can_install && <p className="mt-2 text-xs text-muted-foreground">The app closes, installs and opens again (Windows may ask for permission). Unsent sales are kept.</p>}
      {st.error && <p className="mt-2 text-sm text-destructive">{st.error}</p>}
    </Section>
  )
}

export function UpdateSettings() {
  return (
    <div className="grid max-w-5xl grid-cols-2 items-start gap-4">
      <TillUpdate />
      <HubRequired>
        <HubUpdates />
      </HubRequired>
    </div>
  )
}

function HubUpdates() {
  const state = useLoadedState()
  const qc = useQueryClient()
  const isAdmin = can(state.user?.role, "admin")
  const st = useQuery({ queryKey: ["bo", "update"], queryFn: hub.updateStatus })
  const [feed, setFeed] = useState<string | null>(null)
  const [busy, setBusy] = useState("")
  const d = st.data

  async function run(kind: string, fn: () => Promise<unknown>, ok?: string) {
    setBusy(kind)
    try {
      const r = await fn()
      if (r !== null && ok) toast.success(ok)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy("")
      qc.invalidateQueries({ queryKey: ["bo", "update"] })
      api.getUpdateState().catch(() => {})
    }
  }

  if (st.error) return <QueryError error={st.error} />
  if (!d) return <Loader2 className="animate-spin text-muted-foreground" />
  const feedURL = feed ?? d.feed_url
  return (
    <Section
      title="Hub"
      description="The hub keeps the newest version for the tills to download."
      actions={
        isAdmin && (
          <Button variant="outline" disabled={!!busy} onClick={() => run("load", api.loadUpdateFile, "Update loaded into the hub")}>
            {busy === "load" ? <Loader2 className="animate-spin" /> : <Upload />} Load update file…
          </Button>
        )
      }
    >
      {!d.signing_key && (
        <p className="mb-3 rounded-md bg-amber-500/10 px-3 py-2 text-xs text-amber-800 dark:text-amber-300">
          This is a test build: it accepts update files that aren't signed. Release builds only accept signed updates.
        </p>
      )}
      <div className="divide-y text-sm">
        {(d.packages ?? []).map((p) => (
          <div key={p.manifest.platform} className="flex items-center gap-3 py-2">
            <div className="font-medium">{p.manifest.version}</div>
            <Badge variant="outline">{p.manifest.platform}</Badge>
            {!p.signed && <Badge variant="secondary">unsigned</Badge>}
            <div className="flex-1 text-xs text-muted-foreground">
              {bytes(p.manifest.size)} · {p.source === "feed" ? "downloaded" : "loaded by hand"} {timeAgo(p.stored_at)}
            </div>
          </div>
        ))}
        {!d.packages?.length && <p className="py-2 text-muted-foreground">No update loaded. The hub is on {d.hub_version}.</p>}
      </div>
      {isAdmin && (
        <div className="mt-4 space-y-2 border-t pt-4">
          <Label htmlFor="feed">Check for new versions at</Label>
          <div className="flex gap-2">
            <Input id="feed" value={feedURL} onChange={(e) => setFeed(e.target.value)} placeholder={d.default_feed || "https://… (leave empty to load update files by hand)"} />
            <Button variant="outline" disabled={!!busy || feedURL === d.feed_url} onClick={() => run("feed", () => hub.saveUpdateFeed(feedURL), "Saved")}>
              <Save />
            </Button>
          </div>
          <div className="flex items-center gap-3">
            <Button variant="outline" size="sm" disabled={!!busy || !(d.feed_url || d.default_feed)} onClick={() => run("check", hub.checkForUpdates, "Checked for updates")}>
              {busy === "check" ? <Loader2 className="animate-spin" /> : <RefreshCw />} Check now
            </Button>
            <span className="text-xs text-muted-foreground">
              {d.last_check_at ? `Last checked ${timeAgo(d.last_check_at)}` : "Checks every 6 hours when an address is set"}
            </span>
          </div>
          {d.last_check_error && <p className="text-xs text-destructive">{d.last_check_error}</p>}
          <p className="text-xs text-muted-foreground">No internet at the hub? Download the update file on any computer, bring it on a USB stick and use Load update file.</p>
        </div>
      )}
    </Section>
  )
}

// ---------- email ----------

export function EmailSettingsPanel() {
  return (
    <HubRequired>
      <EmailForm />
    </HubRequired>
  )
}

function EmailForm() {
  const st = useQuery({ queryKey: ["bo", "email"], queryFn: hub.emailSettings })
  if (st.error) return <QueryError error={st.error} />
  if (!st.data) return <Loader2 className="animate-spin text-muted-foreground" />
  return <EmailFormLoaded initial={st.data} />
}

function EmailFormLoaded({ initial }: { initial: EmailSettings }) {
  const qc = useQueryClient()
  const [f, setF] = useState<EmailSettings>({ ...initial, port: initial.port || 587, password: "" })
  const [testTo, setTestTo] = useState("")
  const [busy, setBusy] = useState("")
  const set = <K extends keyof EmailSettings>(k: K, v: EmailSettings[K]) => setF((p) => ({ ...p, [k]: v }))

  async function run(kind: string, fn: () => Promise<unknown>, ok: string) {
    setBusy(kind)
    try {
      await fn()
      toast.success(ok)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy("")
      qc.invalidateQueries({ queryKey: ["bo", "email"] })
    }
  }

  const field = (k: "host" | "username" | "from" | "from_name", label: string, placeholder?: string) => (
    <div className="space-y-1.5">
      <Label htmlFor={`smtp-${k}`}>{label}</Label>
      <Input id={`smtp-${k}`} value={f[k]} placeholder={placeholder} onChange={(e) => set(k, e.target.value)} />
    </div>
  )

  return (
    <div className="max-w-3xl space-y-4">
      <Section title="Sending email" description="Used to email purchase orders, statements and payment advice. The hub PC sends them, so it needs internet; the password stays on the hub.">
        <div className="grid grid-cols-2 gap-4">
          {field("host", "Mail server", "e.g. smtp.gmail.com")}
          <div className="grid grid-cols-2 gap-2">
            <div className="space-y-1.5">
              <Label htmlFor="smtp-port">Port</Label>
              <Input id="smtp-port" inputMode="numeric" value={String(f.port || "")} onChange={(e) => set("port", Number(e.target.value) || 0)} className="tabular" />
            </div>
            <div className="space-y-1.5">
              <Label>Security</Label>
              <Select value={f.security} onValueChange={(v) => set("security", v as EmailSettings["security"])}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="starttls">STARTTLS (587)</SelectItem>
                  <SelectItem value="tls">TLS (465)</SelectItem>
                  <SelectItem value="none">None</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          {field("username", "Username", "Usually your email address")}
          <div className="space-y-1.5">
            <Label htmlFor="smtp-pass">Password</Label>
            <Input id="smtp-pass" type="password" value={f.password ?? ""} placeholder={initial.password_set ? "Saved (leave empty to keep it)" : ""} onChange={(e) => set("password", e.target.value)} />
          </div>
          {field("from", "Send from (address)", "orders@yourshop.com")}
          {field("from_name", "Send from (name)")}
        </div>
        <p className="mt-3 text-xs text-muted-foreground">Gmail: smtp.gmail.com, port 587, STARTTLS, and an app password (Google Account → Security → App passwords), not your normal password.</p>
        <Button className="mt-4" disabled={!!busy} onClick={() => run("save", () => hub.saveEmailSettings(f), "Email settings saved")}>
          {busy === "save" ? <Loader2 className="animate-spin" /> : <Save />} Save
        </Button>
      </Section>
      <Section title="Send a test">
        <div className="flex max-w-lg gap-2">
          <Input type="email" placeholder="you@example.com" value={testTo} onChange={(e) => setTestTo(e.target.value)} aria-label="Test email address" />
          <Button variant="outline" disabled={!!busy || !testTo.includes("@") || !initial.host} onClick={() => run("test", () => hub.sendTestEmail(testTo), `Test email sent to ${testTo}`)}>
            {busy === "test" ? <Loader2 className="animate-spin" /> : <Send />} Send test
          </Button>
        </div>
        {!initial.host && <p className="mt-2 text-xs text-muted-foreground">Save the mail server first.</p>}
      </Section>
    </div>
  )
}

