import { CloudOff, RefreshCw, ShieldAlert, Wifi } from "lucide-react"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { api } from "@/lib/api"
import { timeAgo } from "@/lib/format"
import type { SyncStatus } from "@/lib/types"
import { cn } from "@/lib/utils"

export function SyncPill({ status }: { status: SyncStatus }) {
  let tone = "bg-emerald-500/12 text-emerald-700 dark:text-emerald-300 ring-emerald-600/20"
  let icon = <Wifi className="size-3.5" />
  let label = "Synced"
  let detail = `Connected to hub · last sync ${timeAgo(status.last_sync_at)}`

  if (status.revoked) {
    tone = "bg-red-500/12 text-red-700 dark:text-red-300 ring-red-600/25"
    icon = <ShieldAlert className="size-3.5" />
    label = "Disconnected"
    detail = status.last_error
  } else if (!status.online) {
    tone = "bg-amber-500/15 text-amber-800 dark:text-amber-300 ring-amber-600/25"
    icon = <CloudOff className="size-3.5" />
    label = status.pending > 0 ? `Offline · ${status.pending} queued` : "Offline"
    detail = "Selling continues normally. Sales are kept on this till and sent to the hub automatically when it is reachable."
  } else if (status.pending > 0) {
    icon = <RefreshCw className="size-3.5 animate-spin" />
    label = `Sending ${status.pending}`
  }
  if (status.rejected > 0 && !status.revoked) {
    detail += ` · ${status.rejected} item(s) need attention in Settings → Connection.`
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          onClick={() => api.syncNow()}
          className={cn("inline-flex h-7 items-center gap-1.5 rounded-full px-2.5 text-xs font-medium ring-1 ring-inset transition-colors", tone)}
        >
          {icon}
          {label}
          {status.rejected > 0 && <span className="ml-0.5 size-1.5 rounded-full bg-red-500" />}
        </button>
      </TooltipTrigger>
      <TooltipContent side="bottom" className="max-w-xs">
        {detail}
      </TooltipContent>
    </Tooltip>
  )
}
