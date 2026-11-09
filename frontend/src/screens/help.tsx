// The staff guide: short how-to topics, shown to the roles they concern.
import { useMemo, useState, type ReactNode } from "react"
import { BookOpen, Search } from "lucide-react"
import { Page, PageHeader } from "@/components/page"
import { Input } from "@/components/ui/input"
import { can } from "@/lib/format"
import { useLoadedState } from "@/lib/state"
import type { Role } from "@/lib/types"
import { cn } from "@/lib/utils"

interface Topic {
  id: string
  group: "Selling" | "Customers & returns" | "Shifts & money" | "Stock & buying" | "Running the store" | "When something goes wrong"
  title: string
  roles?: Role[] // who sees it (all staff when omitted)
  words?: string // extra search words
  body: ReactNode
}

const K = ({ children }: { children: ReactNode }) => <kbd className="rounded border bg-muted px-1.5 py-0.5 font-mono text-[0.85em]">{children}</kbd>
const Steps = ({ children }: { children: ReactNode }) => <ol className="list-decimal space-y-1.5 pl-5">{children}</ol>
const Tip = ({ children }: { children: ReactNode }) => <p className="rounded-lg border-l-4 border-primary/50 bg-accent/40 px-3 py-2">{children}</p>
const where = (path: string) => <b className="whitespace-nowrap">{path}</b>

const managers: Role[] = ["admin", "manager"]
const stock: Role[] = ["admin", "manager", "inventory"]
const admins: Role[] = ["admin"]

const topics: Topic[] = [
  // ---------- selling ----------
  {
    id: "sign-in",
    group: "Selling",
    title: "Signing in and out",
    words: "login password pin logout",
    body: (
      <>
        <p>Sign in with your username and your password or PIN. It works even when the till can't reach the hub.</p>
        <p>Sign out with the arrow at the bottom of the left bar when you leave the till, so nobody sells under your name.</p>
      </>
    ),
  },
  {
    id: "sell",
    group: "Selling",
    title: "Making a sale",
    words: "scan barcode search cart pay change receipt",
    body: (
      <>
        <Steps>
          <li>Scan the barcode, or type a name, author, ISBN or code and pick the item. Scanning the same item again adds one more.</li>
          <li>Change a quantity with − and +, or tap the number to type it (for weighed items, e.g. 1.25 kg).</li>
          <li>
            Press <K>F12</K> (or the Charge button) to take payment.
          </li>
          <li>Choose how the customer pays. You can split across several methods; only cash can be more than the total, and the change is shown.</li>
          <li>The receipt prints by itself if a printer is set up. Press Enter for the next sale.</li>
        </Steps>
        <Tip>
          Shortcuts: <K>F1</K> Sell, <K>F2</K> Returns, <K>F3</K> Sales, <K>F8</K> hold the sale, <K>F9</K> add a customer, <K>F12</K> pay, <K>Delete</K>{" "}
          removes the selected line.
        </Tip>
      </>
    ),
  },
  {
    id: "discounts",
    group: "Selling",
    title: "Discounts and price changes",
    words: "approval manager override price",
    body: (
      <>
        <p>Tap a line in the sale to change its price, or use the discount button for the whole sale.</p>
        <p>
          Cashiers need a manager for discounts and price changes: a box asks for the manager's username and password or PIN. The manager's name is kept with the
          sale. An admin can switch this off in {where("Settings → Store → Manager approval")}.
        </p>
      </>
    ),
  },
  {
    id: "hold",
    group: "Selling",
    title: "Holding a sale",
    words: "park recall later",
    body: (
      <p>
        If a customer steps away, press <K>F8</K> (Hold this sale) and serve the next person. Recall it later with the <b>Held sales</b> button at the top of the Sell screen.
      </p>
    ),
  },
  {
    id: "reprint",
    group: "Selling",
    title: "Reprinting a receipt",
    words: "copy duplicate receipt",
    body: (
      <p>
        Open <b>Sales</b> (<K>F3</K>), choose the sale and press <b>Reprint receipt</b>. Reprints are marked "REPRINT".
      </p>
    ),
  },
  {
    id: "offline",
    group: "Selling",
    title: "When the hub can't be reached",
    words: "offline network internet sync pending",
    body: (
      <>
        <p>
          The pill at the top right shows the connection. <b>Synced</b> means everything has reached the hub; <b>Offline</b> or a number waiting means sales are
          saved on this till and will be sent when the connection returns. Nothing is lost.
        </p>
        <p>Offline you can still sell, take returns for this till's receipts, take account payments, run your shift and receive stock that's on its way.</p>
        <p>Purchasing, reports and other back-office screens need the hub; they come back by themselves.</p>
      </>
    ),
  },

  // ---------- customers & returns ----------
  {
    id: "customers",
    group: "Customers & returns",
    title: "Customers and selling on account",
    words: "credit debtor balance owe statement",
    body: (
      <>
        <p>
          Press <K>F9</K> during a sale to pick or add a customer. To let them pay later, choose the account payment method when taking payment (an admin adds one
          in {where("Settings → Payment methods")}).
        </p>
        <p>
          When they pay their account, open <b>Customers</b>, choose them and press <b>Take payment</b>. Their statement shows every sale, payment and refund.
        </p>
      </>
    ),
  },
  {
    id: "returns",
    group: "Customers & returns",
    title: "Returns with a receipt",
    words: "refund exchange receipt",
    body: (
      <Steps>
        <li>
          Open <b>Returns</b> (<K>F2</K>) and scan the QR code on the receipt, or type its number.
        </li>
        <li>Choose how many of each item come back, and whether they go back on the shelf.</li>
        <li>Pick a reason and how to refund. The amount is what the customer actually paid, after any discount.</li>
        <li>A manager approves (unless switched off) and the refund slip prints.</li>
      </Steps>
    ),
  },
  {
    id: "no-receipt",
    group: "Customers & returns",
    title: "Returns without a receipt, and store credit",
    words: "store credit voucher no receipt",
    body: (
      <>
        <Steps>
          <li>
            In <b>Returns</b>, press <b>No receipt? Return without one</b>.
          </li>
          <li>Scan or search the items. The refund starts at today's price; lower it if the item was bought on offer.</li>
          <li>
            Choose <b>Store credit</b> (needs the customer) or a payment method, give a reason, and a manager approves. A manager always approves these.
          </li>
        </Steps>
        <Tip>Store credit stays on the customer's account. Next time, add the customer to the sale and choose "Store credit" when taking payment.</Tip>
      </>
    ),
  },

  // ---------- shifts ----------
  {
    id: "shift",
    group: "Shifts & money",
    title: "Opening and closing a shift",
    words: "float drawer count close day end z report x report",
    body: (
      <>
        <Steps>
          <li>At the start, count the cash in the drawer and open a shift with that float.</li>
          <li>
            During the day, <b>Shift</b> shows takings so far. The <b>X report</b> prints them without closing.
          </li>
          <li>
            At the end, count the cash and press <b>Close shift</b>. The <b>Z report</b> prints with any difference between counted and expected cash.
          </li>
        </Steps>
      </>
    ),
  },
  {
    id: "cash",
    group: "Shifts & money",
    title: "Paid in, paid out and opening the drawer",
    words: "petty cash expense no sale drawer",
    body: (
      <p>
        In <b>Shift</b>, use <b>Paid in / out</b> for cash added to or taken from the drawer (for example fuel or change), with a reason. <b>No sale: open drawer</b>{" "}
        opens it without a sale. Both are recorded and may need a manager.
      </p>
    ),
  },

  // ---------- stock & buying ----------
  {
    id: "products",
    group: "Stock & buying",
    title: "Adding and editing products",
    roles: stock,
    words: "item price barcode variant import spreadsheet csv",
    body: (
      <>
        <p>
          In <b>Products</b>, press <b>Add a product</b>. Give a name, price and barcode. Variants (sizes, colours) each have their own price, code and stock.
        </p>
        <p>
          To load many at once, use <b>Import products from a spreadsheet</b>. It shows what will change before anything is saved.
        </p>
      </>
    ),
  },
  {
    id: "counts",
    group: "Stock & buying",
    title: "Counting stock",
    roles: stock,
    words: "stocktake count adjust shrinkage",
    body: (
      <p>
        In {where("Stock → Stock counts")}, start a count and enter what is on the shelf. You can keep selling while you count; sales made after an item was
        counted are allowed for. When you post the count, the differences are recorded.
      </p>
    ),
  },
  {
    id: "transfers",
    group: "Stock & buying",
    title: "Moving stock between locations",
    roles: stock,
    words: "transfer branch warehouse send receive in transit",
    body: (
      <>
        <p>
          <b>Sending:</b> {where("Stock → Send stock from here")} (or {where("Stock → Transfers")} in the back office). The goods leave this location and show as in
          transit.
        </p>
        <p>
          <b>Receiving:</b> what's coming appears under <b>On its way here</b> at the top of Stock. Count what arrived and book it in; anything missing is
          recorded as short.
        </p>
        <Tip>Both work without the hub. They're sent when the connection returns.</Tip>
      </>
    ),
  },
  {
    id: "purchasing",
    group: "Stock & buying",
    title: "Ordering from suppliers and receiving deliveries",
    roles: stock,
    words: "purchase order po supplier delivery grn goods received",
    body: (
      <Steps>
        <li>
          In <b>Purchasing</b>, create a purchase order for a supplier and mark it ordered. You can print or email it.
        </li>
        <li>
          When the delivery comes, receive it against the order (or under <b>On its way here</b> in Stock at a branch, even offline). Enter what actually
          arrived.
        </li>
        <li>Stock goes up and the item's cost is updated to the average of old and new stock.</li>
      </Steps>
    ),
  },
  {
    id: "payables",
    group: "Stock & buying",
    title: "Supplier bills and payments",
    roles: managers,
    words: "bill invoice pay supplier owe statement payables",
    body: (
      <p>
        Record the supplier's bill from the delivery ({where("Purchasing → Deliveries")}). Pay it in {where("Purchasing → Payments")}; payments go to the bills you
        tick, then the oldest. {where("Reports → Owed to suppliers")} shows what is due and overdue.
      </p>
    ),
  },
  {
    id: "documents",
    group: "Stock & buying",
    title: "Printing and emailing documents",
    roles: managers,
    words: "pdf print email purchase order statement",
    body: (
      <p>
        Purchase orders, transfer notes, supplier statements and payment advice have <b>Print</b> and <b>Email</b> buttons. Print opens the page ready to print or
        save as PDF; Email sends it with a PDF attached (an admin sets up email first).
      </p>
    ),
  },

  // ---------- running the store ----------
  {
    id: "reports",
    group: "Running the store",
    title: "Reports",
    roles: managers,
    words: "sales profit takings export csv",
    body: (
      <p>
        <b>Reports</b> shows sales and profit by day, hour, item, category, cashier, till, customer or location, money in by payment method, shifts and the
        activity log. Pick the period at the top; every table exports to a spreadsheet (CSV).
      </p>
    ),
  },
  {
    id: "team",
    group: "Running the store",
    title: "Staff and roles",
    roles: admins,
    words: "user cashier manager inventory admin pin",
    body: (
      <>
        <p>
          In <b>Team</b>, add each person with their own username. Give cashiers a short PIN for quick sign-in.
        </p>
        <ul className="list-disc space-y-1 pl-5">
          <li>
            <b>Cashier</b>: sells, returns (with approval), shifts.
          </li>
          <li>
            <b>Inventory</b>: products, stock, purchasing.
          </li>
          <li>
            <b>Manager</b>: everything above, approvals, reports, supplier payments.
          </li>
          <li>
            <b>Admin</b>: everything, including settings, team and tills.
          </li>
        </ul>
      </>
    ),
  },
  {
    id: "pair",
    group: "Running the store",
    title: "Adding a till",
    roles: managers,
    words: "pair new computer join code security",
    body: (
      <Steps>
        <li>
          On a till that's already set up, open <b>Tills</b> and press <b>Pair a new till</b>. A 6-digit code appears with the hub's security code.
        </li>
        <li>Install the app on the new computer, choose "Join an existing store", pick the hub and enter the code.</li>
        <li>Check the security code on the new till matches. If it doesn't, don't connect, and ask whoever looks after your system.</li>
      </Steps>
    ),
  },
  {
    id: "settings",
    group: "Running the store",
    title: "Store settings",
    roles: admins,
    words: "tax vat receipt header footer shop type payment methods approval",
    body: (
      <ul className="list-disc space-y-1 pl-5">
        <li>{where("Settings → Store")}: name, address, tax, receipt text and which actions need a manager.</li>
        <li>{where("Settings → Shop type")}: product details and units for your kind of shop.</li>
        <li>{where("Settings → Payment methods")}: cash, POS, transfer and any others you take.</li>
        <li>{where("Settings → Printer")}: the receipt printer for this till, with a test page.</li>
      </ul>
    ),
  },
  {
    id: "backups",
    group: "Running the store",
    title: "Backups",
    roles: managers,
    words: "backup usb restore disk copy",
    body: (
      <>
        <p>
          The hub backs up every day. In {where("Settings → Backups")}, choose a USB stick, second drive or network folder so a copy is kept off the hub PC. A yellow
          bar warns if backups stop.
        </p>
        <p>
          To restore (admins, on the hub PC): <b>Restore</b> next to a backup. The current records are saved first, and tills re-send any sales the backup is
          missing.
        </p>
      </>
    ),
  },
  {
    id: "updates",
    group: "Running the store",
    title: "Updating the app",
    roles: managers,
    words: "update version upgrade install",
    body: (
      <p>
        When a new version is ready at the hub, a bar appears on each till. Press <b>Download from hub</b>, then <b>Install and restart</b>; Windows may ask for
        permission. Unsent sales are kept. Details are in {where("Settings → Updates")}.
      </p>
    ),
  },
  {
    id: "email",
    group: "Running the store",
    title: "Setting up email",
    roles: admins,
    words: "smtp gmail mail",
    body: (
      <p>
        In {where("Settings → Email")}, enter your mail server. For Gmail: smtp.gmail.com, port 587, STARTTLS, and an app password from your Google account. Send
        a test to check.
      </p>
    ),
  },

  // ---------- trouble ----------
  {
    id: "printer",
    group: "When something goes wrong",
    title: "The receipt doesn't print",
    words: "printer paper thermal",
    body: (
      <Steps>
        <li>Check the printer is on, has paper and the lid is closed.</li>
        <li>
          In {where("Settings → Printer")} (managers), press <b>Print test page</b>. The ruler should fill exactly one line; if not, change the paper width.
        </li>
        <li>The sale is saved even when printing fails; reprint it from Sales.</li>
      </Steps>
    ),
  },
  {
    id: "support",
    group: "When something goes wrong",
    title: "Getting help",
    words: "support logs problem error",
    body: (
      <p>
        In {where("Settings → Connection")}, press <b>Save support file</b> and send it to whoever looks after your system. It contains logs and settings, not
        sales or customer details.
      </p>
    ),
  },
  {
    id: "cert",
    group: "When something goes wrong",
    title: '"The hub\'s security certificate has changed"',
    words: "certificate security encrypted",
    body: (
      <p>
        The till refuses to talk to a computer that isn't its hub. If the hub was reinstalled without restoring a backup, pair this till again (
        {where("Settings → Connection")}). If nothing changed, tell whoever looks after your system.
      </p>
    ),
  },
]

export function Help() {
  const state = useLoadedState()
  const role = state.user?.role
  const [q, setQ] = useState("")
  const mine = useMemo(() => topics.filter((t) => !t.roles || can(role, ...t.roles)), [role])
  const shown = useMemo(() => {
    const words = q.toLowerCase().split(/\s+/).filter(Boolean)
    if (!words.length) return mine
    return mine.filter((t) => {
      const text = (t.title + " " + t.group + " " + (t.words ?? "")).toLowerCase()
      return words.every((w) => text.includes(w))
    })
  }, [mine, q])
  const [openId, setOpenId] = useState(mine[0]?.id)
  const open = shown.find((t) => t.id === openId) ?? shown[0]
  const groups = [...new Set(shown.map((t) => t.group))]

  return (
    <Page>
      <PageHeader title="Help" description="How to do things in Zebiventor. Only the topics for your role are shown." />
      <div className="grid min-h-0 grid-cols-[18rem_1fr] gap-6 px-8 pb-8">
        <nav aria-label="Help topics" className="space-y-4">
          <div className="relative">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search help" aria-label="Search help" className="pl-9" />
          </div>
          {groups.map((g) => (
            <div key={g}>
              <div className="mb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">{g}</div>
              <ul className="space-y-0.5">
                {shown
                  .filter((t) => t.group === g)
                  .map((t) => (
                    <li key={t.id}>
                      <button
                        onClick={() => setOpenId(t.id)}
                        aria-current={open?.id === t.id ? "true" : undefined}
                        className={cn("w-full rounded-md px-2.5 py-1.5 text-left text-sm", open?.id === t.id ? "bg-accent font-medium text-accent-foreground" : "hover:bg-muted")}
                      >
                        {t.title}
                      </button>
                    </li>
                  ))}
              </ul>
            </div>
          ))}
          {!shown.length && <p className="text-sm text-muted-foreground">Nothing matches "{q}".</p>}
        </nav>
        {open && (
          <article className="max-w-2xl space-y-3 rounded-xl border bg-card p-6 text-sm leading-relaxed">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <BookOpen className="size-4" /> {open.group}
            </div>
            <h2 className="text-lg font-semibold">{open.title}</h2>
            {open.body}
          </article>
        )}
      </div>
    </Page>
  )
}
