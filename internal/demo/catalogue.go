// Package demo builds a realistic demo store (TMIT Bookstore): a catalogue
// of real books from Open Library, their covers, and years of trading
// generated through the app's own till and hub code.
package demo

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Book is one real title, with facts from Open Library.
type Book struct {
	Title     string `json:"title"`
	Author    string `json:"author"`
	ISBN      string `json:"isbn"` // ISBN-13
	Publisher string `json:"publisher,omitempty"`
	Year      int    `json:"year,omitempty"`
	Pages     int    `json:"pages,omitempty"`
	CoverID   int    `json:"cover_id,omitempty"` // covers.openlibrary.org/b/id/<id>-M.jpg
	Category  string `json:"category"`
	Rank      int    `json:"rank"` // popularity within its category, 0 = most read
}

// Shelf is one Open Library subject search feeding a category.
type Shelf struct {
	Category string
	Subject  string
	Count    int
}

// Shelves is what TMIT Bookstore stocks.
var Shelves = []Shelf{
	{"African Literature", "nigerian fiction", 70},
	{"African Literature", "african fiction", 40},
	{"African Literature", "ghanaian fiction", 15},
	{"African Literature", "kenyan fiction", 15},
	{"Fiction", "fiction", 110},
	{"Fiction", "classic literature", 40},
	{"Fiction", "detective and mystery stories", 30},
	{"Fiction", "love stories", 25},
	{"Fiction", "fantasy fiction", 30},
	{"Young Adult", "young adult fiction", 50},
	{"Children", "picture books", 45},
	{"Children", "children's fiction", 60},
	{"Children", "children's stories", 25},
	{"Non-fiction", "biography", 40},
	{"Non-fiction", "history", 35},
	{"Non-fiction", "africa, history", 20},
	{"Non-fiction", "politics and government", 15},
	{"Business & Self-help", "self-help techniques", 35},
	{"Business & Self-help", "business", 35},
	{"Business & Self-help", "personal finance", 20},
	{"Business & Self-help", "leadership", 20},
	{"Religion", "christian life", 35},
	{"Religion", "islam", 15},
	{"Religion", "bible", 10},
	{"Textbooks & Exam Prep", "mathematics, textbooks", 20},
	{"Textbooks & Exam Prep", "chemistry, textbooks", 12},
	{"Textbooks & Exam Prep", "physics, textbooks", 12},
	{"Textbooks & Exam Prep", "biology, textbooks", 12},
	{"Textbooks & Exam Prep", "english language, grammar", 15},
	{"Textbooks & Exam Prep", "economics, textbooks", 10},
	{"Cookbooks", "cooking", 30},
	{"Poetry & Drama", "poetry", 25},
	{"Poetry & Drama", "drama", 20},
}

//go:embed books.json
var booksJSON []byte

// Books is the saved catalogue (internal/demo/books.json).
func Books() ([]Book, error) {
	var b []Book
	return b, json.Unmarshal(booksJSON, &b)
}

// validISBN13 checks an ISBN-13 checksum.
func validISBN13(s string) bool {
	if len(s) != 13 || (!strings.HasPrefix(s, "978") && !strings.HasPrefix(s, "979")) {
		return false
	}
	sum := 0
	for i, c := range s {
		if c < '0' || c > '9' {
			return false
		}
		d := int(c - '0')
		if i%2 == 1 {
			d *= 3
		}
		sum += d
	}
	return sum%10 == 0
}

// pickISBN prefers an English-language edition (978-0 / 978-1).
func pickISBN(list []string) string {
	best := ""
	for _, s := range list {
		if !validISBN13(s) {
			continue
		}
		if strings.HasPrefix(s, "9780") || strings.HasPrefix(s, "9781") {
			return s
		}
		if best == "" {
			best = s
		}
	}
	return best
}

const userAgent = "Zebiventor-demo/0.5 (builds a demo bookshop catalogue)"

// FetchCatalogue asks Open Library for each shelf's most-read titles.
func FetchCatalogue(ctx context.Context, logf func(string, ...any)) ([]Book, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	seen := map[string]bool{}
	seenTitle := map[string]bool{}
	var out []Book
	for _, sh := range Shelves {
		q := url.Values{}
		q.Set("q", fmt.Sprintf("subject:%q language:eng", sh.Subject))
		q.Set("fields", "key,title,author_name,isbn,publisher,first_publish_year,cover_i,number_of_pages_median")
		q.Set("limit", fmt.Sprint(sh.Count*3))
		q.Set("sort", "readinglog")
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://openlibrary.org/search.json?"+q.Encode(), nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		var res struct {
			Docs []struct {
				Title     string   `json:"title"`
				Authors   []string `json:"author_name"`
				ISBN      []string `json:"isbn"`
				Publisher []string `json:"publisher"`
				Year      int      `json:"first_publish_year"`
				Cover     int      `json:"cover_i"`
				Pages     int      `json:"number_of_pages_median"`
			} `json:"docs"`
		}
		err = json.NewDecoder(resp.Body).Decode(&res)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sh.Subject, err)
		}
		got := 0
		for _, d := range res.Docs {
			if got >= sh.Count {
				break
			}
			isbn := pickISBN(d.ISBN)
			title := strings.TrimSpace(d.Title)
			key := strings.ToLower(title)
			if isbn == "" || len(d.Authors) == 0 || title == "" || seen[isbn] || seenTitle[key] || len(title) > 90 {
				continue
			}
			seen[isbn], seenTitle[key] = true, true
			b := Book{Title: titleCase(title), Author: d.Authors[0], ISBN: isbn, Year: d.Year, Pages: d.Pages, CoverID: d.Cover, Category: sh.Category}
			for _, p := range d.Publisher { // first plausible imprint
				if len(p) <= 40 && !strings.ContainsAny(p, "[]") {
					b.Publisher = p
					break
				}
			}
			out = append(out, b)
			got++
		}
		logf("%-22s %-32s %d books", sh.Category, sh.Subject, got)
		time.Sleep(700 * time.Millisecond) // be polite
	}
	// Popularity rank within each category, in the order Open Library gave.
	rank := map[string]int{}
	for i := range out {
		out[i].Rank = rank[out[i].Category]
		rank[out[i].Category]++
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Category < out[j].Category })
	return out, nil
}

var smallWords = map[string]bool{"a": true, "an": true, "and": true, "as": true, "at": true, "but": true, "by": true, "for": true, "from": true,
	"in": true, "into": true, "nor": true, "of": true, "on": true, "or": true, "the": true, "to": true, "with": true}

// titleCase capitalises a title that came in lower case ("The concubine").
func titleCase(t string) string {
	words := strings.Fields(t)
	for i, w := range words {
		if w == strings.ToLower(w) && (i == 0 || !smallWords[w]) {
			r := []rune(w)
			if len(r) > 0 && r[0] >= 'a' && r[0] <= 'z' {
				r[0] -= 'a' - 'A'
			}
			words[i] = string(r)
		}
	}
	return strings.Join(words, " ")
}
