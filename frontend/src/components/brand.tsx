import { cn } from "@/lib/utils"

export function LogoMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={cn("size-8", className)} aria-hidden>
      <rect width="32" height="32" rx="8" className="fill-primary" />
      <path d="M9 10.5h14l-11 11h11" fill="none" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" className="stroke-primary-foreground" />
    </svg>
  )
}

export function Wordmark({ className }: { className?: string }) {
  return (
    <div className={cn("flex items-center gap-2.5", className)}>
      <LogoMark />
      <span className="text-lg font-semibold tracking-tight">Zebiventor</span>
    </div>
  )
}
