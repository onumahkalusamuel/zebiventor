import type { ReactNode } from "react"
import { Switch } from "@/components/ui/switch"
import { cn } from "@/lib/utils"

export function Section({ title, description, children, className, actions }: { title: string; description?: ReactNode; children: ReactNode; className?: string; actions?: ReactNode }) {
  return (
    <section className={cn("rounded-xl border bg-card p-5", className)}>
      <div className="mb-4 flex items-start justify-between gap-4">
        <div>
          <h3 className="font-medium">{title}</h3>
          {description && <p className="text-sm text-muted-foreground">{description}</p>}
        </div>
        {actions && <div className="flex shrink-0 gap-2">{actions}</div>}
      </div>
      {children}
    </section>
  )
}

export function SwitchRow({ label, hint, checked, onChange }: { label: string; hint?: string; checked: boolean; onChange: (v: boolean) => void }) {
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
