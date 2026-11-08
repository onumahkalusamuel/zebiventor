import { useEffect, useState, type ReactNode } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Banknote, Loader2, Plus, Printer, RefreshCw, Save, Server } from "lucide-react"
import { toast } from "sonner"
import { Page, PageHeader } from "@/components/page"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { api, errorMessage } from "@/lib/api"
import { businessTypes, can, timeAgo } from "@/lib/format"
import { useAppState, useLoadedState } from "@/lib/state"
import type { PaymentMethod, PrinterConfig } from "@/lib/types"
import { cn } from "@/lib/utils"

export function Settings() {
  const state = useLoadedState()
  const role = state.user?.role
  const tabs = [
    { key: "printer", label: "Printer", show: can(role, "admin", "manager") },
    { key: "store", label: "Store", show: can(role, "admin") },
    { key: "payments", label: "Payment methods", show: can(role, "admin") },
    { key: "connection", label: "Connection", show: true },
  ].filter((t) => t.show)

  return (
    <Page>
      <PageHeader title="Settings" />
      <Tabs defaultValue={tabs[0].key} className="px-8 pb-8">
        <TabsList>
          {tabs.map((t) => (
            <TabsTrigger key={t.key} value={t.key}>
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="printer" className="pt-4">
          <PrinterSettings />
        </TabsContent>
        <TabsContent value="store" className="pt-4">
          <StoreSettings />
        </TabsContent>
        <TabsContent value="payments" className="pt-4">
          <PaymentSettings />
        </TabsContent>
        <TabsContent value="connection" className="pt-4">
          <ConnectionSettings />
        </TabsContent>
      </Tabs>
    </Page>
  )
}

function Section({ title, description, children, className }: { title: string; description?: string; children: ReactNode; className?: string }) {
  return (
    <section className={cn("rounded-xl border bg-card p-5", className)}>
      <div className="mb-4">
        <h3 className="font-medium">{title}</h3>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {children}
    </section>
  )
}

function SwitchRow({ label, hint, checked, onChange }: { label: string; hint?: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex items-center justify-between gap-4 py-2">
      <div>
        <div className="text-sm font-medium">{label}</div>
        {hint && <div className="text-xs text-muted-foreground">{hint}</div>}
      </div>
      <Switch checked={checked} onCheckedChange={onChange} />
    </label>
  )
}

// ---------- printer ----------

function PrinterSettings() {
  const loaded = useQuery({ queryKey: ["printer"], queryFn: api.getPrinterConfig })
  const printers = useQuery({ queryKey: ["printers"], queryFn: api.listPrinters })
  const [c, setC] = useState<PrinterConfig | null>(null)
  const [preview, setPreview] = useState("")
  const [busy, setBusy] = useState<"" | "save" | "test">("")

  useEffect(() => {
    if (loaded.data && !c) setC(loaded.data)
  }, [loaded.data, c])
  useEffect(() => {
    if (!c) return
    const t = setTimeout(() => api.testPreview(c).then(setPreview).catch(() => {}), 150)
    return () => clearTimeout(t)
  }, [c])

  if (!c) return <Loader2 className="size-5 animate-spin text-muted-foreground" />
  const set = (patch: Partial<PrinterConfig>) => setC({ ...c, ...patch })
  const setTarget = (patch: Partial<PrinterConfig["target"]>) => setC({ ...c, target: { ...c.target, ...patch } })

  async function save() {
    setBusy("save")
    try {
      await api.savePrinterConfig(c!)
      toast.success("Printer settings saved")
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy("")
    }
  }
  async function test() {
    setBusy("test")
    try {
      await api.testPrint(c!)
      toast.success("Test page sent")
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy("")
    }
  }

  return (
    <div className="grid grid-cols-[1fr_380px] gap-6">
      <div className="space-y-4">
        <Section title="Receipt printer" description="Any ESC/POS thermal printer: USB, network or Bluetooth (paired as a system printer).">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label>Connection</Label>
              <Select value={c.target.kind} onValueChange={(v) => setTarget({ kind: v as PrinterConfig["target"]["kind"], address: "" })}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">No printer</SelectItem>
                  <SelectItem value="system">Installed printer (USB / Bluetooth)</SelectItem>
                  <SelectItem value="network">Network printer (IP address)</SelectItem>
                  <SelectItem value="device">Device path / file</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              {c.target.kind === "system" && (
                <>
                  <Label>Printer</Label>
                  <Select value={c.target.address} onValueChange={(v) => setTarget({ address: v })}>
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder={printers.data?.length ? "Choose a printer" : "No printers installed"} />
                    </SelectTrigger>
                    <SelectContent>
                      {(printers.data ?? []).map((p) => (
                        <SelectItem key={p} value={p}>
                          {p}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </>
              )}
              {c.target.kind === "network" && (
                <>
                  <Label>IP address</Label>
                  <Input value={c.target.address} onChange={(e) => setTarget({ address: e.target.value })} placeholder="192.168.1.50 (port 9100)" />
                </>
              )}
              {c.target.kind === "device" && (
                <>
                  <Label>Path</Label>
                  <Input value={c.target.address} onChange={(e) => setTarget({ address: e.target.value })} placeholder="/dev/usb/lp0" />
                </>
              )}
            </div>
            <div className="space-y-1.5">
              <Label>Paper width</Label>
              <Select value={c.paper} onValueChange={(v) => set({ paper: v as PrinterConfig["paper"] })}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="80mm">80 mm (48 characters)</SelectItem>
                  <SelectItem value="58mm">58 mm (32 characters)</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>Print mode</Label>
              <Select value={c.mode} onValueChange={(v) => set({ mode: v as PrinterConfig["mode"] })}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="raster">Graphic (recommended: ₦ sign, QR, exact layout)</SelectItem>
                  <SelectItem value="text">Text (fastest, plain characters)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </Section>
        <Section title="Behaviour">
          <div className="divide-y">
            <SwitchRow label="Print receipt automatically" hint="After every completed sale" checked={c.auto_print} onChange={(v) => set({ auto_print: v })} />
            <SwitchRow label="Open cash drawer" hint="For payment methods set to open the drawer (cash)" checked={c.open_drawer} onChange={(v) => set({ open_drawer: v })} />
            <SwitchRow label="Cut paper" hint="Partial cut after each receipt" checked={c.cut} onChange={(v) => set({ cut: v })} />
            <div className="flex items-center justify-between py-2">
              <div>
                <div className="text-sm font-medium">Feed before cut</div>
                <div className="text-xs text-muted-foreground">Raise if the last line is cut off</div>
              </div>
              <Input type="number" min={0} max={10} className="w-20 text-right" value={c.feed_lines} onChange={(e) => set({ feed_lines: Number(e.target.value) })} />
            </div>
          </div>
        </Section>
        <div className="flex gap-2">
          <Button onClick={save} disabled={!!busy}>
            {busy === "save" ? <Loader2 className="animate-spin" /> : <Save />} Save
          </Button>
          <Button variant="outline" onClick={test} disabled={!!busy || c.target.kind === "none"}>
            {busy === "test" ? <Loader2 className="animate-spin" /> : <Printer />} Print test page
          </Button>
        </div>
      </div>
      <div className="space-y-2">
        <Label className="text-muted-foreground">Preview (exactly what prints)</Label>
        <div className="max-h-[70vh] overflow-y-auto rounded-xl bg-muted p-4">
          {preview && <img src={preview} alt="" className="mx-auto w-full bg-white shadow-md [image-rendering:pixelated]" />}
        </div>
      </div>
    </div>
  )
}

// ---------- store ----------

const storeKeys = ["store_name", "business_type", "store_address", "store_phone", "store_email", "tax_id", "currency_code", "currency_symbol", "receipt_header", "receipt_footer"]

function StoreSettings() {
  const state = useLoadedState()
  const { refresh } = useAppState()
  const [v, setV] = useState<Record<string, string>>(() => Object.fromEntries(storeKeys.map((k) => [k, state.settings[k] ?? ""])))
  const [busy, setBusy] = useState(false)
  const set = (k: string, val: string) => setV((p) => ({ ...p, [k]: val }))

  async function save() {
    setBusy(true)
    try {
      await api.saveStoreSettings(v)
      toast.success("Store settings saved. Every till will update.")
      setTimeout(refresh, 800)
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  const field = (k: string, label: string, props: Partial<React.ComponentProps<typeof Input>> = {}) => (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      <Input value={v[k]} onChange={(e) => set(k, e.target.value)} {...props} />
    </div>
  )

  return (
    <div className="max-w-3xl space-y-4">
      <Section title="Business">
        <div className="grid grid-cols-2 gap-4">
          {field("store_name", "Store name")}
          <div className="space-y-1.5">
            <Label>Type of business</Label>
            <Select value={v.business_type} onValueChange={(x) => set("business_type", x)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {businessTypes.map((b) => (
                  <SelectItem key={b.value} value={b.value}>
                    {b.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {field("store_address", "Address")}
          {field("store_phone", "Phone")}
          {field("store_email", "Email")}
          {field("tax_id", "Tax ID (TIN)")}
          {field("currency_code", "Currency code")}
          {field("currency_symbol", "Currency symbol")}
        </div>
      </Section>
      <Section title="Receipt text" description="Shown on every printed receipt.">
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <Label>Header (under the address)</Label>
            <textarea
              className="min-h-24 w-full rounded-lg border bg-transparent px-2.5 py-2 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              value={v.receipt_header}
              onChange={(e) => set("receipt_header", e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Footer</Label>
            <textarea
              className="min-h-24 w-full rounded-lg border bg-transparent px-2.5 py-2 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              value={v.receipt_footer}
              onChange={(e) => set("receipt_footer", e.target.value)}
            />
          </div>
        </div>
      </Section>
      <Button onClick={save} disabled={busy || !v.store_name.trim()}>
        {busy ? <Loader2 className="animate-spin" /> : <Save />} Save store settings
      </Button>
    </div>
  )
}

// ---------- payment methods ----------

const kinds: { value: PaymentMethod["kind"]; label: string }[] = [
  { value: "cash", label: "Cash" },
  { value: "card", label: "Card / POS terminal" },
  { value: "transfer", label: "Bank transfer" },
  { value: "mobile_money", label: "Mobile money / wallet" },
  { value: "credit", label: "Customer credit" },
  { value: "voucher", label: "Voucher / gift card" },
  { value: "other", label: "Other" },
]

function PaymentSettings() {
  const qc = useQueryClient()
  const methods = useQuery({ queryKey: ["payment-methods"], queryFn: api.paymentMethods })
  const [draft, setDraft] = useState<Partial<PaymentMethod> | null>(null)

  async function save(m: Partial<PaymentMethod>) {
    try {
      await api.savePaymentMethod(m)
      toast.success("Payment method saved")
      setDraft(null)
      setTimeout(() => qc.invalidateQueries({ queryKey: ["payment-methods"] }), 500)
    } catch (e) {
      toast.error(errorMessage(e))
    }
  }

  return (
    <div className="max-w-3xl space-y-3">
      {(methods.data ?? []).map((m) => (
        <MethodRow key={m.id} m={m} onSave={save} />
      ))}
      {draft ? (
        <MethodRow m={draft as PaymentMethod} onSave={save} onCancel={() => setDraft(null)} isNew />
      ) : (
        <Button variant="outline" onClick={() => setDraft({ id: "", name: "", kind: "other", requires_reference: false, opens_drawer: false, sort_order: (methods.data?.length ?? 0) + 1, active: true })}>
          <Plus /> Add payment method
        </Button>
      )}
      <p className="text-xs text-muted-foreground">Deactivated methods disappear from the till but stay on old receipts and reports.</p>
    </div>
  )
}

function MethodRow({ m, onSave, onCancel, isNew }: { m: PaymentMethod; onSave: (m: PaymentMethod) => void; onCancel?: () => void; isNew?: boolean }) {
  const [f, setF] = useState(m)
  const dirty = JSON.stringify(f) !== JSON.stringify(m) || isNew
  return (
    <div className={cn("grid grid-cols-[auto_1.3fr_1.3fr_auto_auto_auto_auto] items-center gap-3 rounded-xl border bg-card p-3", !f.active && "opacity-60")}>
      <Banknote className="size-4 text-muted-foreground" />
      <Input value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} placeholder="Name shown at the till" />
      <Select value={f.kind} onValueChange={(v) => setF({ ...f, kind: v as PaymentMethod["kind"] })}>
        <SelectTrigger className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {kinds.map((k) => (
            <SelectItem key={k.value} value={k.value}>
              {k.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <label className="flex items-center gap-2 text-xs whitespace-nowrap">
        <Switch checked={f.requires_reference} onCheckedChange={(v) => setF({ ...f, requires_reference: v })} /> Ref. required
      </label>
      <label className="flex items-center gap-2 text-xs whitespace-nowrap">
        <Switch checked={f.opens_drawer} onCheckedChange={(v) => setF({ ...f, opens_drawer: v })} /> Opens drawer
      </label>
      <label className="flex items-center gap-2 text-xs whitespace-nowrap">
        <Switch checked={f.active} onCheckedChange={(v) => setF({ ...f, active: v })} /> Active
      </label>
      <div className="flex gap-1">
        {onCancel && (
          <Button variant="ghost" size="sm" onClick={onCancel}>
            Cancel
          </Button>
        )}
        <Button size="sm" disabled={!dirty || !f.name.trim()} onClick={() => onSave(f)}>
          Save
        </Button>
      </div>
    </div>
  )
}

// ---------- connection ----------

function ConnectionSettings() {
  const state = useLoadedState()
  const { refresh } = useAppState()
  const s = state.status
  const [url, setUrl] = useState(s.hub_url)
  const [busy, setBusy] = useState(false)

  async function saveUrl() {
    setBusy(true)
    try {
      await api.setHubAddress(url)
      toast.success("Hub address updated")
      refresh()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid max-w-4xl grid-cols-2 gap-4">
      <Section title="This till">
        <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
          <dt className="text-muted-foreground">Name</dt>
          <dd>{state.pairing.terminal_name}</dd>
          <dt className="text-muted-foreground">Receipt prefix</dt>
          <dd className="tabular">{state.pairing.receipt_prefix}</dd>
          <dt className="text-muted-foreground">Sync status</dt>
          <dd>{s.revoked ? <Badge variant="destructive">Revoked</Badge> : s.online ? <Badge variant="secondary">Online</Badge> : <Badge variant="outline">Offline</Badge>}</dd>
          <dt className="text-muted-foreground">Last sync</dt>
          <dd>{timeAgo(s.last_sync_at)}</dd>
          <dt className="text-muted-foreground">Waiting to send</dt>
          <dd className="tabular">{s.pending}</dd>
          <dt className="text-muted-foreground">Data revision</dt>
          <dd className="tabular">{s.rev}</dd>
          <dt className="text-muted-foreground">Version</dt>
          <dd>{state.version}</dd>
          <dt className="text-muted-foreground">Data folder</dt>
          <dd className="truncate text-xs" title={state.data_dir}>
            {state.data_dir}
          </dd>
        </dl>
        {s.last_error && !s.online && <p className="mt-4 rounded-lg bg-muted p-2 text-xs text-muted-foreground">{s.last_error}</p>}
        {s.rejected > 0 && (
          <div className="mt-4 flex items-center justify-between rounded-lg border border-red-500/30 bg-red-500/8 p-3 text-sm">
            {s.rejected} item(s) were refused by the hub.
            <Button size="sm" variant="outline" onClick={() => api.retryRejected().then(refresh)}>
              <RefreshCw /> Retry
            </Button>
          </div>
        )}
      </Section>

      <div className="space-y-4">
        {state.mode === "host" && (
          <Section title="Hub on this computer" description="Other tills connect to one of these addresses.">
            <div className="flex items-center gap-2 text-sm">
              <Server className="size-4 text-primary" />
              {state.hub_running ? <span className="text-emerald-700 dark:text-emerald-400">Running on port {state.hub_port}</span> : <span className="text-destructive">Not running: {state.hub_error}</span>}
            </div>
            <div className="mt-3 space-y-1">
              {(state.hub_addresses ?? []).map((a) => (
                <div key={a} className="rounded-md bg-muted px-2.5 py-1.5 font-mono text-sm">
                  {a}:{state.hub_port}
                </div>
              ))}
            </div>
            <p className="mt-3 text-xs text-muted-foreground">Keep this computer on during trading hours. If it’s off, other tills keep selling and catch up when it returns. On Windows, allow Zebiventor through the firewall when asked.</p>
          </Section>
        )}
        <Section title="Hub address" description="Change this if the hub computer got a new IP address. Tills also look for it automatically.">
          <div className="flex gap-2">
            <Input value={url} onChange={(e) => setUrl(e.target.value)} />
            <Button onClick={saveUrl} disabled={busy || url === s.hub_url}>
              {busy && <Loader2 className="animate-spin" />} Update
            </Button>
          </div>
        </Section>
      </div>
    </div>
  )
}
