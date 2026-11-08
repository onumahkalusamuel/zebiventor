// Package ids generates sortable, globally unique identifiers (ULIDs).
// They can be created offline on any till without coordination and sort by
// creation time, which keeps SQLite B-tree inserts append-mostly.
package ids

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

// New returns a new ULID string.
func New() string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

// NowMs returns the current time as unix milliseconds, the timestamp format
// used throughout the database.
func NowMs() int64 { return time.Now().UnixMilli() }
