import { useLabelLinker } from "@/lib/label-linker"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { Loader2, TriangleAlert } from "lucide-react"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { AppStateProvider, useAppState } from "@/lib/state"
import { Setup } from "@/screens/setup"
import { Login } from "@/screens/login"
import { Shell } from "@/screens/shell"

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 5_000, refetchOnWindowFocus: false, retry: false } },
})

function Gate() {
  const { state } = useAppState()
  if (!state) {
    return (
      <div className="grid h-full place-items-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    )
  }
  if (state.error) {
    return (
      <div className="grid h-full place-items-center p-8">
        <div className="max-w-md text-center">
          <TriangleAlert className="mx-auto mb-3 size-8 text-destructive" />
          <h1 className="text-lg font-semibold">Zebiventor can’t start</h1>
          <p className="mt-2 text-sm text-muted-foreground">{state.error}</p>
        </div>
      </div>
    )
  }
  if (!state.paired) return <Setup />
  if (!state.user) return <Login />
  return <Shell />
}

export default function App() {
  useLabelLinker()
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider delayDuration={300}>
        <AppStateProvider>
          <Gate />
          <Toaster position="top-center" richColors closeButton />
        </AppStateProvider>
      </TooltipProvider>
    </QueryClientProvider>
  )
}
