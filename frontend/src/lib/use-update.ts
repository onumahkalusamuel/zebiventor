import { useEffect, useState } from "react"
import { api, onUpdateState } from "@/lib/api"
import type { UpdateState } from "@/lib/types"

/** The till's update state: asks the hub once, then follows download progress. */
export function useUpdateState() {
  const [st, setSt] = useState<UpdateState | null>(null)
  useEffect(() => {
    let live = true
    api.getUpdateState().then(
      (s) => live && setSt(s),
      () => {},
    )
    const off = onUpdateState((s) => setSt(s))
    return () => {
      live = false
      off()
    }
  }, [])
  return st
}
