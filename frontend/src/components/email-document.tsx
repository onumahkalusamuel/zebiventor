import { useState } from "react"
import { Loader2, Mail, Send } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api, errorMessage } from "@/lib/api"
import type { DocumentKind, EmailDraft } from "@/lib/types"

export function EmailButton({ kind, id, variant = "outline" }: { kind: DocumentKind; id: string; variant?: "outline" | "ghost" }) {
  const [draft, setDraft] = useState<EmailDraft | null>(null)
  const [loading, setLoading] = useState(false)
  async function open() {
    setLoading(true)
    try {
      setDraft(await api.documentEmailDraft(kind, id))
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setLoading(false)
    }
  }
  return (
    <>
      <Button variant={variant} onClick={open} disabled={loading}>
        {loading ? <Loader2 className="animate-spin" /> : <Mail />} Email
      </Button>
      {draft && <EmailDialog kind={kind} id={id} initial={draft} onClose={() => setDraft(null)} />}
    </>
  )
}

function EmailDialog({ kind, id, initial, onClose }: { kind: DocumentKind; id: string; initial: EmailDraft; onClose: () => void }) {
  const [d, setD] = useState(initial)
  const [busy, setBusy] = useState(false)
  const set = (k: keyof EmailDraft, v: string) => setD((p) => ({ ...p, [k]: v }))
  async function send() {
    setBusy(true)
    try {
      await api.emailDocument(kind, id, d)
      toast.success(`Sent to ${d.to}`)
      onClose()
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && !busy && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Email {d.subject.split(" from ")[0].toLowerCase()}</DialogTitle>
          <DialogDescription>The document goes in the email and as a PDF attachment.</DialogDescription>
        </DialogHeader>
        {!initial.ready ? (
          <p className="rounded-md bg-amber-500/10 px-3 py-2 text-sm text-amber-900 dark:text-amber-200">
            Email isn't set up yet. An admin can add the mail server in Settings → Email.
          </p>
        ) : (
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="em-to">To</Label>
              <Input id="em-to" value={d.to} onChange={(e) => set("to", e.target.value)} placeholder="name@example.com (separate several with commas)" autoFocus={!d.to} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="em-cc">Copy to</Label>
              <Input id="em-cc" value={d.cc} onChange={(e) => set("cc", e.target.value)} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="em-subject">Subject</Label>
              <Input id="em-subject" value={d.subject} onChange={(e) => set("subject", e.target.value)} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="em-msg">Message</Label>
              <textarea
                id="em-msg"
                className="min-h-32 w-full rounded-lg border bg-transparent px-2.5 py-2 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                value={d.message}
                onChange={(e) => set("message", e.target.value)}
              />
            </div>
          </div>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={busy}>
            {initial.ready ? "Cancel" : "Close"}
          </Button>
          {initial.ready && (
            <Button onClick={send} disabled={busy || !d.to.includes("@") || !d.subject.trim()}>
              {busy ? <Loader2 className="animate-spin" /> : <Send />} Send
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
