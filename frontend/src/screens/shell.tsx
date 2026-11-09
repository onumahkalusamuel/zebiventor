import { useEffect, useState, type ComponentType } from "react"
import { BarChart3, Boxes, Clock, Truck, Warehouse, LogOut, MonitorSmartphone, Moon, Receipt, RotateCcw, Settings as SettingsIcon, ShoppingCart, Sun, UserRound, Users, Wallet } from "lucide-react"
import { ApprovalProvider } from "@/components/approval"
import { useShift } from "@/components/shift-gate"
import { LogoMark } from "@/components/brand"
import { Notices } from "@/components/notices"
import { SyncPill } from "@/components/sync-pill"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { api } from "@/lib/api"
import { can, roleLabel } from "@/lib/format"
import { useAppState, useLoadedState } from "@/lib/state"
import type { Role } from "@/lib/types"
import { cn } from "@/lib/utils"
import { Sell } from "./sell"
import { Sales } from "./sales"
import { Products } from "./products"
import { Team } from "./team"
import { Tills } from "./tills"
import { Settings } from "./settings"
import { Returns } from "./returns"
import { Shift } from "./shift"
import { Customers } from "./customers"
import { Purchasing } from "./purchasing"
import { Stock } from "./stock"
import { Reports } from "./reports"
import { formatDateTime } from "@/lib/format"

type PageKey = "sell" | "returns" | "sales" | "shift" | "customers" | "products" | "stock" | "purchasing" | "reports" | "team" | "tills" | "settings"

const pages: { key: PageKey; label: string; icon: ComponentType<{ className?: string }>; roles: Role[]; el: ComponentType }[] = [
  { key: "sell", label: "Sell", icon: ShoppingCart, roles: ["admin", "manager", "cashier"], el: Sell },
  { key: "returns", label: "Returns", icon: RotateCcw, roles: ["admin", "manager", "cashier"], el: Returns },
  { key: "sales", label: "Sales", icon: Receipt, roles: ["admin", "manager", "cashier"], el: Sales },
  { key: "shift", label: "Shift", icon: Wallet, roles: ["admin", "manager", "cashier"], el: Shift },
  { key: "customers", label: "Customers", icon: UserRound, roles: ["admin", "manager", "cashier"], el: Customers },
  { key: "products", label: "Products", icon: Boxes, roles: ["admin", "manager", "inventory", "cashier"], el: Products },
  { key: "stock", label: "Stock", icon: Warehouse, roles: ["admin", "manager", "inventory"], el: Stock },
  { key: "purchasing", label: "Purchasing", icon: Truck, roles: ["admin", "manager", "inventory"], el: Purchasing },
  { key: "reports", label: "Reports", icon: BarChart3, roles: ["admin", "manager"], el: Reports },
  { key: "team", label: "Team", icon: Users, roles: ["admin"], el: Team },
  { key: "tills", label: "Tills", icon: MonitorSmartphone, roles: ["admin", "manager"], el: Tills },
  { key: "settings", label: "Settings", icon: SettingsIcon, roles: ["admin", "manager", "inventory", "cashier"], el: Settings },
]

function useClock() {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 15_000)
    return () => clearInterval(t)
  }, [])
  return now
}

function useTheme() {
  const [dark, setDark] = useState(() => {
    try {
      return localStorage.getItem("theme") === "dark"
    } catch {
      return false
    }
  })
  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark)
    try {
      localStorage.setItem("theme", dark ? "dark" : "light")
    } catch {
      /* ignore */
    }
  }, [dark])
  return [dark, setDark] as const
}

export function Shell() {
  return (
    <ApprovalProvider>
      <ShellInner />
    </ApprovalProvider>
  )
}

function ShiftChip() {
  const shift = useShift()
  if (shift.isLoading) return null
  return shift.data ? (
    <div className="flex items-center gap-1.5 text-xs text-muted-foreground" title={`Opened ${formatDateTime(shift.data.opened_at)} by ${shift.data.opened_by_name}`}>
      <Clock className="size-3.5" /> Shift open since {new Date(shift.data.opened_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
    </div>
  ) : (
    <div className="text-xs font-medium text-amber-700 dark:text-amber-400">No shift open</div>
  )
}

function ShellInner() {
  const state = useLoadedState()
  const { refresh } = useAppState()
  const role = state.user!.role
  const allowed = pages.filter((p) => can(role, ...p.roles))
  const [page, setPage] = useState<PageKey>(allowed[0].key)
  const now = useClock()
  const [dark, setDark] = useTheme()
  const [settingsTab, setSettingsTab] = useState<string | undefined>()
  const current = allowed.find((p) => p.key === page) ?? allowed[0]
  const Current = current.el

  // F-key navigation, as on most retail systems.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "F1") setPage("sell")
      if (e.key === "F2") setPage("returns")
      if (e.key === "F3") setPage("sales")
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [])

  async function logout() {
    await api.logout()
    refresh()
  }

  return (
    <div className="flex h-full">
      <nav className="flex w-[76px] shrink-0 flex-col items-center gap-0.5 overflow-y-auto bg-sidebar py-3 text-sidebar-foreground">
        <LogoMark className="mb-4 size-9" />
        {allowed.map((p) => (
          <button
            key={p.key}
            onClick={() => setPage(p.key)}
            className={cn(
              "flex w-[64px] shrink-0 flex-col items-center gap-1 rounded-xl py-2 text-[11px] font-medium transition-colors",
              page === p.key
                ? "bg-sidebar-accent text-sidebar-accent-foreground"
                : "text-sidebar-foreground/70 hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
            )}
          >
            <p.icon className={cn("size-5", page === p.key && "text-sidebar-primary")} />
            {p.label}
          </button>
        ))}
        <div className="mt-auto flex flex-col items-center gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <button onClick={() => setDark(!dark)} className="grid size-10 place-items-center rounded-lg text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground">
                {dark ? <Sun className="size-4.5" /> : <Moon className="size-4.5" />}
              </button>
            </TooltipTrigger>
            <TooltipContent side="right">{dark ? "Light mode" : "Dark mode"}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <button onClick={logout} className="grid size-10 place-items-center rounded-lg text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground">
                <LogOut className="size-4.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="right">Sign out</TooltipContent>
          </Tooltip>
        </div>
      </nav>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center gap-4 border-b bg-card px-5">
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold">{state.settings.store_name || state.pairing.store_name}</div>
            <div className="truncate text-xs text-muted-foreground">
              {state.pairing.terminal_name} · {state.pairing.receipt_prefix}
              {state.mode === "host" && " · hosting hub"}
            </div>
          </div>
          <div className="ml-auto flex items-center gap-4">
            <ShiftChip />
            <SyncPill status={state.status} />
            <div className="text-right">
              <div className="text-sm font-medium tabular">{now.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</div>
              <div className="text-xs text-muted-foreground">{now.toLocaleDateString([], { weekday: "short", day: "numeric", month: "short" })}</div>
            </div>
            <div className="flex items-center gap-2.5 border-l pl-4">
              <div className="grid size-8 place-items-center rounded-full bg-accent text-xs font-semibold text-accent-foreground">
                {state.user!.name.split(" ").map((s) => s[0]).slice(0, 2).join("").toUpperCase()}
              </div>
              <div className="leading-tight">
                <div className="text-sm font-medium">{state.user!.name}</div>
                <div className="text-xs text-muted-foreground">{roleLabel[role]}</div>
              </div>
            </div>
          </div>
        </header>
        <Notices
          openSettings={(tab) => {
            setSettingsTab(tab)
            setPage("settings")
          }}
        />
        <main className="min-h-0 flex-1 overflow-hidden">
          {current.key === "settings" ? <Settings key={settingsTab} initialTab={settingsTab} /> : <Current />}
        </main>
      </div>
    </div>
  )
}
