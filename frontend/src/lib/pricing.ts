// Mirrors internal/terminal/sales.go exactly so the till shows the same
// total the backend will charge. All values are integers (minor units / milli-units).

export type TaxMode = "none" | "inclusive" | "exclusive"

const mulDivRound = (a: number, b: number, c: number) => (c === 0 ? 0 : Math.floor((a * b + Math.floor(c / 2)) / c))

export const lineGross = (qtyMilli: number, unitMinor: number) => Math.floor((qtyMilli * unitMinor + 500) / 1000)

export function lineTax(mode: TaxMode, net: number, bp: number): number {
  if (bp <= 0 || net <= 0) return 0
  if (mode === "inclusive") return net - mulDivRound(net, 10000, 10000 + bp)
  if (mode === "exclusive") return mulDivRound(net, bp, 10000)
  return 0
}

/** Proportional split with largest-remainder rounding (stable on ties). */
export function allocate(amount: number, weights: number[]): number[] {
  const out = weights.map(() => 0)
  const total = weights.reduce((s, w) => s + w, 0)
  if (amount === 0 || total === 0) return out
  let given = 0
  const rems = weights.map((w, i) => {
    out[i] = Math.floor((amount * w) / total)
    given += out[i]
    return { i, r: (amount * w) % total }
  })
  rems.sort((a, b) => b.r - a.r)
  for (let k = 0; given < amount; k++) {
    out[rems[k % rems.length].i]++
    given++
  }
  return out
}

export interface PricedLine {
  qty_milli: number
  unit_minor: number
  tax_rate_bp: number
}

export function computeTotals(lines: PricedLine[], discount: number, mode: TaxMode) {
  const gross = lines.map((l) => lineGross(l.qty_milli, l.unit_minor))
  const subtotal = gross.reduce((s, g) => s + g, 0)
  const shares = allocate(Math.min(discount, subtotal), gross)
  let tax = 0
  let net = 0
  gross.forEach((g, i) => {
    const n = g - shares[i]
    tax += lineTax(mode, n, mode === "none" ? 0 : lines[i].tax_rate_bp)
    net += n
  })
  return { subtotal, tax, total: mode === "exclusive" ? net + tax : net }
}
