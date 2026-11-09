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
  { value: "general", label: "General retail" },
]
