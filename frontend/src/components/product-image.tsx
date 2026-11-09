import { useState } from "react"
import { cn } from "@/lib/utils"

// Cover colours for products without a picture, picked from the name so
// the same product always looks the same.
const tints = [
  ["#1f4e5f", "#e8f1f2"],
  ["#5b3a29", "#f6ece4"],
  ["#2f4858", "#eef2f5"],
  ["#4a3f6b", "#efedf6"],
  ["#2d5a3d", "#e9f3ec"],
  ["#7a2e3a", "#f7eaec"],
  ["#665c1e", "#f5f3e3"],
  ["#1e4f7a", "#e8f0f7"],
]

function hash(s: string) {
  let h = 0
  for (const c of s) h = (h * 31 + c.charCodeAt(0)) | 0
  return Math.abs(h)
}

/** A product's picture, or a typographic cover when it has none (or it fails to load). */
export function ProductImage({ hash: img, name, sub, className }: { hash?: string; name: string; sub?: string; className?: string }) {
  const [failed, setFailed] = useState(false)
  if (img && !failed) {
    return <img src={`/img/${img}`} alt="" loading="lazy" onError={() => setFailed(true)} className={cn("rounded-md bg-muted object-cover", className)} />
  }
  const [ink, paper] = tints[hash(name) % tints.length]
  return (
    <div
      aria-hidden
      className={cn("flex flex-col justify-between overflow-hidden rounded-md p-[8%] text-left", className)}
      style={{ background: ink, color: paper, containerType: "inline-size" }}
    >
      <div className="line-clamp-4 font-serif leading-tight font-semibold" style={{ fontSize: "clamp(6px, 13cqi, 15px)" }}>
        {name}
      </div>
      {sub && (
        <div className="truncate opacity-80" style={{ fontSize: "clamp(5px, 9cqi, 11px)" }}>
          {sub}
        </div>
      )}
    </div>
  )
}
