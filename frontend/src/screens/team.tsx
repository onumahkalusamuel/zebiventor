import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Loader2, UserPlus } from "lucide-react"
import { toast } from "sonner"
import { Page, PageHeader } from "@/components/page"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api, errorMessage } from "@/lib/api"
import { roleLabel } from "@/lib/format"
import type { Role, StaffUser, UserInput } from "@/lib/types"

const roleHelp: Record<Role, string> = {
  admin: "Everything, including staff, tills and store settings",
  manager: "Sell, discounts, products, stock, pair tills, printer",
  inventory: "Products and stock only",
  cashier: "Sell and view own till’s sales",
}

export function Team() {
  const users = useQuery({ queryKey: ["users"], queryFn: api.listUsers })
  const [editing, setEditing] = useState<StaffUser | "new" | null>(null)
  return (
    <Page>
      <PageHeader
        title="Team"
        description="Staff can sign in on any till, even when the hub is offline."
        actions={
          <Button className="h-10" onClick={() => setEditing("new")}>
            <UserPlus /> Add staff
          </Button>
        }
      />
      <div className="px-8 pb-8">
        <div className="overflow-hidden rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="pl-4">Name</TableHead>
                <TableHead>Username</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Sign-in</TableHead>
                <TableHead className="pr-4">Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(users.data ?? []).map((u) => (
                <TableRow key={u.id} className="cursor-pointer" onClick={() => setEditing(u)}>
                  <TableCell className="pl-4 font-medium">{u.name}</TableCell>
                  <TableCell className="text-muted-foreground">{u.username}</TableCell>
                  <TableCell>{roleLabel[u.role]}</TableCell>
                  <TableCell className="text-muted-foreground">{u.has_pin ? "Password or PIN" : "Password"}</TableCell>
                  <TableCell className="pr-4">{u.active ? <Badge variant="secondary">Active</Badge> : <Badge variant="outline">Disabled</Badge>}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </div>
      {editing && <UserDialog user={editing === "new" ? null : editing} onClose={() => setEditing(null)} />}
    </Page>
  )
}

function UserDialog({ user, onClose }: { user: StaffUser | null; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<UserInput>({
    id: user?.id ?? "",
    name: user?.name ?? "",
    username: user?.username ?? "",
    role: user?.role ?? "cashier",
    password: "",
    pin: "",
    active: user?.active ?? true,
  })
  const [busy, setBusy] = useState(false)
  const set = <K extends keyof UserInput>(k: K, v: UserInput[K]) => setF((p) => ({ ...p, [k]: v }))

  async function save() {
    setBusy(true)
    try {
      await api.saveUser(f)
      toast.success(user ? "Staff member updated" : "Staff member added")
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
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{user ? `Edit ${user.name}` : "Add staff member"}</DialogTitle>
          <DialogDescription>Cashiers can use a short numeric PIN for quick sign-in at the till.</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <Label>Full name</Label>
            <Input autoFocus value={f.name} onChange={(e) => set("name", e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>Username</Label>
            <Input value={f.username} onChange={(e) => set("username", e.target.value)} />
          </div>
          <div className="col-span-2 space-y-1.5">
            <Label>Role</Label>
            <Select value={f.role} onValueChange={(v) => set("role", v as Role)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(roleLabel) as Role[]).map((r) => (
                  <SelectItem key={r} value={r}>
                    {roleLabel[r]} <span className="text-muted-foreground">· {roleHelp[r]}</span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label>{user ? "New password" : "Password"}</Label>
            <Input type="password" value={f.password} onChange={(e) => set("password", e.target.value)} placeholder={user ? "unchanged" : "6+ characters"} />
          </div>
          <div className="space-y-1.5">
            <Label>{user ? "New PIN" : "PIN (optional)"}</Label>
            <Input inputMode="numeric" value={f.pin} onChange={(e) => set("pin", e.target.value.replace(/\D/g, ""))} placeholder={user ? "unchanged" : "4+ digits"} />
          </div>
          <label className="col-span-2 flex items-center justify-between rounded-lg border px-3 py-2 text-sm">
            Account active
            <Switch checked={f.active} onCheckedChange={(v) => set("active", v)} />
          </label>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={save} disabled={busy || !f.name.trim() || !f.username.trim()}>
            {busy && <Loader2 className="animate-spin" />} Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
