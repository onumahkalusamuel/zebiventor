package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	err = db.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE counter (id INTEGER PRIMARY KEY, n INTEGER NOT NULL);
			INSERT INTO counter VALUES (1, 0);
			CREATE TABLE items (id INTEGER PRIMARY KEY AUTOINCREMENT, who INTEGER NOT NULL)`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// Many concurrent writers (think: many tills plus background sync) must all
// succeed with no SQLITE_BUSY and no lost updates.
func TestConcurrentWritersNoLostUpdates(t *testing.T) {
	db := openTest(t)
	const writers, perWriter = 50, 200

	var wg sync.WaitGroup
	errCh := make(chan error, writers*perWriter)
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWriter {
				err := db.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
					// read-modify-write: would lose updates without serialization
					var n int
					if err := tx.QueryRowContext(ctx, "SELECT n FROM counter WHERE id=1").Scan(&n); err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, "UPDATE counter SET n=? WHERE id=1", n+1); err != nil {
						return err
					}
					_, err := tx.ExecContext(ctx, "INSERT INTO items(who) VALUES (?)", w)
					return err
				})
				if err != nil {
					errCh <- err
				}
			}
		}()
	}

	// Readers hammer the DB concurrently; WAL means they never block writers.
	stop := make(chan struct{})
	var rwg sync.WaitGroup
	for range 8 {
		rwg.Add(1)
		go func() {
			defer rwg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				var n int
				if err := db.R().QueryRow("SELECT count(*) FROM items").Scan(&n); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(stop)
	rwg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	var n, items int
	db.R().QueryRow("SELECT n FROM counter").Scan(&n)
	db.R().QueryRow("SELECT count(*) FROM items").Scan(&items)
	if n != writers*perWriter || items != writers*perWriter {
		t.Fatalf("counter=%d items=%d, want %d", n, items, writers*perWriter)
	}
}

// A failing job inside a group-committed batch must not roll back its neighbours.
func TestFailingJobIsolated(t *testing.T) {
	db := openTest(t)
	boom := errors.New("boom")

	var wg sync.WaitGroup
	results := make([]error, 30)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = db.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
				if _, err := tx.Exec("INSERT INTO items(who) VALUES (?)", i); err != nil {
					return err
				}
				if i%3 == 0 {
					return boom // the insert above must be undone
				}
				return nil
			})
		}()
	}
	wg.Wait()

	want := 0
	for i, err := range results {
		if i%3 == 0 {
			if !errors.Is(err, boom) {
				t.Fatalf("job %d: got %v, want boom", i, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("job %d: %v", i, err)
		}
		want++
	}
	var got int
	db.R().QueryRow("SELECT count(*) FROM items").Scan(&got)
	if got != want {
		t.Fatalf("items=%d want %d", got, want)
	}
	var bad int
	db.R().QueryRow("SELECT count(*) FROM items WHERE who % 3 = 0").Scan(&bad)
	if bad != 0 {
		t.Fatalf("%d rows from failed jobs leaked", bad)
	}
}

func TestPanicIsContained(t *testing.T) {
	db := openTest(t)
	err := db.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error { panic("oops") })
	if err == nil {
		t.Fatal("expected error from panicking job")
	}
	if err := db.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO items(who) VALUES (1)")
		return err
	}); err != nil {
		t.Fatalf("writer unusable after panic: %v", err)
	}
}

func TestCancelledJobSkipped(t *testing.T) {
	db := openTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO items(who) VALUES (1)")
		return err
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestCloseDrainsQueue(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "c.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	db.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec("CREATE TABLE t (x)")
		return err
	})
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := db.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
				time.Sleep(10 * time.Microsecond)
				_, err := tx.Exec("INSERT INTO t VALUES (1)")
				return err
			})
			if err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			} else if !errors.Is(err, ErrClosed) {
				t.Error(err)
			}
		}()
	}
	time.Sleep(time.Millisecond)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if err := db.Write(context.Background(), func(context.Context, *sql.Tx) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatalf("write after close: %v", err)
	}
	t.Logf("%d writes committed before close", ok)
}

func TestBackup(t *testing.T) {
	db := openTest(t)
	dst := filepath.Join(t.TempDir(), "backup.db")
	if err := db.Backup(context.Background(), dst); err != nil {
		t.Fatal(err)
	}
	b, err := Open(dst, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var n int
	if err := b.R().QueryRow("SELECT n FROM counter").Scan(&n); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkQueuedWrites(b *testing.B) {
	db, _ := Open(filepath.Join(b.TempDir(), "b.db"), Options{})
	defer db.Close()
	db.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")
		return err
	})
	b.SetParallelism(8)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			db.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.Exec("INSERT INTO t(v) VALUES ('sale')")
				return err
			})
		}
	})
}
