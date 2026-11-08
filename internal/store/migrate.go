package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

// Migrate applies numbered SQL files (e.g. "0001_init.sql") from fsys/dir
// whose number is greater than PRAGMA user_version. Each file runs in its
// own transaction together with the user_version bump.
func (db *DB) Migrate(ctx context.Context, fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	type mig struct {
		version int
		name    string
	}
	var migs []mig
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		num, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			return fmt.Errorf("migrate: bad file name %q", e.Name())
		}
		v, err := strconv.Atoi(num)
		if err != nil {
			return fmt.Errorf("migrate: bad version in %q: %w", e.Name(), err)
		}
		migs = append(migs, mig{v, e.Name()})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })

	var current int
	if err := db.R().QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	for _, m := range migs {
		if m.version <= current {
			continue
		}
		body, err := fs.ReadFile(fsys, dir+"/"+m.name)
		if err != nil {
			return err
		}
		err = db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(body)); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.version))
			return err
		})
		if err != nil {
			return fmt.Errorf("migrate %s: %w", m.name, err)
		}
	}
	return nil
}
