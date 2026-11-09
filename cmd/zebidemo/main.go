// Command zebidemo builds the TMIT Bookstore demo store.
//
//	zebidemo catalogue                  fetch real books from Open Library into internal/demo/books.json
//	zebidemo covers   [-cache DIR]      download their covers (once; cached)
//	zebidemo generate -out FILE [...]   simulate years of trading; writes a store data file
//
// Open the data file on any PC: first-run setup → "Restore a store from a
// data file" (or Settings → Backups → Restore on an existing hub).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/demo"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: zebidemo catalogue | covers | generate")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	logf := func(f string, a ...any) { log.Printf(f, a...) }
	home, _ := os.UserCacheDir()
	defCache := filepath.Join(home, "zebidemo", "covers")
	var err error
	switch os.Args[1] {
	case "catalogue":
		fs := flag.NewFlagSet("catalogue", flag.ExitOnError)
		out := fs.String("out", "internal/demo/books.json", "where to write the catalogue")
		fs.Parse(os.Args[2:])
		var books []demo.Book
		if books, err = demo.FetchCatalogue(ctx, logf); err == nil {
			b, _ := json.MarshalIndent(books, "", " ")
			err = os.WriteFile(*out, b, 0o644)
			logf("%d books written to %s", len(books), *out)
		}
	case "covers":
		fs := flag.NewFlagSet("covers", flag.ExitOnError)
		cache := fs.String("cache", defCache, "cover cache folder")
		fs.Parse(os.Args[2:])
		err = demo.FetchCovers(ctx, *cache, logf)
	case "generate":
		fs := flag.NewFlagSet("generate", flag.ExitOnError)
		var cfg demo.Config
		fs.StringVar(&cfg.Out, "out", "TMIT Bookstore.zebi.db", "store data file to write")
		fs.StringVar(&cfg.CoverCache, "cache", defCache, "cover cache folder (from zebidemo covers)")
		from := fs.String("from", "2023-01-02", "first trading day")
		to := fs.String("to", time.Now().AddDate(0, 0, -1).Format("2006-01-02"), "last trading day")
		fs.Float64Var(&cfg.Scale, "scale", 1, "multiply daily sales (0.1 for a quick run)")
		fs.Int64Var(&cfg.Seed, "seed", 2023, "random seed (same seed, same store)")
		fs.Parse(os.Args[2:])
		if cfg.From, err = time.ParseInLocation("2006-01-02", *from, time.Local); err == nil {
			if cfg.To, err = time.ParseInLocation("2006-01-02", *to, time.Local); err == nil {
				err = demo.Generate(ctx, cfg, logf)
			}
		}
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}
