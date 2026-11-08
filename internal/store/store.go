// Package store wraps SQLite with the concurrency model the whole system relies on:
//
//   - exactly one writer connection, owned by a single goroutine;
//   - every write is submitted to a queue and executed by that goroutine;
//   - queued writes are group-committed: up to MaxBatch jobs share one
//     BEGIN IMMEDIATE transaction, each isolated in its own SAVEPOINT so a
//     failing job never affects the others;
//   - reads go through a separate pool of read-only connections and, thanks
//     to WAL, never block (and are never blocked by) the writer.
//
// The result is that any number of concurrent callers (HTTP handlers serving
// many tills, background sync, UI actions) can write at once without ever
// seeing SQLITE_BUSY, and throughput improves under load because fsyncs are
// shared across a batch.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ErrClosed is returned for writes submitted after Close.
var ErrClosed = errors.New("store: database is closed")

// TxFunc is a unit of work executed inside the writer transaction.
// It must only use tx (never the read pool) for queries that need to see
// its own writes.
type TxFunc func(ctx context.Context, tx *sql.Tx) error

// Options tunes the store. Zero values pick sensible defaults.
type Options struct {
	// MaxBatch is the maximum number of queued jobs folded into one transaction.
	MaxBatch int
	// QueueSize is the capacity of the write queue before callers block.
	QueueSize int
	// Readers is the size of the read connection pool.
	Readers int
	// BusyTimeout guards against external processes (backup tools, sqlite3 CLI).
	BusyTimeout time.Duration
}

func (o *Options) defaults() {
	if o.MaxBatch <= 0 {
		o.MaxBatch = 64
	}
	if o.QueueSize <= 0 {
		o.QueueSize = 1024
	}
	if o.Readers <= 0 {
		o.Readers = max(4, runtime.NumCPU())
	}
	if o.BusyTimeout <= 0 {
		o.BusyTimeout = 10 * time.Second
	}
}

// DB is a queued-writer SQLite database.
type DB struct {
	path   string
	opts   Options
	read   *sql.DB
	write  *sql.DB
	jobs   chan *job
	mu     sync.RWMutex
	closed bool
	loopWG sync.WaitGroup
}

type job struct {
	ctx    context.Context
	fn     TxFunc
	result chan error
}

func dsn(path string, readOnly bool, busy time.Duration) string {
	q := url.Values{}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busy.Milliseconds()))
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", "temp_store(MEMORY)")
	q.Add("_pragma", "cache_size(-16000)") // ~16MB page cache per connection
	if readOnly {
		q.Add("_pragma", "query_only(ON)")
	} else {
		q.Add("_txlock", "immediate")
	}
	return "file:" + path + "?" + q.Encode()
}

// Open opens (creating if needed) the database at path.
func Open(path string, opts Options) (*DB, error) {
	opts.defaults()

	w, err := sql.Open("sqlite", dsn(path, false, opts.BusyTimeout))
	if err != nil {
		return nil, err
	}
	w.SetMaxOpenConns(1)
	w.SetMaxIdleConns(1)
	w.SetConnMaxLifetime(0)
	w.SetConnMaxIdleTime(0)
	// Touch the writer first so the file and WAL exist before readers open.
	if err := w.Ping(); err != nil {
		w.Close()
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	var mode string
	if err := w.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		w.Close()
		return nil, fmt.Errorf("store: WAL mode not enabled (got %q): %v", mode, err)
	}

	r, err := sql.Open("sqlite", dsn(path, true, opts.BusyTimeout))
	if err != nil {
		w.Close()
		return nil, err
	}
	r.SetMaxOpenConns(opts.Readers)
	r.SetMaxIdleConns(opts.Readers)

	db := &DB{
		path:  path,
		opts:  opts,
		read:  r,
		write: w,
		jobs:  make(chan *job, opts.QueueSize),
	}
	db.loopWG.Add(1)
	go db.loop()
	return db, nil
}

// Path returns the database file path.
func (db *DB) Path() string { return db.path }

// R returns the read-only connection pool. Use it for all queries outside Write.
func (db *DB) R() *sql.DB { return db.read }

// Write enqueues fn and waits until the transaction containing it commits
// (nil) or fn / the commit fails (error). If fn returns an error, only fn's
// own changes are rolled back.
func (db *DB) Write(ctx context.Context, fn TxFunc) error {
	j := &job{ctx: ctx, fn: fn, result: make(chan error, 1)}

	db.mu.RLock()
	if db.closed {
		db.mu.RUnlock()
		return ErrClosed
	}
	select {
	case db.jobs <- j:
	case <-ctx.Done():
		db.mu.RUnlock()
		return ctx.Err()
	}
	db.mu.RUnlock()

	// Once queued, always wait for the outcome: reporting a timeout for a
	// write that later commits would be worse than waiting a few ms.
	return <-j.result
}

func (db *DB) loop() {
	defer db.loopWG.Done()
	batch := make([]*job, 0, db.opts.MaxBatch)
	for first := range db.jobs {
		batch = append(batch[:0], first)
	drain:
		for len(batch) < db.opts.MaxBatch {
			select {
			case j, ok := <-db.jobs:
				if !ok {
					break drain
				}
				batch = append(batch, j)
			default:
				break drain
			}
		}
		db.runBatch(batch)
	}
}

func (db *DB) runBatch(batch []*job) {
	errs := make([]error, len(batch))
	live := 0
	for i, j := range batch {
		if err := j.ctx.Err(); err != nil {
			errs[i] = err
			continue
		}
		live++
	}
	if live == 0 {
		deliver(batch, errs)
		return
	}

	tx, err := db.write.BeginTx(context.Background(), nil)
	if err != nil {
		fill(errs, fmt.Errorf("store: begin: %w", err))
		deliver(batch, errs)
		return
	}

	for i, j := range batch {
		if errs[i] != nil {
			continue
		}
		if _, err := tx.Exec("SAVEPOINT job"); err != nil {
			errs[i] = err
			continue
		}
		if err := safeCall(j, tx); err != nil {
			errs[i] = err
			if _, rbErr := tx.Exec("ROLLBACK TO job"); rbErr != nil {
				// The transaction is unusable; fail everything.
				_ = tx.Rollback()
				fill(errs, fmt.Errorf("store: rollback to savepoint: %w (job error: %v)", rbErr, err))
				deliver(batch, errs)
				return
			}
		}
		if _, err := tx.Exec("RELEASE job"); err != nil {
			_ = tx.Rollback()
			fill(errs, fmt.Errorf("store: release savepoint: %w", err))
			deliver(batch, errs)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		for i := range errs {
			if errs[i] == nil {
				errs[i] = fmt.Errorf("store: commit: %w", err)
			}
		}
	}
	deliver(batch, errs)
}

func safeCall(j *job, tx *sql.Tx) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("store: panic in write job: %v", r)
		}
	}()
	return j.fn(j.ctx, tx)
}

func fill(errs []error, err error) {
	for i := range errs {
		if errs[i] == nil {
			errs[i] = err
		}
	}
}

func deliver(batch []*job, errs []error) {
	for i, j := range batch {
		j.result <- errs[i]
	}
}

// Backup writes a consistent, compacted snapshot of the database to dst
// while the database stays fully usable.
func (db *DB) Backup(ctx context.Context, dst string) error {
	_, err := db.write.ExecContext(ctx, "VACUUM INTO ?", dst)
	return err
}

// Checkpoint folds the WAL back into the main database file.
func (db *DB) Checkpoint(ctx context.Context) error {
	_, err := db.write.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

// Close drains the write queue, then closes all connections.
func (db *DB) Close() error {
	db.mu.Lock()
	if db.closed {
		db.mu.Unlock()
		return nil
	}
	db.closed = true
	close(db.jobs)
	db.mu.Unlock()

	db.loopWG.Wait()
	_, _ = db.write.Exec("PRAGMA optimize")
	return errors.Join(db.read.Close(), db.write.Close())
}
