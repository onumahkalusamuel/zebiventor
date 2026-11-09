/** Categorical series colours, in the validated order (see index.css --viz-*). */
export const series = ["var(--viz-1)", "var(--viz-2)", "var(--viz-3)", "var(--viz-4)", "var(--viz-5)", "var(--viz-6)", "var(--viz-7)", "var(--viz-8)"]

/** Compact axis numbers: 950, 12K, 3.4M, 1.2B. */
export function compact(n: number): string {
  const a = Math.abs(n)
  if (a >= 1e9) return (n / 1e9).toFixed(a >= 1e10 ? 0 : 1).replace(/\.0$/, "") + "B"
  if (a >= 1e6) return (n / 1e6).toFixed(a >= 1e7 ? 0 : 1).replace(/\.0$/, "") + "M"
  if (a >= 1e4) return (n / 1e3).toFixed(0) + "K"
  return Math.round(n).toLocaleString()
}
