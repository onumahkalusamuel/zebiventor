// Package ids generates sortable, globally unique identifiers (ULIDs).
// They can be created offline on any till without coordination and sort by
// creation time, which keeps SQLite B-tree inserts append-mostly.
package ids

import (
	"crypto/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

var clock atomic.Pointer[func() time.Time]

// SetClock replaces the time source (nil restores the real clock). Only the
// demo-data generator uses it, to record years of history at their dates.
func SetClock(f func() time.Time) {
	if f == nil {
		clock.Store(nil)
		return
	}
	clock.Store(&f)
}

// Now is the current time (from SetClock's source when one is set).
func Now() time.Time {
	if f := clock.Load(); f != nil {
		return (*f)()
	}
	return time.Now()
}

// New returns a new ULID string.
func New() string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(Now()), entropy).String()
}

// NowMs returns the current time as unix milliseconds, the timestamp format
// used throughout the database.
func NowMs() int64 { return Now().UnixMilli() }
