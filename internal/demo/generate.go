package demo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/appdata"
	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/terminal"
)

// Config sets up a generation run.
type Config struct {
	Out        string    // store data file to write
	CoverCache string    // downloaded covers (zebidemo covers)
	From, To   time.Time // trading days, inclusive
	Scale      float64   // multiplies daily sales
	Seed       int64
}

// Demo sign-ins (also in docs/DEMO-DATA.md).
const (
	DemoPassword = "tmit-demo-2023"
	DemoPIN      = "2468"
)

// ---------- the store ----------

type staffer struct {
	name, user, role string
	from, until      string // YYYY-MM-DD, "" = always
	till             int    // usual till (0..2), -1 = back office
}

var staff = []staffer{
	{"Tolu Adebayo", "admin", protocol.RoleAdmin, "", "", -1},
	{"Chiamaka Okafor", "chiamaka", protocol.RoleManager, "", "", -1},
	{"Ibrahim Musa", "ibrahim", protocol.RoleManager, "2024-06-03", "", -1},
	{"Femi Ogunleye", "femi", protocol.RoleInventory, "", "", -1},
	{"Blessing Eze", "blessing", protocol.RoleCashier, "", "", 0},
	{"Kunle Ajayi", "kunle", protocol.RoleCashier, "", "2024-08-30", 0},
	{"Sade Williams", "sade", protocol.RoleCashier, "2024-09-02", "", 0},
	{"Ngozi Nwosu", "ngozi", protocol.RoleCashier, "", "", 1},
	{"Aisha Bello", "aisha", protocol.RoleCashier, "", "2025-03-28", 2},
	{"Emeka Obi", "emeka", protocol.RoleCashier, "2025-04-01", "", 2},
	{"Halima Yusuf", "halima", protocol.RoleCashier, "2023-06-05", "", 0},
}

type tillDef struct {
	name     string
	kiosk    bool
	base     float64 // sales on an ordinary weekday
	open     [7][2]int
	weights  map[string]float64
	lines    []float64 // chance of 1, 2, 3, 4 lines
	transfer float64   // extra share of bank transfers (students)
}

var tills = []tillDef{
	{
		name: "Front Counter", base: 78,
		open:    [7][2]int{{12, 17}, {9, 19}, {9, 19}, {9, 19}, {9, 19}, {9, 19}, {9, 19}},
		weights: map[string]float64{"African Literature": .22, "Fiction": .17, "Non-fiction": .1, "Business & Self-help": .13, "Religion": .09, "Textbooks & Exam Prep": .05, "Cookbooks": .04, "Poetry & Drama": .03, "Young Adult": .05, "Children": .04, "Stationery": .08},
		lines:   []float64{.62, .25, .09, .04},
	},
	{
		name: "Children's Corner", base: 30,
		open:    [7][2]int{{0, 0}, {10, 18}, {10, 18}, {10, 18}, {10, 18}, {10, 18}, {10, 18}},
		weights: map[string]float64{"Children": .56, "Young Adult": .2, "Stationery": .2, "Textbooks & Exam Prep": .04},
		lines:   []float64{.45, .3, .15, .1},
	},
	{
		name: "Campus Kiosk", kiosk: true, base: 38, transfer: .15,
		open:    [7][2]int{{0, 0}, {8, 17}, {8, 17}, {8, 17}, {8, 17}, {8, 17}, {10, 14}},
		weights: map[string]float64{"Textbooks & Exam Prep": .38, "Stationery": .32, "Business & Self-help": .1, "Fiction": .08, "African Literature": .12},
		lines:   []float64{.55, .28, .12, .05},
	},
}

type supplierDef struct {
	name, contact string
	terms         int
	categories    []string
}

var suppliers = []supplierDef{
	{"Lagos Book Distributors Ltd", "Tunde Bello", 30, []string{"African Literature", "Poetry & Drama"}},
	{"Abuja Literary Imports", "Grace Adeyemi", 45, []string{"Fiction", "Young Adult", "Non-fiction", "Cookbooks"}},
	{"Ibadan Educational Supplies", "Musa Danjuma", 30, []string{"Textbooks & Exam Prep"}},
	{"Enugu Children's Books", "Chidi Okeke", 30, []string{"Children"}},
	{"Kano Faith & Life Books", "Fatima Sani", 21, []string{"Religion"}},
	{"Port Harcourt Business Press Sales", "Ebi Douglas", 30, []string{"Business & Self-help"}},
	{"Lekki Stationery Wholesale", "Bisi Coker", 14, []string{"Stationery"}},
}

type stationery struct {
	name  string
	price int64 // naira, 2023
	bulk  int   // typical quantity per sale
}

var stationeryItems = []stationery{
	{"Exercise book, 20 leaves", 150, 6}, {"Exercise book, 40 leaves", 250, 6}, {"Exercise book, 60 leaves", 350, 4}, {"Exercise book, 80 leaves", 450, 4},
	{"Hardcover notebook A5", 1500, 1}, {"Hardcover notebook A4", 2200, 1}, {"Graph book A4", 600, 2}, {"Drawing book A3", 700, 2},
	{"Ballpoint pen, blue", 100, 4}, {"Ballpoint pen, black", 100, 3}, {"Ballpoint pen, red", 100, 2}, {"Gel pen, assorted", 350, 2},
	{"Pencil HB", 80, 4}, {"Pencil 2B", 100, 3}, {"Eraser", 100, 2}, {"Sharpener", 100, 2}, {"Ruler 30cm", 200, 1},
	{"Mathematical set", 2500, 1}, {"Scientific calculator, two-line display", 9000, 1}, {"Highlighter, yellow", 500, 2},
	{"Permanent marker", 600, 1}, {"Whiteboard marker", 700, 2}, {"Crayons, 12 colours", 1200, 1}, {"Coloured pencils, 12", 1500, 1},
	{"Glue stick", 400, 1}, {"Sticky notes", 800, 1}, {"A4 paper, ream (500 sheets)", 4500, 1}, {"File jacket", 150, 3},
	{"Document wallet", 500, 2}, {"Pencil case", 1500, 1}, {"Book cover, plastic", 200, 4}, {"Bookmark", 300, 1},
	{"Stapler", 2500, 1}, {"Staples, box", 400, 1}, {"Envelope A4, brown", 100, 3}, {"Correction pen", 600, 1},
}

var firstNames = strings.Fields("Adaeze Adebayo Adekunle Adenike Ademola Afolabi Aisha Amaka Aminu Amina Babatunde Bisola Bolaji Chidi Chidinma Chinedu Chioma Chukwuemeka Damilola Danjuma Ebuka Efosa Emeka Esther Fatima Femi Folake Funke Gbenga Grace Habib Hadiza Halima Ifeanyi Ifeoma Ijeoma Ikechukwu Isioma Jide Joy Kayode Kehinde Kemi Lanre Lola Mariam Moses Musa Nkechi Nnamdi Obinna Oluwaseun Olumide Omolara Onyeka Osaro Rasheed Ronke Sade Samuel Segun Seun Shola Sunday Taiwo Temitope Tobi Tunde Uche Usman Victoria Wale Yemi Yetunde Zainab")
var lastNames = strings.Fields("Abiodun Adeleke Adeyemi Agbaje Ajayi Akande Akinola Alabi Aliyu Anyanwu Arinze Ayodele Babalola Bakare Balogun Bello Chukwu Dada Danladi Eke Eze Ezeh Fashola Garba Ibrahim Idowu Igwe Ihejirika Jimoh Kalu Lawal Mohammed Nnadi Nwachukwu Nwankwo Obi Ogbonna Ogunbiyi Okafor Okeke Okonkwo Okoro Olaniyan Olawale Oni Onyekachi Osagie Salami Sanni Shittu Suleiman Udeh Umar Uzor Yusuf")

var schools = []string{
	"Greenfield Academy, Ikeja", "Bright Stars College, Ogba", "Crescent Heights School, Maryland", "Royal Palm International School",
	"Ikeja Grammar Model School", "St. Agnes Girls' College (demo)", "Unity Hall Secondary School", "Lekki Pathfinders School",
	"Victory Gate College", "Ojodu Excellence Academy", "Harmony Montessori, Magodo", "Gateway Tutorial Centre",
}

// ---------- prices ----------

// priceIndex is how naira prices moved (inflation, exchange rate).
func priceIndex(t time.Time) float64 {
	switch {
	case t.Before(date("2024-01-02")):
		return 1.0
	case t.Before(date("2024-07-01")):
		return 1.4
	case t.Before(date("2025-01-02")):
		return 1.62
	case t.Before(date("2026-01-02")):
		return 1.95
	}
	return 2.15
}

var priceChanges = []string{"2024-01-02", "2024-07-01", "2025-01-02", "2026-01-02"}

func date(s string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", s, time.Local)
	return t
}

func h32(s string) uint32 {
	f := fnv.New32a()
	f.Write([]byte(s))
	return f.Sum32()
}

var priceRange = map[string][2]int64{
	"African Literature": {3500, 7500}, "Fiction": {5000, 11000}, "Young Adult": {4500, 9000}, "Children": {2000, 6500},
	"Non-fiction": {6500, 14000}, "Business & Self-help": {6000, 15000}, "Religion": {2500, 8000},
	"Textbooks & Exam Prep": {3000, 9500}, "Cookbooks": {6000, 14000}, "Poetry & Drama": {2500, 6000},
}

func basePrice(b Book) int64 {
	r := priceRange[b.Category]
	p := r[0] + int64(h32(b.ISBN)%uint32(r[1]-r[0]+1))
	if b.Pages > 0 {
		p += int64(b.Pages-250) * 6
	}
	return max(r[0], min(p, r[1]*13/10))
}

// roundPrice rounds naira to a shelf price (₦50 steps under 1,000, else ₦100 or ₦500).
func roundPrice(naira float64) int64 {
	step := 500.0
	switch {
	case naira < 1000:
		step = 50
	case naira < 5000:
		step = 100
	}
	return int64(math.Round(naira/step) * step)
}

// ---------- generator ----------

type item struct {
	variant, product string
	name, category   string
	base             int64 // naira, 2023
	bulk             int
	rank             int
	supplier         int
	input            protocol.ProductInput
	launch           time.Time // first on sale
	daily            float64   // expected units/day across tills (at scale)
	kiosk            bool      // also stocked at the campus kiosk
	created          bool
}

type gen struct {
	cfg  Config
	logf func(string, ...any)
	rng  *rand.Rand
	ctx  context.Context

	clockMu    sync.Mutex
	now        time.Time
	lateEvents int // events the clock had to push later (should stay ~0)
	invoiceSeq int

	h       *hub.Hub
	srv     *httptest.Server
	main    string // location ids
	kioskID string
	tills   [3]*terminal.Terminal
	tillDir [3]string

	users      map[string]terminal.User
	sessions   map[string]*hub.Session
	methods    map[string]protocol.PaymentMethod // cash, card, transfer, credit
	categories map[string]string
	items      []*item
	byCat      map[string][]*item
	supplierID []string

	stock      [2]map[string]int // [main, kiosk] variant -> units (our shadow of the hub)
	customers  []string          // customer ids, oldest first
	synced     int               // customers every till knew at the start of the day
	schoolIDs  []string
	recent     [3][]recentSale
	pendingPOs []pendingPO
	stats      struct{ sales, returns, pos, transfers, counts int }
}

type recentSale struct {
	receipt string
	at      time.Time
}

type pendingPO struct {
	id       string
	supplier int
	due      time.Time
}

func (g *gen) setClock(t time.Time) {
	g.clockMu.Lock()
	if !t.After(g.now) {
		if g.now.Sub(t) > time.Minute {
			g.lateEvents++
		}
		t = g.now.Add(time.Millisecond)
	}
	g.now = t
	g.clockMu.Unlock()
}

func (g *gen) clock() time.Time {
	g.clockMu.Lock()
	defer g.clockMu.Unlock()
	return g.now
}

func (g *gen) price(it *item, t time.Time) int64 {
	return roundPrice(float64(it.base)*priceIndex(t)) * 100
}

// Generate builds the store and writes it to cfg.Out.
func Generate(ctx context.Context, cfg Config, logf func(string, ...any)) error {
	if cfg.Scale <= 0 {
		cfg.Scale = 1
	}
	if !cfg.To.After(cfg.From) {
		return errors.New("the last day must be after the first")
	}
	books, err := Books()
	if err != nil || len(books) == 0 {
		return fmt.Errorf("no catalogue: run zebidemo catalogue first (%v)", err)
	}
	tmp, err := os.MkdirTemp("", "zebidemo-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	g := &gen{cfg: cfg, logf: logf, ctx: ctx, rng: rand.New(rand.NewPCG(uint64(cfg.Seed), 7)), now: cfg.From.AddDate(0, 0, -3).Add(9 * time.Hour),
		users: map[string]terminal.User{}, sessions: map[string]*hub.Session{}, methods: map[string]protocol.PaymentMethod{},
		categories: map[string]string{}, byCat: map[string][]*item{}}
	g.stock[0], g.stock[1] = map[string]int{}, map[string]int{}
	ids.SetClock(g.clock)
	defer ids.SetClock(nil)

	if g.h, err = hub.Open(ctx, hub.Config{DataDir: filepath.Join(tmp, "hub"), Version: appdata.Version, Logger: quiet}); err != nil {
		return err
	}
	defer g.h.Close()
	g.srv = httptest.NewServer(g.h.Handler())
	defer g.srv.Close()
	start := time.Now()
	if err := g.setup(books); err != nil {
		return fmt.Errorf("setting up: %w", err)
	}
	defer func() {
		for _, t := range g.tills {
			if t != nil {
				t.Close()
			}
		}
	}()
	logf("store set up: %d products, %d users (%s)", len(g.items), len(g.users), time.Since(start).Round(time.Second))

	for day := cfg.From; !day.After(cfg.To); day = day.AddDate(0, 0, 1) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := g.day(day); err != nil {
			return fmt.Errorf("%s: %w", day.Format("2 Jan 2006"), err)
		}
		if day.Day() == 1 || day.Equal(cfg.To) {
			logf("%s  sales %d  returns %d  orders %d  transfers %d  (%s)", day.Format("Jan 2006"), g.stats.sales, g.stats.returns, g.stats.pos,
				g.stats.transfers, time.Since(start).Round(time.Second))
		}
	}
	if err := g.waitSynced(); err != nil {
		return err
	}
	// Final touches at the end of the last day.
	g.setClock(cfg.To.Add(20 * time.Hour))
	if err := os.MkdirAll(filepath.Dir(cfg.Out), 0o755); err != nil && filepath.Dir(cfg.Out) != "." {
		return err
	}
	_ = os.Remove(cfg.Out)
	ids.SetClock(nil)
	if err := g.h.DB().Backup(ctx, cfg.Out); err != nil {
		return err
	}
	fi, _ := os.Stat(cfg.Out)
	logf("events pushed later by back-office work: %d", g.lateEvents)
	logf("done: %d sales, %d returns, %d purchase orders, %d transfers, %d stock counts -> %s (%.0f MB, %s)",
		g.stats.sales, g.stats.returns, g.stats.pos, g.stats.transfers, g.stats.counts, cfg.Out, float64(fi.Size())/1e6, time.Since(start).Round(time.Second))
	return nil
}

func (g *gen) session(user string) *hub.Session { return g.sessions[user] }

func (g *gen) setup(books []Book) error {
	ctx, h := g.ctx, g.h
	if err := h.Setup(ctx, hub.SetupInput{StoreName: "TMIT Bookstore", BusinessType: "bookshop", LocationName: "TMIT Main Store",
		Address: "14 Oduduwa Way, Ikeja GRA, Lagos", Phone: "0809 555 0142", CurrencyCode: "NGN", CurrencySym: "₦",
		AdminName: staff[0].name, AdminUsername: staff[0].user, AdminPassword: DemoPassword}); err != nil {
		return err
	}
	sys := hub.SystemSession
	if err := h.SaveSettings(ctx, sys, map[string]string{"receipt_footer": "Thank you for shopping at TMIT Bookstore!\nBooks bought can be exchanged within 14 days with this receipt.",
		"tax_id": "TIN 12345678-0001", "receipt_header": "Books • Stationery • School supplies"}); err != nil {
		return err
	}
	g.main, _ = h.Setting(ctx, "default_location_id")
	var err error
	if g.kioskID, err = h.SaveLocation(ctx, sys, protocol.Location{Name: "TMIT Campus Kiosk", Code: "KIOSK", Active: true}); err != nil {
		return err
	}

	// Staff.
	for _, s := range staff {
		var id string
		if s.user == "admin" {
			_ = h.DB().R().QueryRow("SELECT id FROM users WHERE username='admin'").Scan(&id)
		} else {
			in := protocol.UserInput{Name: s.name, Username: s.user, Role: s.role, Active: true, Password: DemoPassword}
			if s.role == protocol.RoleCashier {
				in.Pin = DemoPIN
			}
			if id, err = h.SaveUser(ctx, sys, in); err != nil {
				return err
			}
		}
		g.users[s.user] = terminal.User{ID: id, Name: s.name, Username: s.user, Role: s.role}
		g.sessions[s.user] = &hub.Session{UserID: id, Name: s.name, Role: s.role}
	}

	// Payment methods: the template's, plus schools' accounts.
	if _, err := h.SavePaymentMethod(ctx, sys, protocol.PaymentMethod{Name: "School account", Kind: protocol.PayCredit, Active: true, SortOrder: 9}); err != nil {
		return err
	}
	rows, err := h.DB().R().QueryContext(ctx, "SELECT id,name,kind FROM payment_methods WHERE active=1")
	if err != nil {
		return err
	}
	for rows.Next() {
		var m protocol.PaymentMethod
		rows.Scan(&m.ID, &m.Name, &m.Kind)
		g.methods[m.Kind] = m
	}
	rows.Close()

	// Categories.
	cats := []string{"Stationery"}
	for _, sh := range Shelves {
		cats = append(cats, sh.Category)
	}
	for _, c := range cats {
		if g.categories[c] != "" {
			continue
		}
		var id string
		_ = h.DB().R().QueryRow("SELECT id FROM categories WHERE lower(name)=lower(?)", c).Scan(&id)
		if id == "" {
			if id, err = h.SaveCategory(ctx, sys, protocol.Category{Name: c, Active: true}); err != nil {
				return err
			}
		}
		g.categories[c] = id
	}

	// Suppliers.
	for _, s := range suppliers {
		id, err := h.SaveSupplier(ctx, sys, protocol.Supplier{Name: s.name, ContactName: s.contact, Phone: fmt.Sprintf("0803 555 %04d", h32(s.name)%10000),
			Address: "Lagos", TermsDays: int64(s.terms), Active: true})
		if err != nil {
			return err
		}
		g.supplierID = append(g.supplierID, id)
	}

	// Catalogue: books, then stationery. A slice of the books arrive later.
	span := g.cfg.To.Sub(g.cfg.From)
	for _, b := range books {
		it := &item{name: b.Title, category: b.Category, base: basePrice(b), bulk: 1, rank: b.Rank, launch: g.cfg.From}
		if b.Rank > 6 && h32(b.ISBN+"new")%100 < 14 {
			it.launch = g.cfg.From.Add(time.Duration(float64(span) * (0.05 + 0.9*float64(h32(b.ISBN+"when")%1000)/1000))).Truncate(24 * time.Hour)
		}
		edition := ""
		if b.Year > 0 {
			edition = fmt.Sprint(b.Year)
		}
		it.input = protocol.ProductInput{
			Product: protocol.Product{Name: b.Title, CategoryID: g.categories[b.Category], TrackStock: true, Unit: "pc", Active: true,
				Attributes: map[string]string{"author": b.Author, "isbn": b.ISBN, "publisher": b.Publisher, "edition": edition}},
			Variants: []protocol.VariantInput{{Variant: protocol.Variant{Active: true}, Barcodes: []string{b.ISBN}}},
		}
		g.items = append(g.items, it)
		g.byCat[b.Category] = append(g.byCat[b.Category], it)
		_ = b
	}
	for i, s := range stationeryItems {
		code := ean13(fmt.Sprintf("200%09d", 1000+i))
		it := &item{name: s.name, category: "Stationery", base: s.price, bulk: s.bulk, rank: i, launch: g.cfg.From}
		it.input = protocol.ProductInput{
			Product:  protocol.Product{Name: s.name, CategoryID: g.categories["Stationery"], TrackStock: true, Unit: "pc", Active: true, Attributes: map[string]string{}},
			Variants: []protocol.VariantInput{{Variant: protocol.Variant{Active: true}, Barcodes: []string{code}}},
		}
		g.items = append(g.items, it)
		g.byCat["Stationery"] = append(g.byCat["Stationery"], it)
	}
	for s, sd := range suppliers {
		for _, c := range sd.categories {
			for _, it := range g.byCat[c] {
				it.supplier = s
			}
		}
	}
	// Expected demand per item (drives stock levels and reorders).
	for ti, td := range tills {
		avgLines := 0.0
		for i, p := range td.lines {
			avgLines += float64(i+1) * p
		}
		for c, w := range td.weights {
			list := g.byCat[c]
			norm := 0.0
			for _, it := range list {
				norm += zipf(it.rank)
			}
			for _, it := range list {
				d := td.base * g.cfg.Scale * avgLines * w * zipf(it.rank) / norm * float64(it.bulk)
				it.daily += d
				if ti == 2 && (c == "Textbooks & Exam Prep" || c == "Stationery" || it.rank < 12) {
					it.kiosk = true
				}
			}
		}
	}
	g.setClock(g.cfg.From.AddDate(0, 0, -2).Add(10 * time.Hour))
	for _, it := range g.items {
		if !it.launch.After(g.cfg.From) {
			if err := g.createItem(it, true, books); err != nil {
				return err
			}
		}
	}

	// Tills.
	for i, td := range tills {
		loc := g.main
		if td.kiosk {
			loc = g.kioskID
		}
		code, err := h.CreatePairingCode(ctx, "", protocol.PairingCodeRequest{TerminalName: td.name, LocationID: loc})
		if err != nil {
			return err
		}
		g.tillDir[i], _ = os.MkdirTemp("", "zebidemo-till-")
		tm, err := terminal.Open(ctx, terminal.Config{DataDir: g.tillDir[i], Version: appdata.Version, Logger: quiet})
		if err != nil {
			return err
		}
		if _, err := tm.Pair(ctx, g.srv.URL, code.Code, td.name); err != nil {
			return err
		}
		tm.StartSync()
		g.tills[i] = tm
	}

	// Opening stock at the kiosk, sent from the main store.
	var lines []protocol.StockTransferLine
	for _, it := range g.items {
		if it.kiosk && it.created {
			q := kioskTarget(it)
			if q > 0 && g.stock[0][it.variant] > q {
				lines = append(lines, protocol.StockTransferLine{VariantID: it.variant, QtyMilli: int64(q) * 1000})
				g.stock[0][it.variant] -= q
				g.stock[1][it.variant] += q
			}
		}
	}
	if _, err := h.TransferStock(ctx, g.session("femi"), hub.TransferInput{FromLocationID: g.main, ToLocationID: g.kioskID, ReceiveNow: true,
		Note: "Opening stock for the kiosk", Lines: lines}); err != nil {
		return err
	}

	// Customers: schools on account, and the first regulars.
	g.setClock(g.cfg.From.AddDate(0, 0, -1).Add(11 * time.Hour))
	for _, s := range schools {
		c, err := g.tills[0].SaveCustomer(ctx, protocol.Customer{Name: s, Phone: fmt.Sprintf("0812 555 %04d", h32(s)%10000), Notes: "School account: pays monthly", Active: true})
		if err != nil {
			return err
		}
		g.schoolIDs = append(g.schoolIDs, c.ID)
	}
	for range 120 {
		if err := g.newCustomer(0); err != nil {
			return err
		}
	}
	return g.waitSynced()
}

func zipf(rank int) float64 { return 1 / math.Pow(float64(rank)+2, 0.85) }

func kioskTarget(it *item) int { return max(3, int(math.Ceil(it.daily*0.35*10))) }

func mainTarget(it *item) int { return max(4, min(400, int(math.Ceil(it.daily*24)))) }

func ean13(twelve string) string {
	sum := 0
	for i, c := range twelve {
		d := int(c - '0')
		if i%2 == 1 {
			d *= 3
		}
		sum += d
	}
	return twelve + fmt.Sprint((10-sum%10)%10)
}

// createItem adds a product (with its cover). Opening stock for the
// launch catalogue; later titles arrive through a purchase order.
func (g *gen) createItem(it *item, opening bool, books []Book) error {
	ctx, h := g.ctx, g.h
	t := g.clock()
	v := &it.input.Variants[0]
	v.PriceMinor = g.price(it, t)
	v.CostMinor = v.PriceMinor * 68 / 100
	v.ReorderLevelMilli = int64(max(2, mainTarget(it)/3)) * 1000
	if opening {
		q := mainTarget(it) + max(1, mainTarget(it)/2)
		v.OpeningQtyMilli = int64(q) * 1000
		g.stock[0][""] = 0
	}
	id, err := h.SaveProduct(ctx, g.session("femi"), it.input)
	if err != nil {
		return fmt.Errorf("%s: %w", it.name, err)
	}
	it.product = id
	it.input.Product.ID = id
	if err := h.DB().R().QueryRow("SELECT id, sku FROM variants WHERE product_id=?", id).Scan(&it.variant, &v.SKU); err != nil {
		return err
	}
	v.ID, v.ProductID, v.OpeningQtyMilli = it.variant, id, 0
	if opening {
		g.stock[0][it.variant] = mainTarget(it) + max(1, mainTarget(it)/2)
	}
	delete(g.stock[0], "")
	it.created = true
	if it.category != "Stationery" {
		var b Book
		for _, x := range books {
			if x.ISBN == it.input.Product.Attributes["isbn"] {
				b = x
				break
			}
		}
		if _, err := h.SetProductImage(ctx, g.session("femi"), id, Cover(g.cfg.CoverCache, b)); err != nil {
			return fmt.Errorf("cover for %s: %w", it.name, err)
		}
	}
	return nil
}

func (g *gen) newCustomer(till int) error {
	f := firstNames[g.rng.IntN(len(firstNames))]
	l := lastNames[g.rng.IntN(len(lastNames))]
	prefix := []string{"0803", "0806", "0813", "0816", "0802", "0808", "0705", "0810", "0903", "0905"}[g.rng.IntN(10)]
	// Unique, obviously made-up numbers (555 in the middle).
	c, err := g.tills[till].SaveCustomer(g.ctx, protocol.Customer{Name: f + " " + l, Phone: fmt.Sprintf("%s 555 %04d", prefix, len(g.customers)), Active: true})
	if err != nil {
		return err
	}
	g.customers = append(g.customers, c.ID)
	return nil
}

// waitSynced waits until every till has sent its work and has the hub's latest.
func (g *gen) waitSynced() error {
	deadline := time.Now().Add(2 * time.Minute)
	for _, t := range g.tills {
		t.Nudge()
	}
	for {
		rev := g.h.Info(g.ctx).Rev
		done := true
		for _, t := range g.tills {
			s := t.Status()
			if s.Pending > 0 || s.Rev < rev {
				done = false
			}
			if s.Rejected > 0 {
				return fmt.Errorf("the hub refused an operation from a till: %s", s.LastError)
			}
		}
		if done {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("tills did not sync in time")
		}
		time.Sleep(3 * time.Millisecond)
	}
}

// ---------- one day ----------

type event struct {
	at   time.Time
	till int
	kind int // 0 sale, 1 open shift, 2 close shift, 3 return, 4 paid out, 5 school order, 6 account payment, 7 new customer, 8 no-receipt return
	n    int
}

var monthFactor = [13]float64{0, 1.25, 1.0, .95, .95, .9, .9, .85, .95, 1.45, 1.05, 1.1, 1.5}
var weekdayFactor = [7]float64{.4, .85, .9, .95, 1, 1.15, 1.45}
var yearFactor = map[int]float64{2023: 1, 2024: .93, 2025: 1.04, 2026: 1.11}

func closedDay(d time.Time) bool {
	return (d.Month() == 12 && d.Day() == 25) || (d.Month() == 1 && d.Day() == 1) || (d.Month() == 12 && d.Day() == 26)
}

func (g *gen) day(d time.Time) error {
	if closedDay(d) {
		return nil
	}
	if err := g.morning(d); err != nil {
		return err
	}
	if err := g.waitSynced(); err != nil {
		return err
	}
	g.synced = len(g.customers)
	var evs []event
	wd := int(d.Weekday())
	for ti, td := range tills {
		o := td.open[wd]
		if o[1] == 0 {
			continue
		}
		f := td.base * g.cfg.Scale * monthFactor[d.Month()] * weekdayFactor[wd] * yearFactor[d.Year()]
		if d.Month() == 8 && d.Day() >= 20 {
			f *= 1.4 // back to school
		}
		if td.kiosk && (d.Month() == 8 || (d.Month() == 12 && d.Day() > 18)) {
			f *= 0.45 // campus quiet
		}
		if ti == 1 && wd == 6 {
			f *= 1.3 // family Saturdays
		}
		if d.Year() == 2023 && d.Month() == 2 {
			f *= 0.82 // naira notes shortage
		}
		f *= math.Exp(g.rng.NormFloat64() * 0.18)
		n := int(math.Round(f))
		openAt := d.Add(time.Duration(o[0])*time.Hour + time.Duration(g.rng.IntN(10))*time.Minute)
		closeAt := d.Add(time.Duration(o[1])*time.Hour + time.Duration(35+g.rng.IntN(20))*time.Minute) // after the last customer
		evs = append(evs, event{at: openAt, till: ti, kind: 1}, event{at: closeAt, till: ti, kind: 2})
		for range n {
			evs = append(evs, event{at: g.saleTime(d, o), till: ti})
		}
		for range int(math.Round(float64(n) * 0.007 * g.rng.Float64() * 2)) {
			evs = append(evs, event{at: g.saleTime(d, o), till: ti, kind: 3})
		}
		if g.rng.Float64() < 0.004*float64(n) {
			evs = append(evs, event{at: g.saleTime(d, o), till: ti, kind: 8})
		}
		if ti == 0 && g.rng.Float64() < 0.35 {
			evs = append(evs, event{at: g.saleTime(d, o), till: ti, kind: 4})
		}
		if ti == 0 {
			for range g.rng.IntN(2) + int(g.cfg.Scale*0.3+g.rng.Float64()) {
				evs = append(evs, event{at: g.saleTime(d, o), till: ti, kind: 7})
			}
		}
		// Schools stock up at the start of term.
		if ti != 1 && wd >= 1 && wd <= 5 && ((d.Month() == 1 && d.Day() <= 25) || (d.Month() == 9 && d.Day() <= 25) || (d.Month() == 4 && d.Day() >= 15)) {
			for range g.rng.IntN(3) {
				evs = append(evs, event{at: g.saleTime(d, o), till: ti, kind: 5})
			}
		}
		if ti == 0 && d.Day() >= 10 && d.Day() <= 20 && wd >= 1 && wd <= 5 {
			evs = append(evs, event{at: g.saleTime(d, o), till: ti, kind: 6, n: g.rng.IntN(len(schools))})
		}
	}
	sort.SliceStable(evs, func(i, j int) bool {
		if !evs[i].at.Equal(evs[j].at) {
			return evs[i].at.Before(evs[j].at)
		}
		return evs[i].kind == 1 // shifts open before anything else at the same moment
	})
	for _, e := range evs {
		g.setClock(e.at)
		if err := g.event(d, e); err != nil {
			return err
		}
	}
	return g.waitSynced()
}

// saleTime picks a moment in opening hours, busier at lunch and after work.
func (g *gen) saleTime(d time.Time, o [2]int) time.Time {
	for {
		h := o[0] + g.rng.IntN(o[1]-o[0])
		w := 1.0
		switch {
		case h >= 12 && h < 14:
			w = 1.6
		case h >= 16 && h < 18:
			w = 1.35
		case h == o[0]:
			w = 0.5
		}
		if g.rng.Float64()*1.6 < w {
			return d.Add(time.Duration(h)*time.Hour + time.Duration(g.rng.IntN(3600))*time.Second + 30*time.Minute)
		}
	}
}

func (g *gen) active(user string, d time.Time) bool {
	for _, s := range staff {
		if s.user == user {
			return (s.from == "" || !d.Before(date(s.from))) && (s.until == "" || !d.After(date(s.until)))
		}
	}
	return false
}

func (g *gen) cashier(till int, d time.Time) terminal.User {
	var pool []string
	for _, s := range staff {
		if s.role == protocol.RoleCashier && g.active(s.user, d) && (s.till == till || (till == 0 && s.till == 0)) {
			pool = append(pool, s.user)
		}
	}
	if len(pool) == 0 {
		return g.users["chiamaka"]
	}
	return g.users[pool[(d.YearDay()+till)%len(pool)]]
}

func (g *gen) manager(d time.Time) terminal.User {
	if g.active("ibrahim", d) && d.Weekday()%2 == 0 {
		return g.users["ibrahim"]
	}
	return g.users["chiamaka"]
}

func (g *gen) payment(till int, d time.Time) protocol.PaymentMethod {
	cash, card := .38, .37
	switch {
	case d.Year() == 2023 && d.Month() <= 1:
		cash, card = .45, .35
	case d.Year() == 2023 && d.Month() <= 3:
		cash, card = .12, .43 // cash scarcity
	case d.Year() == 2024:
		cash, card = .28, .4
	case d.Year() == 2025:
		cash, card = .22, .4
	case d.Year() >= 2026:
		cash, card = .18, .4
	}
	cash -= tills[till].transfer / 2
	card -= tills[till].transfer / 2
	r := g.rng.Float64()
	switch {
	case r < cash:
		return g.methods[protocol.PayCash]
	case r < cash+card:
		return g.methods[protocol.PayCard]
	}
	return g.methods[protocol.PayTransfer]
}

func (g *gen) pick(till int, d time.Time) *item {
	td := tills[till]
	for tries := 0; tries < 40; tries++ {
		r := g.rng.Float64()
		cat := ""
		for c, w := range td.weights {
			if r < w {
				cat = c
				break
			}
			r -= w
		}
		if cat == "" {
			continue
		}
		list := g.byCat[cat]
		// Zipf over the category's popularity ranks.
		norm := 0.0
		for _, it := range list {
			norm += zipf(it.rank)
		}
		x := g.rng.Float64() * norm
		for _, it := range list {
			x -= zipf(it.rank)
			if x <= 0 {
				loc := 0
				if td.kiosk {
					loc = 1
				}
				if it.created && !it.launch.After(d) && g.stock[loc][it.variant] > 0 {
					return it
				}
				break
			}
		}
	}
	return nil
}

func (g *gen) event(d time.Time, e event) error {
	ctx, tm := g.ctx, g.tills[e.till]
	loc := 0
	if tills[e.till].kiosk {
		loc = 1
	}
	switch e.kind {
	case 1: // open shift
		_, err := tm.OpenShift(ctx, g.cashier(e.till, d), int64(roundPrice(15000*priceIndex(d)))*100)
		return err
	case 2: // close shift: count the drawer, now and then a little out
		sh, err := tm.CurrentShift(ctx)
		if err != nil || sh == nil {
			return err
		}
		counted := sh.Summary.ExpectedCash
		note := ""
		switch r := g.rng.Float64(); {
		case r < 0.07:
			counted -= int64(1+g.rng.IntN(20)) * 10000
			note = "Short; recounted twice"
		case r < 0.1:
			counted += int64(1+g.rng.IntN(5)) * 10000
			note = "Over"
		}
		_, err = tm.CloseShift(ctx, g.cashier(e.till, d), counted, note)
		return err
	case 4: // paid out
		reasons := []struct {
			why string
			amt float64
		}{{"Diesel for the generator", 12000}, {"Diesel for the generator", 18000}, {"Cleaning supplies", 3500}, {"Drinking water", 2500}, {"Dispatch rider", 4000}}
		r := reasons[g.rng.IntN(len(reasons))]
		_, err := tm.AddCashMovement(ctx, g.cashier(e.till, d), ptr(g.manager(d)), protocol.CashPaidOut, roundPrice(r.amt*priceIndex(d)*(0.8+0.4*g.rng.Float64()))*100, r.why)
		return err
	case 7:
		return g.newCustomer(e.till)
	case 6: // a school pays its account
		c := g.schoolIDs[e.n]
		found, _ := tm.SearchCustomers(ctx, "", 500)
		for _, x := range found {
			if x.ID == c && x.BalanceMinor > 5_000_00 {
				amt := x.BalanceMinor * int64(60+g.rng.IntN(41)) / 100 / 100_000 * 100_000
				if amt <= 0 {
					return nil
				}
				_, err := tm.RecordAccountPayment(ctx, g.cashier(e.till, d), terminal.AccountPaymentInput{CustomerID: c, MethodID: g.methods[protocol.PayTransfer].ID,
					AmountMinor: amt, Reference: fmt.Sprintf("TRF%08d", g.rng.IntN(1e8)), Note: "Termly book list payment"})
				return err
			}
		}
		return nil
	case 3: // return with the receipt, within 14 days
		rs := g.recent[e.till]
		if len(rs) == 0 {
			return nil
		}
		pick := rs[g.rng.IntN(len(rs))]
		ret, err := tm.LookupForReturn(ctx, pick.receipt)
		if err != nil {
			return nil
		}
		for _, l := range ret.Lines {
			if l.ReturnedMilli == 0 && l.QtyMilli == 1000 && l.PaidMinor > 0 {
				restock := g.rng.Float64() < 0.85
				reasons := []string{"Changed mind", "Wrong item", "Duplicate copy", "Damaged", "Faulty / defective"}
				why := reasons[g.rng.IntN(len(reasons))]
				if why == "Damaged" || why == "Faulty / defective" {
					restock = false
				}
				method := g.methods[protocol.PayCash]
				if len(ret.Sale.Payments) == 1 && ret.Sale.Payments[0].Kind != protocol.PayCredit {
					method.ID = ret.Sale.Payments[0].MethodID
				}
				_, err := tm.RecordReturn(ctx, g.cashier(e.till, d), ptr(g.manager(d)), terminal.ReturnInput{ReceiptNo: pick.receipt, Reason: why,
					Lines:   []terminal.ReturnLineInput{{SaleLineID: l.ID, QtyMilli: 1000, Restock: restock}},
					Refunds: []terminal.PaymentInput{{MethodID: method.ID, AmountMinor: l.PaidMinor, Reference: "refund"}}})
				if err == nil {
					g.stats.returns++
					if restock {
						g.stock[loc][l.VariantID]++
					}
				}
				return err
			}
		}
		return nil
	case 8: // no receipt: store credit for a known customer
		it := g.pick(e.till, d)
		if it == nil || g.synced < 10 {
			return nil
		}
		price, _ := tm.NoReceiptPrice(ctx, it.variant)
		_, err := tm.RecordReturnNoReceipt(ctx, g.cashier(e.till, d), ptr(g.manager(d)), terminal.NoReceiptInput{
			Lines: []terminal.NoReceiptLine{{VariantID: it.variant, QtyMilli: 1000, Restock: true}}, CustomerID: g.customers[g.rng.IntN(g.synced)],
			Refunds: []terminal.PaymentInput{{MethodID: terminal.StoreCreditMethodID, AmountMinor: price}}, Reason: "Gift, no receipt"})
		if err == nil {
			g.stats.returns++
			g.stock[loc][it.variant]++
		}
		return err
	}

	// Sales (kind 0) and school orders (kind 5).
	in := terminal.SaleInput{}
	var total int64
	add := func(it *item, qty int) {
		have := g.stock[loc][it.variant]
		qty = min(qty, have)
		if qty <= 0 {
			return
		}
		for _, l := range in.Lines {
			if l.VariantID == it.variant {
				return
			}
		}
		in.Lines = append(in.Lines, terminal.CartLine{VariantID: it.variant, QtyMilli: int64(qty) * 1000})
		total += g.price(it, d) * int64(qty)
		g.stock[loc][it.variant] -= qty
	}
	school := e.kind == 5
	if school {
		in.CustomerID = g.schoolIDs[g.rng.IntN(len(g.schoolIDs))]
		for range 1 + g.rng.IntN(3) {
			if it := g.pickFrom("Textbooks & Exam Prep", loc, d); it != nil {
				add(it, 10+g.rng.IntN(40))
			}
		}
		if it := g.pickFrom("Stationery", loc, d); it != nil {
			add(it, 20+g.rng.IntN(60))
		}
	} else {
		r := g.rng.Float64()
		n := 1
		for i, p := range tills[e.till].lines {
			if r < p {
				n = i + 1
				break
			}
			r -= p
			n = i + 2
		}
		for range n {
			if it := g.pick(e.till, d); it != nil {
				q := 1
				if it.bulk > 1 {
					q = 1 + g.rng.IntN(it.bulk)
				}
				add(it, q)
			}
		}
		if g.rng.Float64() < 0.22 && g.synced > 5 {
			// Regulars come back: favour earlier customers.
			k := int(float64(g.synced) * math.Pow(g.rng.Float64(), 1.6))
			in.CustomerID = g.customers[min(k, g.synced-1)]
		}
	}
	if len(in.Lines) == 0 {
		return nil
	}
	cashier := g.cashier(e.till, d)
	var approver *terminal.User
	bookWeek := (d.Month() == 11 && d.Day() >= 24) || (d.Month() == 4 && d.Day() >= 21 && d.Day() <= 25)
	switch {
	case school && total > 50_000_00:
		in.DiscountMinor = total * 10 / 100
	case bookWeek && g.rng.Float64() < 0.45:
		in.DiscountMinor = total * 10 / 100
	case g.rng.Float64() < 0.03:
		in.DiscountMinor = total * int64(5+5*g.rng.IntN(2)) / 100
	}
	if in.DiscountMinor > 0 {
		in.DiscountMinor = in.DiscountMinor / 100 * 100
		approver = ptr(g.manager(d))
		in.Note = map[bool]string{true: "School discount", false: "Book week"}[school]
	}
	due := total - in.DiscountMinor
	if school {
		in.Payments = []terminal.PaymentInput{{MethodID: g.methods[protocol.PayCredit].ID, AmountMinor: due}}
	} else {
		m := g.payment(e.till, d)
		amt := due
		ref := ""
		if m.Kind == protocol.PayCash {
			note := int64(50000) // ₦500 notes
			if due > 500000 {
				note = 100000
			}
			amt = (due + note - 1) / note * note
			if g.rng.Float64() < 0.4 {
				amt = due
			}
		} else if m.RequiresReference || m.Kind == protocol.PayTransfer {
			ref = fmt.Sprintf("TRF%08d", g.rng.IntN(1e8))
		}
		in.Payments = []terminal.PaymentInput{{MethodID: m.ID, AmountMinor: amt, Reference: ref}}
	}
	s, err := tm.RecordSale(ctx, cashier, approver, in)
	if err != nil {
		var se terminal.SaleError
		if errors.As(err, &se) {
			// Put the shadow stock back and carry on; the generator's
			// price view was out of step (logged, rare).
			g.logf("skipped a sale at %s: %v", tills[e.till].name, err)
			for _, l := range in.Lines {
				g.stock[loc][l.VariantID] += int(l.QtyMilli / 1000)
			}
			return nil
		}
		return err
	}
	g.stats.sales++
	g.recent[e.till] = append(g.recent[e.till], recentSale{s.ReceiptNo, d})
	cut := 0
	for cut < len(g.recent[e.till]) && d.Sub(g.recent[e.till][cut].at) > 14*24*time.Hour {
		cut++
	}
	g.recent[e.till] = g.recent[e.till][cut:]
	return nil
}

func (g *gen) pickFrom(cat string, loc int, d time.Time) *item {
	list := g.byCat[cat]
	for range 20 {
		it := list[min(len(list)-1, int(float64(len(list))*math.Pow(g.rng.Float64(), 2.2)))]
		if it.created && !it.launch.After(d) && g.stock[loc][it.variant] > 0 {
			return it
		}
	}
	return nil
}

func ptr[T any](v T) *T { return &v }

// ---------- back office, each morning ----------

func (g *gen) morning(d time.Time) error {
	ctx, h := g.ctx, g.h
	g.setClock(d.Add(6*time.Hour + 20*time.Minute))
	femi, mgr := g.session("femi"), g.session(g.manager(d).Username)

	// Staff who have left.
	for _, s := range staff {
		if s.until != "" && d.Equal(date(s.until).AddDate(0, 0, 1)) {
			u := g.users[s.user]
			if _, err := h.SaveUser(ctx, hub.SystemSession, protocol.UserInput{ID: u.ID, Name: u.Name, Username: u.Username, Role: u.Role, Active: false}); err != nil {
				return err
			}
		}
	}

	// Price rises.
	for _, pc := range priceChanges {
		if d.Equal(date(pc)) {
			for _, it := range g.items {
				if !it.created {
					continue
				}
				v := &it.input.Variants[0]
				v.PriceMinor = g.price(it, d)
				v.Barcodes = nil // unchanged
				in := it.input
				in.Variants = []protocol.VariantInput{*v}
				if _, err := h.SaveProduct(ctx, mgr, in); err != nil {
					return fmt.Errorf("price change %s: %w", it.name, err)
				}
			}
			g.logf("%s: prices raised (index %.2f)", pc, priceIndex(d))
		}
	}

	// New titles.
	books, _ := Books()
	for _, it := range g.items {
		if !it.created && !it.launch.After(d) {
			g.setClock(d.Add(6*time.Hour + 25*time.Minute))
			if err := g.createItem(it, false, books); err != nil {
				return err
			}
		}
	}

	// Deliveries due today.
	var keep []pendingPO
	for _, po := range g.pendingPOs {
		if po.due.After(d) {
			keep = append(keep, po)
			continue
		}
		if err := g.receive(po, d); err != nil {
			return err
		}
	}
	g.pendingPOs = keep

	// Orders twice a week (Mondays and Thursdays).
	if wd := d.Weekday(); wd == time.Monday || wd == time.Thursday {
		if err := g.reorder(d); err != nil {
			return err
		}
	}
	// Topping up the kiosk (Tuesdays and Fridays), received at the kiosk till.
	if wd := d.Weekday(); wd == time.Tuesday || wd == time.Friday {
		if err := g.kioskRun(d); err != nil {
			return err
		}
	}
	// Supplier payments on Fridays: bills due within a week.
	if d.Weekday() == time.Friday {
		if err := g.payBills(d, mgr); err != nil {
			return err
		}
	}
	// Quarterly stock count on the first Sunday of the quarter.
	if d.Weekday() == time.Sunday && d.Day() <= 7 && (d.Month()-1)%3 == 0 {
		if err := g.stockCount(d, femi); err != nil {
			return err
		}
	}
	return nil
}

func (g *gen) reorder(d time.Time) error {
	ctx, h := g.ctx, g.h
	bySupplier := map[int][]protocol.PurchaseOrderLine{}
	for _, it := range g.items {
		if !it.created {
			continue
		}
		have := g.stock[0][it.variant]
		for _, po := range g.pendingPOs {
			_ = po
		}
		if have > mainTarget(it)/3 || g.onOrder(it) {
			continue
		}
		q := mainTarget(it) - have + kioskTarget(it)/2
		if it.category == "Stationery" {
			q = (q + 11) / 12 * 12 // by the dozen
		}
		bySupplier[it.supplier] = append(bySupplier[it.supplier], protocol.PurchaseOrderLine{VariantID: it.variant, QtyOrderedMilli: int64(q) * 1000,
			UnitCostMinor: g.price(it, d) * int64(64+g.rng.IntN(8)) / 100})
	}
	keys := make([]int, 0, len(bySupplier))
	for k := range bySupplier {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, s := range keys {
		g.setClock(d.Add(6*time.Hour + 40*time.Minute))
		id, err := h.SavePurchaseOrder(ctx, g.session("femi"), protocol.PurchaseOrderInput{SupplierID: g.supplierID[s], LocationID: g.main, Submit: true,
			ExpectedAt: d.AddDate(0, 0, 5).UnixMilli(), Lines: bySupplier[s]})
		if err != nil {
			return err
		}
		g.stats.pos++
		g.pendingPOs = append(g.pendingPOs, pendingPO{id: id, supplier: s, due: d.AddDate(0, 0, 3+g.rng.IntN(7))})
		g.ordered(bySupplier[s])
	}
	return nil
}

var onOrder = map[string]bool{}

func (g *gen) onOrder(it *item) bool { return onOrder[it.variant] }
func (g *gen) ordered(lines []protocol.PurchaseOrderLine) {
	for _, l := range lines {
		onOrder[l.VariantID] = true
	}
}

func (g *gen) receive(po pendingPO, d time.Time) error {
	ctx, h := g.ctx, g.h
	o, err := h.GetPurchaseOrder(ctx, hub.SystemSession, po.id)
	if err != nil {
		return err
	}
	g.invoiceSeq++ // suppliers number their invoices; never the same twice
	in := protocol.GoodsReceiptInput{POID: o.ID, SupplierRef: fmt.Sprintf("INV-%d-%05d", d.Year()%100, 10000+g.invoiceSeq*7%90000+g.invoiceSeq/90000), ClosePO: true}
	for _, l := range o.Lines {
		q := l.QtyOrderedMilli
		if g.rng.Float64() < 0.07 {
			q = q * int64(70+g.rng.IntN(25)) / 100 / 1000 * 1000 // short delivery
		}
		in.Lines = append(in.Lines, protocol.GoodsReceiptLine{POLineID: l.ID, VariantID: l.VariantID, QtyMilli: q, UnitCostMinor: l.UnitCostMinor})
		g.stock[0][l.VariantID] += int(q / 1000)
		delete(onOrder, l.VariantID)
	}
	g.setClock(d.Add(7*time.Hour + time.Duration(g.rng.IntN(40))*time.Minute))
	grn, err := h.ReceiveGoods(ctx, g.session("femi"), in)
	if err != nil {
		return err
	}
	_, err = h.CreateBill(ctx, g.session("chiamaka"), protocol.SupplierBillInput{SupplierID: o.SupplierID, GRNID: grn, SupplierRef: in.SupplierRef, BillDate: g.clock().UnixMilli()})
	return err
}

func (g *gen) payBills(d time.Time, by *hub.Session) error {
	ctx, h := g.ctx, g.h
	open, err := h.ListBills(ctx, hub.SystemSession, "", protocol.BillOpen)
	if err != nil {
		return err
	}
	bySupplier := map[string][]protocol.SupplierBill{}
	for _, b := range open {
		if b.DueDate <= d.AddDate(0, 0, 7).UnixMilli() && g.rng.Float64() < 0.9 {
			bySupplier[b.SupplierID] = append(bySupplier[b.SupplierID], b)
		}
	}
	keys := make([]string, 0, len(bySupplier))
	for k := range bySupplier {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, sid := range keys {
		var amt int64
		var billIDs []string
		for _, b := range bySupplier[sid] {
			amt += b.AmountMinor - b.PaidMinor
			billIDs = append(billIDs, b.ID)
		}
		g.setClock(d.Add(7*time.Hour + 45*time.Minute + time.Duration(g.rng.IntN(5))*time.Minute))
		if _, err := h.PaySupplier(ctx, by, protocol.SupplierPaymentInput{SupplierID: sid, Method: "Bank transfer", Reference: fmt.Sprintf("NIP%010d", g.rng.IntN(1e9)),
			AmountMinor: amt, PaidAt: g.clock().UnixMilli(), BillIDs: billIDs}); err != nil {
			return err
		}
	}
	return nil
}

func (g *gen) kioskRun(d time.Time) error {
	var lines []protocol.StockTransferLine
	for _, it := range g.items {
		if !it.kiosk || !it.created {
			continue
		}
		have, want := g.stock[1][it.variant], kioskTarget(it)
		if have > want/2 {
			continue
		}
		q := min(want-have, g.stock[0][it.variant]-2)
		if q <= 0 {
			continue
		}
		lines = append(lines, protocol.StockTransferLine{VariantID: it.variant, QtyMilli: int64(q) * 1000})
		g.stock[0][it.variant] -= q
	}
	if len(lines) == 0 {
		return nil
	}
	g.setClock(d.Add(6*time.Hour + 50*time.Minute))
	id, err := g.h.TransferStock(g.ctx, g.session("femi"), hub.TransferInput{FromLocationID: g.main, ToLocationID: g.kioskID, Note: "Kiosk top-up", Lines: lines})
	if err != nil {
		return err
	}
	g.stats.transfers++
	if err := g.waitSynced(); err != nil {
		return err
	}
	// The kiosk counts it in when the van arrives; now and then a copy is missing.
	receipt := protocol.TransferReceipt{}
	for _, l := range lines {
		got := l.QtyMilli
		if g.rng.Float64() < 0.03 && got > 1000 {
			got -= 1000
			receipt.Note = "One copy missing from the carton"
		}
		receipt.Lines = append(receipt.Lines, protocol.StockTransferLine{VariantID: l.VariantID, ReceivedQtyMilli: got})
		g.stock[1][l.VariantID] += int(got / 1000)
	}
	g.setClock(d.Add(7*time.Hour + 55*time.Minute))
	return g.tills[2].ReceiveTransfer(g.ctx, g.users["femi"], id, receipt)
}

func (g *gen) stockCount(d time.Time, by *hub.Session) error {
	ctx, h := g.ctx, g.h
	g.setClock(d.Add(5*time.Hour + 30*time.Minute))
	id, err := h.CreateStockCount(ctx, by, hub.StockCountInput{Name: "Quarterly count " + d.Format("Jan 2006"), LocationID: g.main})
	if err != nil {
		return err
	}
	for _, it := range g.items {
		if !it.created {
			continue
		}
		q := g.stock[0][it.variant]
		if q > 0 && g.rng.Float64() < 0.025 {
			q-- // shrinkage
			g.stock[0][it.variant] = q
		}
		if _, err := h.CountEntry(ctx, by, id, protocol.StockCountEntry{VariantID: it.variant, QtyMilli: int64(q) * 1000, Mode: "set"}); err != nil {
			return err
		}
	}
	g.setClock(d.Add(6 * time.Hour))
	g.stats.counts++
	return h.PostStockCount(ctx, by, id, protocol.StockCountPost{Note: "Quarterly count"})
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

var _ = sql.ErrNoRows
