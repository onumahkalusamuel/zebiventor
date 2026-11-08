import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Search, UserPlus, UsersRound } from "lucide-react"
import { CustomerForm } from "@/components/customer-picker"
import { EmptyState, Page, PageHeader, StatCard } from "@/components/page"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api } from "@/lib/api"
import { timeAgo } from "@/lib/format"
import { useMoney } from "@/lib/state"
import type { Customer } from "@/lib/types"
import { cn } from "@/lib/utils"

export function Customers() {
  const { fmt } = useMoney()
  const [q, setQ] = useState("")
  const [editing, setEditing] = useState<Customer | "new" | null>(null)
  const list = useQuery({ queryKey: ["customers", q], queryFn: () => api.searchCustomers(q) })
  const owing = (list.data ?? []).filter((c) => c.balance_minor > 0)
  const totalOwed = owing.reduce((s, c) => s + c.balance_minor, 0)

  return (
    <Page>
      <PageHeader
        title="Customers"
        description="Shared by every till. Add customers even while offline."
        actions={
          <Button className="h-10" onClick={() => setEditing("new")}>
            <UserPlus /> New customer
          </Button>
        }
      />
      <div className="space-y-5 px-8 pb-8">
        <div className="grid grid-cols-3 gap-3">
          <StatCard label="Customers" value={list.data?.length ?? 0} sub={q ? "matching your search" : undefined} />
          <StatCard label="Owing on account" value={owing.length} />
          <StatCard label="Total owed" value={fmt(totalOwed)} sub="Collect payments on account in the next release" />
        </div>
        <div className="relative max-w-md">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Name, phone or email" className="h-10 pl-9" />
        </div>
        {list.data?.length === 0 ? (
          <EmptyState icon={<UsersRound />} title={q ? "No matching customers" : "No customers yet"} body="Add customers at the till or here." />
        ) : (
          <div className="overflow-hidden rounded-xl border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40">
                  <TableHead className="pl-4">Name</TableHead>
                  <TableHead>Phone</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Updated</TableHead>
                  <TableHead className="pr-4 text-right">Balance owed</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(list.data ?? []).map((c) => (
                  <TableRow key={c.id} className={cn("cursor-pointer", !c.active && "opacity-50")} onClick={() => setEditing(c)}>
                    <TableCell className="pl-4 font-medium">
                      {c.name}
                      {!c.active && <Badge variant="outline" className="ml-2">Inactive</Badge>}
                    </TableCell>
                    <TableCell className="text-muted-foreground tabular">{c.phone}</TableCell>
                    <TableCell className="text-muted-foreground">{c.email}</TableCell>
                    <TableCell className="text-muted-foreground">{timeAgo(c.updated_at)}</TableCell>
                    <TableCell className={cn("pr-4 text-right font-semibold tabular", c.balance_minor > 0 && "text-amber-700 dark:text-amber-400")}>
                      {c.balance_minor ? fmt(c.balance_minor) : "–"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
      <Dialog open={!!editing} onOpenChange={(o) => !o && setEditing(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{editing === "new" ? "New customer" : "Edit customer"}</DialogTitle>
            <DialogDescription>Changes reach every till automatically.</DialogDescription>
          </DialogHeader>
          {editing && <CustomerForm initial={editing === "new" ? {} : editing} onSaved={() => setEditing(null)} onCancel={() => setEditing(null)} />}
        </DialogContent>
      </Dialog>
    </Page>
  )
}
