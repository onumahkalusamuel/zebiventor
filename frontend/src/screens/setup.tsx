import { useEffect, useState, type ReactNode } from "react"
import { ArrowLeft, ArrowRight, Check, Loader2, Monitor, Network, Radar, Server, Store } from "lucide-react"
import { toast } from "sonner"
import { Wordmark } from "@/components/brand"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { InputOTP, InputOTPGroup, InputOTPSlot } from "@/components/ui/input-otp"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { api, errorMessage } from "@/lib/api"
import { businessTypes } from "@/lib/format"
import { useAppState } from "@/lib/state"
import type { DiscoveredHub, SetupHostInput } from "@/lib/types"
import { cn } from "@/lib/utils"

type Step = "choose" | "host" | "join"

export function Setup() {
  const [step, setStep] = useState<Step>("choose")
  return (
    <div className="grid h-full grid-cols-[minmax(320px,38%)_1fr]">
      <aside className="relative flex flex-col justify-between overflow-hidden bg-sidebar p-10 text-sidebar-foreground">
        <div className="pointer-events-none absolute -top-32 -right-32 size-96 rounded-full bg-sidebar-primary/10 blur-3xl" />
        <Wordmark className="text-sidebar-accent-foreground" />
        <div className="relative space-y-6">
          <h1 className="text-3xl font-semibold leading-tight tracking-tight text-sidebar-accent-foreground">
            Point of sale and inventory that keeps selling, network or not.
          </h1>
          <ul className="space-y-3 text-sm">
            {[
              "Every till sells from its own database: no network, no problem",
              "Tills sync with the store hub automatically over your LAN",
              "Works for bookshops, groceries, pharmacies and distributors",
              "Crisp 80mm thermal receipts with the ₦ sign, sized to fit",
            ].map((t) => (
              <li key={t} className="flex gap-2.5">
                <Check className="mt-0.5 size-4 shrink-0 text-sidebar-primary" />
                {t}
              </li>
            ))}
          </ul>
        </div>
        <p className="relative text-xs opacity-60">© {new Date().getFullYear()} Zebitech</p>
      </aside>
      <main className="flex items-center justify-center overflow-y-auto p-10">
        <div className="w-full max-w-xl">
          {step === "choose" && <Choose onPick={setStep} />}
          {step === "host" && <HostForm onBack={() => setStep("choose")} />}
          {step === "join" && <JoinForm onBack={() => setStep("choose")} />}
        </div>
      </main>
    </div>
  )
}

function Choose({ onPick }: { onPick: (s: Step) => void }) {
  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold tracking-tight">Set up this computer</h2>
        <p className="mt-1 text-muted-foreground">Each shop has one hub. Every computer is a till.</p>
      </div>
      <div className="grid gap-3">
        <ChoiceCard
          icon={<Server className="size-5" />}
          title="Start a new store"
          body="This computer hosts the store hub and is also a till. Other tills will connect to it. Best on the computer that is switched on most."
          onClick={() => onPick("host")}
        />
        <ChoiceCard
          icon={<Network className="size-5" />}
          title="Join an existing store"
          body="Connect this till to a hub already running on your network, either on another till or on a back-office PC."
          onClick={() => onPick("join")}
        />
      </div>
      <p className="text-xs text-muted-foreground">
        Prefer a dedicated server? Run <code className="rounded bg-muted px-1 py-0.5">zebihub init</code> on that PC, then choose “Join”.
      </p>
    </div>
  )
}

function ChoiceCard({ icon, title, body, onClick }: { icon: ReactNode; title: string; body: string; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="group flex items-start gap-4 rounded-xl border bg-card p-5 text-left transition-all hover:border-primary/50 hover:shadow-sm"
    >
      <div className="grid size-10 shrink-0 place-items-center rounded-lg bg-accent text-accent-foreground">{icon}</div>
      <div className="flex-1">
        <div className="font-medium">{title}</div>
        <p className="mt-1 text-sm text-muted-foreground">{body}</p>
      </div>
      <ArrowRight className="mt-2 size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-primary" />
    </button>
  )
}

function Field({ label, children, hint, className }: { label: string; children: ReactNode; hint?: string; className?: string }) {
  return (
    <div className={cn("space-y-1.5", className)}>
      <Label>{label}</Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

function HostForm({ onBack }: { onBack: () => void }) {
  const { refresh } = useAppState()
  const [busy, setBusy] = useState(false)
  const [f, setF] = useState<SetupHostInput>({
    store_name: "",
    business_type: "bookshop",
    location_name: "Main Store",
    address: "",
    phone: "",
    currency_code: "NGN",
    currency_symbol: "₦",
    admin_name: "",
    admin_username: "admin",
    admin_password: "",
    terminal_name: "Front Till",
    port: 7420,
    demo: false,
  })
  const set = <K extends keyof SetupHostInput>(k: K, v: SetupHostInput[K]) => setF((p) => ({ ...p, [k]: v }))
  const valid = f.store_name.trim() && f.admin_name.trim() && f.admin_username.trim() && f.admin_password.length >= 6

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.setupHost(f)
      toast.success("Your store is ready")
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(false)
      refresh()
    }
  }

  return (
    <form onSubmit={submit} className="space-y-6">
      <BackHeader onBack={onBack} icon={<Store className="size-5" />} title="Start a new store" sub="You can change all of this later in Settings." />
      <section className="grid grid-cols-2 gap-4">
        <Field label="Store name" className="col-span-2">
          <Input autoFocus value={f.store_name} onChange={(e) => set("store_name", e.target.value)} placeholder="e.g. Zebi Books & Stationery" />
        </Field>
        <Field label="Type of business">
          <Select value={f.business_type} onValueChange={(v) => set("business_type", v)}>
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
        </Field>
        <Field label="Branch / location">
          <Input value={f.location_name} onChange={(e) => set("location_name", e.target.value)} />
        </Field>
        <Field label="Address">
          <Input value={f.address} onChange={(e) => set("address", e.target.value)} />
        </Field>
        <Field label="Phone">
          <Input value={f.phone} onChange={(e) => set("phone", e.target.value)} />
        </Field>
      </section>

      <section className="space-y-4 rounded-xl border bg-card p-5">
        <div className="text-sm font-medium">Administrator account</div>
        <div className="grid grid-cols-3 gap-4">
          <Field label="Full name">
            <Input value={f.admin_name} onChange={(e) => set("admin_name", e.target.value)} />
          </Field>
          <Field label="Username">
            <Input value={f.admin_username} onChange={(e) => set("admin_username", e.target.value)} />
          </Field>
          <Field label="Password" hint="At least 6 characters">
            <Input type="password" value={f.admin_password} onChange={(e) => set("admin_password", e.target.value)} />
          </Field>
        </div>
      </section>

      <section className="grid grid-cols-2 gap-4">
        <Field label="Name for this till">
          <Input value={f.terminal_name} onChange={(e) => set("terminal_name", e.target.value)} />
        </Field>
        <label className="flex items-center justify-between gap-3 rounded-lg border px-3 py-2">
          <div>
            <div className="text-sm font-medium">Add sample products</div>
            <div className="text-xs text-muted-foreground">A few books and stationery to try things out</div>
          </div>
          <Switch checked={f.demo} onCheckedChange={(v) => set("demo", v)} />
        </label>
      </section>

      <Button type="submit" size="lg" className="h-11 w-full text-base" disabled={!valid || busy}>
        {busy ? <Loader2 className="animate-spin" /> : <Check />}
        Create store and open this till
      </Button>
    </form>
  )
}

function JoinForm({ onBack }: { onBack: () => void }) {
  const { refresh } = useAppState()
  const [hubs, setHubs] = useState<DiscoveredHub[] | null>(null)
  const [scanning, setScanning] = useState(false)
  const [url, setUrl] = useState("")
  const [code, setCode] = useState("")
  const [name, setName] = useState("")
  const [busy, setBusy] = useState(false)

  async function scan() {
    setScanning(true)
    try {
      const found = await api.discoverHubs()
      setHubs(found)
      if (found.length === 1 && !url) setUrl(found[0].url)
    } catch {
      setHubs([])
    } finally {
      setScanning(false)
    }
  }
  useEffect(() => {
    scan()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      await api.joinHub(url, code, name)
      toast.success("This till is now connected")
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(false)
      refresh()
    }
  }

  return (
    <form onSubmit={submit} className="space-y-6">
      <BackHeader onBack={onBack} icon={<Monitor className="size-5" />} title="Join an existing store" sub="Get a pairing code from an admin: Tills → Pair a new till." />

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label>Hubs found on this network</Label>
          <Button type="button" variant="ghost" size="sm" onClick={scan} disabled={scanning}>
            {scanning ? <Loader2 className="animate-spin" /> : <Radar />}
            Scan again
          </Button>
        </div>
        <div className="space-y-2">
          {hubs === null || scanning ? (
            <div className="flex items-center gap-2 rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" /> Looking for hubs…
            </div>
          ) : hubs.length === 0 ? (
            <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
              No hub found automatically. Some networks block discovery; type the hub’s address below (shown on the hub under Settings → Connection).
            </div>
          ) : (
            hubs.map((h) => (
              <button
                type="button"
                key={h.hub_id + h.url}
                onClick={() => setUrl(h.url)}
                className={cn(
                  "flex w-full items-center gap-3 rounded-lg border p-3 text-left transition-colors",
                  url === h.url ? "border-primary bg-accent" : "hover:bg-muted",
                )}
              >
                <Server className="size-4 text-primary" />
                <div className="flex-1">
                  <div className="text-sm font-medium">{h.store_name}</div>
                  <div className="text-xs text-muted-foreground">{h.url}</div>
                </div>
                {url === h.url && <Check className="size-4 text-primary" />}
              </button>
            ))
          )}
        </div>
      </div>

      <Field label="Hub address" hint="e.g. 192.168.1.20 (port 7420 is assumed)">
        <Input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="192.168.1.20" />
      </Field>

      <div className="space-y-1.5">
        <Label>Pairing code</Label>
        <InputOTP maxLength={6} value={code} onChange={setCode}>
          <InputOTPGroup>
            {[0, 1, 2, 3, 4, 5].map((i) => (
              <InputOTPSlot key={i} index={i} className="size-12 text-lg" />
            ))}
          </InputOTPGroup>
        </InputOTP>
      </div>

      <Field label="Name for this till (optional)">
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Counter 2" />
      </Field>

      <Button type="submit" size="lg" className="h-11 w-full text-base" disabled={!url || code.length !== 6 || busy}>
        {busy ? <Loader2 className="animate-spin" /> : <Network />}
        Connect this till
      </Button>
    </form>
  )
}

function BackHeader({ onBack, icon, title, sub }: { onBack: () => void; icon: ReactNode; title: string; sub: string }) {
  return (
    <div className="space-y-4">
      <Button type="button" variant="ghost" size="sm" className="-ml-2" onClick={onBack}>
        <ArrowLeft /> Back
      </Button>
      <div className="flex items-center gap-3">
        <div className="grid size-10 place-items-center rounded-lg bg-accent text-accent-foreground">{icon}</div>
        <div>
          <h2 className="text-xl font-semibold tracking-tight">{title}</h2>
          <p className="text-sm text-muted-foreground">{sub}</p>
        </div>
      </div>
    </div>
  )
}
