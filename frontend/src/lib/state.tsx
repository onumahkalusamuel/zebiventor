import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { api, onSyncStatus } from "./api"
import type { AppState, Currency } from "./types"
import { formatMoney } from "./format"

interface Ctx {
  state: AppState | null
  refresh: () => Promise<void>
}

const AppStateContext = createContext<Ctx>({ state: null, refresh: async () => {} })

export function AppStateProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AppState | null>(null)
  const qc = useQueryClient()
  const lastRev = useRef<number | null>(null)

  const refresh = useCallback(async () => {
    setState(await api.getState())
  }, [])

  useEffect(() => {
    refresh()
    return onSyncStatus((status) => {
      setState((prev) => (prev ? { ...prev, status } : prev))
      // New data arrived from the hub: refetch whatever is on screen.
      if (lastRev.current !== null && status.rev !== lastRev.current) {
        qc.invalidateQueries()
        refresh()
      }
      lastRev.current = status.rev
    })
  }, [refresh, qc])

  return <AppStateContext.Provider value={{ state, refresh }}>{children}</AppStateContext.Provider>
}

export function useAppState() {
  return useContext(AppStateContext)
}

/** Returns state that is known to be loaded (use under the root gate). */
export function useLoadedState(): AppState {
  const { state } = useContext(AppStateContext)
  if (!state) throw new Error("app state not loaded")
  return state
}

export function useMoney() {
  const s = useLoadedState()
  const c: Currency = s.currency?.code ? s.currency : { code: "NGN", symbol: "₦", digits: 2 }
  return {
    currency: c,
    fmt: (minor: number, symbol = true) => formatMoney(minor, c, symbol),
  }
}
