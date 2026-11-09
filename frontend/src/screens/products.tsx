import { useEffect, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Boxes, FileUp, Loader2, PackagePlus, Plus, Search, SlidersHorizontal, Trash2, Upload } from "lucide-react"
import { toast } from "sonner"
import { EmptyState, Page, PageHeader } from "@/components/page"
import { ExportButton, LocationSelect } from "@/components/backoffice"
import { hub } from "@/lib/hub"
import type { ImportResult } from "@/lib/types"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api, errorMessage } from "@/lib/api"
import { can, formatQty, parseMoney, parseQty, productFields, shopUnits } from "@/lib/format"
import { useLoadedState, useMoney } from "@/lib/state"
import type { ProductDetail, ProductInput, VariantDetail } from "@/lib/types"
import { cn } from "@/lib/utils"
import { ProductImage } from "@/components/product-image"

export function Products() {
  const state = useLoadedState()
  const { fmt } = useMoney()
  const [q, setQ] = useState("")
  const [editing, setEditing] = useState<ProductDetail | "new" | null>(null)
  const [adjust, setAdjust] = useState<{ p: ProductDetail; v: VariantDetail } | null>(null)
  const [importing, setImporting] = useState(false)
  const products = useQuery({ queryKey: ["products", q], queryFn: () => api.listProducts(q) })
  const canEdit = can(state.user?.role, "admin", "manager", "inventory")

  return (
    <Page>
      <PageHeader
        title="Products"
        description={canEdit ? "Changes are saved at the hub and reach every till within seconds." : "Prices and stock as known by this till."}
        actions={
          canEdit && (
            <>
              {state.has_session && (
                <>
                  <Button variant="outline" className="h-10" onClick={() => setImporting(true)}>
                    <Upload /> Import CSV
                  </Button>
                  <ExportButton name="products.csv" label="Export" build={async () => (await hub.exportProducts()).csv} />
                </>
              )}
              <Button className="h-10" onClick={() => setEditing("new")}>
                <PackagePlus /> New product
              </Button>
            </>
          )
        }
      />
      <div className="space-y-4 px-8 pb-8">
        <div className="relative max-w-md">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search name, author, ISBN, SKU, barcode…" className="h-10 pl-9" />
        </div>

        {products.data && products.data.length === 0 ? (
          <EmptyState
            icon={<Boxes />}
            title={q ? "No matching products" : "No products yet"}
            body={q ? undefined : "Add your first product to start selling."}
            action={canEdit && !q && <Button onClick={() => setEditing("new")}>Add a product</Button>}
          />
        ) : (
          <div className="overflow-hidden rounded-xl border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40">
                  <TableHead className="pl-4">Product</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead>SKU</TableHead>
                  <TableHead className="text-right">Price</TableHead>
                  <TableHead className="text-right">In stock</TableHead>
                  <TableHead className="w-12 pr-4" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {(products.data ?? []).flatMap((p) =>
                  p.variants.map((v, i) => (
                    <TableRow key={v.id} className={cn(!p.active && "opacity-50", canEdit && "cursor-pointer")} onClick={() => canEdit && setEditing(p)}>
                      <TableCell className="pl-4">
                        <div className="flex items-center gap-3">
                        {i === 0 ? <ProductImage hash={p.image_hash} name={p.name} className="aspect-[2/3] w-8 shrink-0" /> : <span className="w-8 shrink-0" />}
                        <div>
                        <div className="font-medium">
                          {i === 0 || p.variants.length === 1 ? p.name : <span className="text-muted-foreground">↳</span>}
                          {v.name && <span className="text-muted-foreground"> · {v.name}</span>}
                          {!p.active && <Badge variant="outline" className="ml-2">Inactive</Badge>}
                        </div>
                        {i === 0 && (p.attributes.author || p.attributes.brand) && (
                          <div className="text-xs text-muted-foreground">{p.attributes.author || p.attributes.brand}</div>
                        )}
                        </div>
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">{i === 0 ? p.category_name : ""}</TableCell>
                      <TableCell className="text-muted-foreground tabular">{v.sku}</TableCell>
                      <TableCell className="text-right font-medium tabular">{fmt(v.price_minor)}</TableCell>
                      <TableCell className="text-right tabular">
                        {p.track_stock ? (
                          <span className={cn(v.stock_milli <= 0 ? "text-red-600" : v.reorder_level_milli > 0 && v.stock_milli <= v.reorder_level_milli && "text-amber-600")}>
                            {formatQty(v.stock_milli)} {p.unit !== "pc" && p.unit}
                          </span>
                        ) : (
                          <span className="text-muted-foreground">n/a</span>
                        )}
                      </TableCell>
                      <TableCell className="pr-4">
                        {canEdit && p.track_stock && (
                          <Button aria-label={`Adjust stock of ${p.name}`}
                            variant="ghost"
                            size="icon-sm"
                            title="Adjust stock"
                            onClick={(e) => {
                              e.stopPropagation()
                              setAdjust({ p, v })
                            }}
                          >
                            <SlidersHorizontal />
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  )),
                )}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
      {editing && <ProductDialog product={editing === "new" ? null : editing} onClose={() => setEditing(null)} />}
      {adjust && <StockDialog {...adjust} onClose={() => setAdjust(null)} />}
      {importing && <ImportDialog onClose={() => setImporting(false)} />}
    </Page>
  )
}

interface VariantForm {
  id: string
  name: string
  sku: string
  price: string
  cost: string
  barcodes: string
  opening: string
  reorder: string
  active: boolean
}

function ProductDialog({ product, onClose }: { product: ProductDetail | null; onClose: () => void }) {
  const state = useLoadedState()
  const qc = useQueryClient()
  const { currency } = useMoney()
  const categories = useQuery({ queryKey: ["categories"], queryFn: api.categories })
  const business = state.settings.business_type || "general"
  const fields = productFields(state.settings)
  const units = shopUnits(state.settings)
  const toMajor = (m: number) => (m / 10 ** currency.digits).toFixed(currency.digits)

  const [name, setName] = useState(product?.name ?? "")
  const [categoryId, setCategoryId] = useState(product?.category_id ?? "")
  const [attrs, setAttrs] = useState<Record<string, string>>(product?.attributes ?? {})
  const [unit, setUnit] = useState(product?.unit ?? "pc")
  const [allowFraction, setAllowFraction] = useState(product?.allow_fraction ?? false)
  const [trackStock, setTrackStock] = useState(product?.track_stock ?? true)
  const [active, setActive] = useState(product?.active ?? true)
  const [taxPct, setTaxPct] = useState(product ? String((product.tax_rate_bp ?? 0) / 100) : defaultTaxPct(state.settings))
  const [newCategory, setNewCategory] = useState("")
  // Picture: undefined = unchanged, "" = remove, otherwise base64 of the new file.
  const [picture, setPicture] = useState<string | undefined>()
  const [preview, setPreview] = useState<string | null>(null)
  const [variants, setVariants] = useState<VariantForm[]>(
    product?.variants.map((v) => ({
      id: v.id,
      name: v.name,
      sku: v.sku,
      price: toMajor(v.price_minor),
      cost: v.cost_minor ? toMajor(v.cost_minor) : "",
      barcodes: v.barcodes.join(", "),
      opening: "",
      reorder: v.reorder_level_milli ? formatQty(v.reorder_level_milli) : "",
      active: v.active,
    })) ?? [{ id: "", name: "", sku: "", price: "", cost: "", barcodes: "", opening: "", reorder: "", active: true }],
  )
  const [busy, setBusy] = useState(false)

  // A bookshop's ISBN is the barcode: prefill it for new single-variant books.
  useEffect(() => {
    if (!product && attrs.isbn && variants.length === 1 && !variants[0].barcodes) {
      setVariants((vs) => [{ ...vs[0], barcodes: attrs.isbn }])
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [attrs.isbn])

  const setV = (i: number, patch: Partial<VariantForm>) => setVariants((vs) => vs.map((v, j) => (j === i ? { ...v, ...patch } : v)))
  const priceErr =
    variants.some((v) => isNaN(parseMoney(v.price, currency)) || v.price.trim() === "") || !(Number(taxPct) >= 0 && Number(taxPct) <= 100)

  async function save() {
    setBusy(true)
    try {
      let cat = categoryId
      if (cat === "__new" && newCategory.trim()) {
        cat = await api.saveCategory({ name: newCategory.trim(), active: true })
      } else if (cat === "__new") cat = ""
      const input: ProductInput = {
        product: {
          id: product?.id ?? "",
          name: name.trim(),
          category_id: cat,
          description: product?.description ?? "",
          attributes: attrs,
          unit,
          allow_fraction: allowFraction,
          track_stock: trackStock,
          tax_rate_bp: Math.round((Number(taxPct) || 0) * 100),
          active,
        },
        variants: variants.map((v) => ({
          id: v.id,
          product_id: product?.id ?? "",
          name: v.name.trim(),
          sku: v.sku.trim(),
          price_minor: parseMoney(v.price, currency),
          cost_minor: parseMoney(v.cost, currency) || 0,
          active: v.active,
          barcodes: v.barcodes.split(/[,\s]+/).filter(Boolean),
          opening_qty_milli: v.id ? 0 : parseQty(v.opening) || 0,
          reorder_level_milli: parseQty(v.reorder) || 0,
          opening_location_id: "",
        })),
      }
      const id = await api.saveProduct(input)
      if (picture !== undefined) {
        try {
          await api.setProductImage(id || product!.id, picture)
        } catch (e) {
          toast.error("Saved, but the picture wasn't: " + errorMessage(e))
        }
      }
      toast.success(product ? "Product updated" : "Product added")
      qc.invalidateQueries()
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[92vh] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>{product ? "Edit product" : "New product"}</DialogTitle>
          <DialogDescription>Saved at the hub, then sent to every till automatically.</DialogDescription>
        </DialogHeader>

        <div className="flex items-center gap-4">
          {preview ? (
            <img src={preview} alt="" className="aspect-[2/3] w-16 rounded-md object-cover" />
          ) : (
            <ProductImage hash={picture === "" ? undefined : product?.image_hash} name={name || "New product"} className="aspect-[2/3] w-16" />
          )}
          <div className="space-y-1.5">
            <div className="flex gap-2">
              <Button type="button" variant="outline" size="sm" asChild>
                <label className="cursor-pointer">
                  Choose picture
                  <input
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    className="sr-only"
                    onChange={(e) => {
                      const f = e.target.files?.[0]
                      if (!f) return
                      const r = new FileReader()
                      r.onload = () => {
                        const url = String(r.result)
                        setPreview(url)
                        setPicture(url.slice(url.indexOf(",") + 1))
                      }
                      r.readAsDataURL(f)
                    }}
                  />
                </label>
              </Button>
              {(preview || (product?.image_hash && picture !== "")) && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setPreview(null)
                    setPicture(product?.image_hash ? "" : undefined)
                  }}
                >
                  Remove
                </Button>
              )}
            </div>
            <p className="text-xs text-muted-foreground">JPEG, PNG or WebP. Shown on the till; a cover is drawn when there's none.</p>
          </div>
        </div>

        <div className="grid grid-cols-6 gap-4">
          <div className="col-span-4 space-y-1.5">
            <Label>Name</Label>
            <Input autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder={business === "bookshop" ? "Book title" : "Product name"} />
          </div>
          <div className="col-span-2 space-y-1.5">
            <Label>Category</Label>
            <Select value={categoryId} onValueChange={setCategoryId}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder="None" />
              </SelectTrigger>
              <SelectContent>
                {(categories.data ?? []).map((c) => (
                  <SelectItem key={c.id} value={c.id}>
                    {c.name}
                  </SelectItem>
                ))}
                <SelectItem value="__new">+ New category…</SelectItem>
              </SelectContent>
            </Select>
            {categoryId === "__new" && <Input value={newCategory} onChange={(e) => setNewCategory(e.target.value)} placeholder="Category name" />}
          </div>
          {fields.map((f) => (
            <div key={f.key} className="col-span-3 space-y-1.5">
              <Label>{f.label}</Label>
              <Input value={attrs[f.key] ?? ""} placeholder={f.placeholder} onChange={(e) => setAttrs((a) => ({ ...a, [f.key]: e.target.value }))} />
            </div>
          ))}
        </div>

        <div className="grid grid-cols-5 gap-3">
          <div className="space-y-1.5">
            <Label>{state.settings.tax_name || "Tax"} rate %</Label>
            <Input inputMode="decimal" value={taxPct} onChange={(e) => setTaxPct(e.target.value)} className="text-right tabular" />
          </div>
          <div className="space-y-1.5">
            <Label>Unit</Label>
            <Select value={unit} onValueChange={setUnit}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(units.includes(unit) ? units : [...units, unit]).map((u) => (
                  <SelectItem key={u} value={u}>
                    {u}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Toggle label="Sold by weight / decimals" checked={allowFraction} onChange={setAllowFraction} />
          <Toggle label="Track stock" checked={trackStock} onChange={setTrackStock} />
          <Toggle label="Active (for sale)" checked={active} onChange={setActive} />
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label>Prices &amp; variants</Label>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setVariants((vs) => [...vs, { id: "", name: "", sku: "", price: "", cost: "", barcodes: "", opening: "", reorder: "", active: true }])}
            >
              <Plus /> Add variant
            </Button>
          </div>
          <div className="space-y-2">
            {variants.map((v, i) => (
              <div key={i} className="grid grid-cols-[1.2fr_1fr_1fr_1fr_1.5fr_0.9fr_0.8fr_auto] items-end gap-2 rounded-lg border bg-muted/30 p-2.5">
                <MiniField label="Variant">
                  <Input value={v.name} onChange={(e) => setV(i, { name: e.target.value })} placeholder={variants.length > 1 ? "e.g. Hardcover" : "(standard)"} />
                </MiniField>
                <MiniField label="SKU">
                  <Input value={v.sku} onChange={(e) => setV(i, { sku: e.target.value })} placeholder="auto" />
                </MiniField>
                <MiniField label={`Price ${currency.symbol}`}>
                  <Input inputMode="decimal" value={v.price} onChange={(e) => setV(i, { price: e.target.value })} className="text-right tabular" />
                </MiniField>
                <MiniField label={`Cost ${currency.symbol}`}>
                  <Input inputMode="decimal" value={v.cost} onChange={(e) => setV(i, { cost: e.target.value })} className="text-right tabular" />
                </MiniField>
                <MiniField label="Barcodes">
                  <Input value={v.barcodes} onChange={(e) => setV(i, { barcodes: e.target.value })} placeholder="scan or type" />
                </MiniField>
                <MiniField label={v.id ? "Stock" : "Opening stock"}>
                  {v.id ? (
                    <div className="flex h-8 items-center px-1 text-sm text-muted-foreground tabular">
                      {formatQty(product?.variants.find((x) => x.id === v.id)?.stock_milli ?? 0)}
                    </div>
                  ) : (
                    <Input inputMode="decimal" value={v.opening} onChange={(e) => setV(i, { opening: e.target.value })} disabled={!trackStock} className="text-right tabular" />
                  )}
                </MiniField>
                <MiniField label="Reorder at">
                  <Input inputMode="decimal" value={v.reorder} onChange={(e) => setV(i, { reorder: e.target.value })} disabled={!trackStock} placeholder="–" className="text-right tabular" title="Flag as low on stock at or below this quantity" />
                </MiniField>
                <Button aria-label="Remove variant"
                  variant="ghost"
                  size="icon"
                  disabled={variants.length === 1 || !!v.id}
                  title={v.id ? "Existing variants can be deactivated, not removed" : "Remove"}
                  onClick={() => setVariants((vs) => vs.filter((_, j) => j !== i))}
                >
                  <Trash2 />
                </Button>
              </div>
            ))}
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={save} disabled={!name.trim() || priceErr || busy}>
            {busy && <Loader2 className="animate-spin" />}
            {product ? "Save changes" : "Add product"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function MiniField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <div className="text-[11px] font-medium text-muted-foreground">{label}</div>
      {children}
    </div>
  )
}

function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex items-center justify-between gap-2 self-end rounded-lg border px-3 py-1.5 text-sm">
      {label}
      <Switch checked={checked} onCheckedChange={onChange} />
    </label>
  )
}

function StockDialog({ p, v, onClose }: { p: ProductDetail; v: VariantDetail; onClose: () => void }) {
  const qc = useQueryClient()
  const [mode, setMode] = useState<"count" | "delta">("count")
  const [value, setValue] = useState("")
  const [note, setNote] = useState("")
  const [busy, setBusy] = useState(false)
  const n = mode === "count" ? Number(value) : Number(value)
  const bad = value.trim() === "" || !isFinite(n) || (mode === "count" && n < 0)

  async function save() {
    setBusy(true)
    try {
      const milli = Math.round(n * 1000)
      await api.adjustStock({
        variant_id: v.id,
        location_id: "",
        ...(mode === "count" ? { count_milli: milli } : { delta_milli: milli }),
        note,
      })
      toast.success("Stock updated")
      qc.invalidateQueries()
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Adjust stock</DialogTitle>
          <DialogDescription>
            {p.name}
            {v.name && ` · ${v.name}`}. Currently {formatQty(v.stock_milli)} in stock here.
          </DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-2 gap-1 rounded-lg bg-muted p-1">
          {(
            [
              ["count", "I counted"],
              ["delta", "Add / remove"],
            ] as const
          ).map(([m, label]) => (
            <button key={m} onClick={() => setMode(m)} className={cn("rounded-md py-1.5 text-sm font-medium", mode === m ? "bg-card shadow-sm" : "text-muted-foreground")}>
              {label}
            </button>
          ))}
        </div>
        <div className="space-y-1.5">
          <Label>{mode === "count" ? "Quantity on the shelf" : "Change (use − to remove, e.g. -3)"}</Label>
          <Input autoFocus inputMode="decimal" value={value} onChange={(e) => setValue(e.target.value)} className="h-12 text-center text-xl font-semibold tabular" />
        </div>
        <div className="space-y-1.5">
          <Label>Reason</Label>
          <Input value={note} onChange={(e) => setNote(e.target.value)} placeholder={mode === "count" ? "Stock take" : "e.g. Damaged, received from supplier"} />
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={save} disabled={bad || busy}>
            {busy && <Loader2 className="animate-spin" />} Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

const sampleCSV = `name,variant,sku,barcode,price,cost,quantity,category,unit,tax_rate,reorder_level,author,isbn
Things Fall Apart,,,9780385474542,4500,3150,40,Fiction,pc,0,5,Chinua Achebe,9780385474542
Exercise Book,40 leaves,EX40,6151100000401,300,180,200,Stationery,pc,0,50,,
Exercise Book,80 leaves,EX80,6151100000802,500,300,150,Stationery,pc,0,50,,
`

function ImportDialog({ onClose }: { onClose: () => void }) {
  const state = useLoadedState()
  const qc = useQueryClient()
  const [csv, setCsv] = useState("")
  const [fileName, setFileName] = useState("")
  const [location, setLocation] = useState(state.pairing.location_id)
  const [preview, setPreview] = useState<ImportResult | null>(null)
  const [busy, setBusy] = useState(false)

  async function run(dry: boolean, text = csv) {
    setBusy(true)
    try {
      const r = await hub.importProducts(text, dry, location)
      setPreview(r)
      if (!dry) {
        toast.success(`Imported: ${r.created} new, ${r.updated} updated`)
        qc.invalidateQueries()
        if (r.errors.length === 0) onClose()
      }
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  async function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0]
    if (!f) return
    const text = await f.text()
    setFileName(f.name)
    setCsv(text)
    setPreview(null)
    run(true, text)
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Import products from a spreadsheet</DialogTitle>
          <DialogDescription>
            Save your spreadsheet as CSV. Existing items are matched by SKU or barcode and updated; new ones are created with their opening stock. Nothing changes until you confirm.
          </DialogDescription>
        </DialogHeader>
        <div className="rounded-lg border bg-muted/30 p-3 text-xs text-muted-foreground">
          <div className="mb-1 font-medium text-foreground">Columns (first row), in any order</div>
          <b>name</b> and <b>price</b> are needed for new items. Optional: variant, sku, barcode (several separated by |), cost, quantity, category (created if new), unit,
          tax_rate (%), reorder_level, description. Any other column, such as author, isbn, brand or strength, is kept as a product detail. Quantity only applies to new items; use
          a stock count to correct existing stock.
          <button
            type="button"
            className="ml-1 font-medium text-primary underline-offset-2 hover:underline"
            onClick={async () => {
              const path = await api.saveTextFile("products-template.csv", sampleCSV)
              if (path) toast.success("Saved " + path)
            }}
          >
            Download an example
          </button>
        </div>
        <div className="grid grid-cols-2 items-end gap-3">
          <label className="flex h-10 cursor-pointer items-center gap-2 rounded-md border border-dashed px-3 text-sm hover:bg-muted/50">
            <FileUp className="size-4" />
            <span className="truncate">{fileName || "Choose a CSV file…"}</span>
            <input type="file" accept=".csv,text/csv" className="hidden" onChange={onFile} />
          </label>
          <LocationSelect value={location} onChange={setLocation} />
        </div>
        {busy && !preview && <Loader2 className="mx-auto size-5 animate-spin text-muted-foreground" />}
        {preview && (
          <div className="space-y-3">
            <div className="grid grid-cols-3 gap-2 text-center">
              <div className="rounded-lg border p-3">
                <div className="text-2xl font-semibold tabular">{preview.created}</div>
                <div className="text-xs text-muted-foreground">{preview.dry_run ? "will be created" : "created"}</div>
              </div>
              <div className="rounded-lg border p-3">
                <div className="text-2xl font-semibold tabular">{preview.updated}</div>
                <div className="text-xs text-muted-foreground">{preview.dry_run ? "will be updated" : "updated"}</div>
              </div>
              <div className={cn("rounded-lg border p-3", preview.errors.length > 0 && "border-destructive/40")}>
                <div className="text-2xl font-semibold tabular">{preview.errors.length}</div>
                <div className="text-xs text-muted-foreground">rows with problems</div>
              </div>
            </div>
            {preview.new_categories.length > 0 && <p className="text-sm text-muted-foreground">New categories: {preview.new_categories.join(", ")}</p>}
            {preview.errors.length > 0 && (
              <div className="max-h-48 overflow-y-auto rounded-lg border border-destructive/30 bg-destructive/5 p-2 text-sm">
                {preview.errors.map((e, i) => (
                  <div key={i} className="py-0.5">
                    <span className="font-medium tabular">Row {e.row}:</span> {e.message}
                  </div>
                ))}
              </div>
            )}
            {preview.dry_run && preview.errors.length > 0 && <p className="text-sm text-muted-foreground">Rows with problems are skipped; fix them in the file and import again any time.</p>}
          </div>
        )}
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            {preview && !preview.dry_run ? "Close" : "Cancel"}
          </Button>
          {preview?.dry_run && (
            <Button onClick={() => run(false)} disabled={busy || preview.created + preview.updated === 0}>
              {busy && <Loader2 className="animate-spin" />} Import {preview.created + preview.updated} item{preview.created + preview.updated === 1 ? "" : "s"}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Tax rate new products start with: the shop's default, else 7.5% when tax is on. */
function defaultTaxPct(settings: Record<string, string>): string {
  if (settings.default_tax_bp) return String(Number(settings.default_tax_bp) / 100)
  return settings.tax_mode && settings.tax_mode !== "none" ? "7.5" : "0"
}
