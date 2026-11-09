import type { Currency, Role } from "./types"

export function formatMoney(minor: number, c: Currency, withSymbol = true): string {
  const digits = c.digits ?? 2
  const neg = minor < 0
  const abs = Math.abs(minor)
  const div = 10 ** digits
  const whole = Math.floor(abs / div).toLocaleString("en-US")
  const frac = digits > 0 ? "." + String(abs % div).padStart(digits, "0") : ""
  return (neg ? "-" : "") + (withSymbol ? c.symbol : "") + whole + frac
}

/** Parses "1,500.5" → 150050 (for 2 digits). Returns NaN on bad input. */
export function parseMoney(input: string, c: Currency): number {
  const s = input.replace(/[,\s]/g, "").replace(c.symbol, "")
  if (s === "") return 0
  if (!/^\d*(\.\d*)?$/.test(s)) return NaN
  const [w, f = ""] = s.split(".")
  const digits = c.digits ?? 2
  const frac = (f + "0".repeat(digits)).slice(0, digits)
  return Number(w || "0") * 10 ** digits + Number(frac || "0")
}

export function formatQty(milli: number): string {
  const v = milli / 1000
  return Number.isInteger(v) ? String(v) : v.toFixed(3).replace(/0+$/, "")
}

export function parseQty(input: string): number {
  const n = Number(input.replace(/,/g, ""))
  if (!isFinite(n) || n <= 0) return NaN
  return Math.round(n * 1000)
}

export function timeAgo(ms: number): string {
  if (!ms) return "never"
  const s = Math.round((Date.now() - ms) / 1000)
  if (s < 10) return "just now"
  if (s < 60) return `${s}s ago`
  if (s < 3600) return `${Math.round(s / 60)}m ago`
  if (s < 86400) return `${Math.round(s / 3600)}h ago`
  return new Date(ms).toLocaleDateString()
}

export function formatDateTime(ms: number): string {
  return new Date(ms).toLocaleString(undefined, { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })
}

/** A calendar date, e.g. "09 Nov 2026". */
export function formatDate(ms: number): string {
  if (!ms) return ""
  return new Date(ms).toLocaleDateString(undefined, { day: "2-digit", month: "short", year: "numeric" })
}

export const roleLabel: Record<Role, string> = {
  admin: "Administrator",
  manager: "Manager",
  inventory: "Inventory",
  cashier: "Cashier",
}

const rank: Record<Role, number> = { admin: 4, manager: 3, inventory: 2, cashier: 1 }

export function can(role: Role | undefined, ...allowed: Role[]): boolean {
  return !!role && allowed.includes(role)
}

export function atLeast(role: Role | undefined, min: Role): boolean {
  return !!role && rank[role] >= rank[min]
}

/** Product attribute fields offered per business type. */
export const attributeFields: Record<string, { key: string; label: string; placeholder?: string }[]> = {
  bookshop: [
    { key: "author", label: "Author" },
    { key: "isbn", label: "ISBN", placeholder: "978…" },
    { key: "publisher", label: "Publisher" },
    { key: "edition", label: "Edition / Year" },
  ],
  grocery: [
    { key: "brand", label: "Brand" },
    { key: "size", label: "Pack size", placeholder: "e.g. 50cl, 1kg" },
    { key: "origin", label: "Origin" },
  ],
  pharmacy: [
    { key: "generic_name", label: "Generic name" },
    { key: "strength", label: "Strength", placeholder: "e.g. 500mg" },
    { key: "form", label: "Form", placeholder: "Tablet, syrup…" },
    { key: "manufacturer", label: "Manufacturer" },
  ],
  distributor: [
    { key: "brand", label: "Brand" },
    { key: "pack", label: "Pack", placeholder: "e.g. Carton of 24" },
    { key: "supplier_code", label: "Supplier code" },
  ],
  general: [
    { key: "brand", label: "Brand" },
    { key: "model", label: "Model" },
  ],
}

export const businessTypes = [
  { value: "bookshop", label: "Bookshop" },
  { value: "grocery", label: "Grocery / Supermarket" },
  { value: "pharmacy", label: "Pharmacy" },
  { value: "distributor", label: "Distributor / Wholesale" },
  { value: "fashion", label: "Fashion / Boutique" },
  { value: "hardware", label: "Hardware / Building materials" },
  { value: "general", label: "General retail" },
]

/** Product details the shop records (Settings → Shop type), with a fallback for old hubs. */
export function productFields(settings: Record<string, string>): { key: string; label: string; placeholder?: string }[] {
  try {
    const v = JSON.parse(settings.product_fields || "null")
    if (Array.isArray(v)) return v
  } catch {
    /* fall through */
  }
  return attributeFields[settings.business_type || "general"] ?? attributeFields.general
}

/** Units products can be sold in. */
export function shopUnits(settings: Record<string, string>): string[] {
  try {
    const v = JSON.parse(settings.units || "null")
    if (Array.isArray(v) && v.length) return v
  } catch {
    /* fall through */
  }
  return ["pc", "kg", "g", "l", "ml", "m", "pack", "box", "carton", "dozen"]
}

/** Compares dotted versions ("0.4.0", "0.4.1-dev"): negative, 0 or positive. */
export function compareVersions(a: string, b: string): number {
  const split = (v: string) => {
    const [nums, pre = ""] = v.trim().replace(/^v/, "").split("-", 2)
    return { n: nums.split(".").map((x) => parseInt(x, 10) || 0), pre }
  }
  const x = split(a)
  const y = split(b)
  for (let i = 0; i < Math.max(x.n.length, y.n.length); i++) {
    const d = (x.n[i] ?? 0) - (y.n[i] ?? 0)
    if (d) return d
  }
  if (x.pre === y.pre) return 0
  if (!x.pre) return 1
  if (!y.pre) return -1
  return x.pre < y.pre ? -1 : 1
}

/** Short form of a hub certificate fingerprint ("4F2A-91C3"), compared by eye when pairing. */
export function securityCode(fp: string | undefined): string {
  if (!fp || fp.length < 8) return ""
  return (fp.slice(0, 4) + "-" + fp.slice(4, 8)).toUpperCase()
}
