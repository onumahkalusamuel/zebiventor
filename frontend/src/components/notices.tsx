// Thin notices under the header: things that need someone's attention.
import { useState, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { AlertTriangle, ArchiveRestore, CloudDownload, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { can } from "@/lib/format"
import { hub } from "@/lib/hub"
import { useLoadedState } from "@/lib/state"
import { cn } from "@/lib/utils"
import { TillUpdate } from "@/screens/settings-ops"

function Notice({ tone, icon, children, action, onDismiss }: { tone: "danger" | "warn" | "info"; icon: ReactNode; children: ReactNode; action?: ReactNode; onDismiss?: () => void }) {
  return (
    <div
      role="status"
      className={cn(
        "flex items-center gap-3 border-b px-5 py-1.5 text-sm [&_svg]:size-4 [&_svg]:shrink-0",
        tone === "danger" && "bg-destructive/10 text-destructive",
        tone === "warn" && "bg-amber-500/10 text-amber-900 dark:text-amber-200",
        tone === "info" && "bg-primary/5",
      )}
    >
      {icon}
      <div className="min-w-0 flex-1 truncate">{children}</div>
      {action}
      {onDismiss && (
        <button onClick={onDismiss} aria-label="Dismiss" className="rounded p-1 opacity-60 hover:opacity-100">
          <X />
        </button>
      )}
    </div>
  )
}

export function Notices({ openSettings }: { openSettings: (tab?: string) => void }) {
  const state = useLoadedState()
  const role = state.user?.role
  const manager = can(role, "admin", "manager")
  const [hidden, setHidden] = useState<Record<string, boolean>>({})
  const hide = (k: string) => setHidden((p) => ({ ...p, [k]: true }))
  const backups = useQuery({
    queryKey: ["bo", "backups"],
    queryFn: hub.backups,
    enabled: manager && state.has_session,
    refetchInterval: 10 * 60_000,
    retry: false,
  })
  const s = state.status

  return (
    <>
      {s.version_error && (
        <Notice tone="danger" icon={<AlertTriangle />} action={<TillUpdate compact />}>
          {s.version_error} Sales are kept on this till until then.
        </Notice>
      )}
      {!s.version_error && s.update_version && !hidden.update && (
        <Notice tone="info" icon={<CloudDownload />} action={<TillUpdate compact />} onDismiss={() => hide("update")}>
          Version {s.update_version} is ready to install from the hub.
        </Notice>
      )}
      {s.resent > 0 && !hidden.resent && (
        <Notice tone="info" icon={<ArchiveRestore />} onDismiss={() => hide("resent")}>
          The hub was restored from a backup. This till re-sent {s.resent} recent record{s.resent === 1 ? "" : "s"} so nothing is lost.
        </Notice>
      )}
      {backups.data?.warning && !hidden.backup && (
        <Notice
          tone="warn"
          icon={<AlertTriangle />}
          onDismiss={() => hide("backup")}
          action={
            <Button size="sm" variant="ghost" className="h-7" onClick={() => openSettings("backups")}>
              Backup settings
            </Button>
          }
        >
          {backups.data.warning}
        </Notice>
      )}
    </>
  )
}
