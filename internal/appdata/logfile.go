package appdata

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// MaxLogBytes is the size at which a log file is rotated to <name>.1.
const MaxLogBytes = 5 << 20

// LogFile is an append-only log that keeps the current file and one
// previous file, so logs never fill the disk.
type LogFile struct {
	mu   sync.Mutex
	path string
	f    *os.File
	size int64
}

// OpenLog opens (or creates) dir/logs/name.
func OpenLog(dir, name string) (*LogFile, error) {
	p := filepath.Join(dir, "logs", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	l := &LogFile{path: p}
	return l, l.open()
}

func (l *LogFile) open() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	st, _ := f.Stat()
	l.f, l.size = f, 0
	if st != nil {
		l.size = st.Size()
	}
	return nil
}

// Path is the current log file.
func (l *LogFile) Path() string { return l.path }

func (l *LogFile) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.size+int64(len(p)) > MaxLogBytes {
		l.f.Close()
		_ = os.Rename(l.path, l.path+".1")
		if err := l.open(); err != nil {
			return 0, err
		}
	}
	n, err := l.f.Write(p)
	l.size += int64(n)
	return n, err
}

func (l *LogFile) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}

// NewLogger logs to the file and, when also is non-nil, to also (e.g.
// stderr while developing). If the file cannot be opened it logs to also.
func NewLogger(dir, name string, also io.Writer) (*slog.Logger, *LogFile) {
	lf, err := OpenLog(dir, name)
	var w io.Writer = also
	switch {
	case err == nil && also != nil:
		w = io.MultiWriter(lf, also)
	case err == nil:
		w = lf
	case also == nil:
		w = io.Discard
	}
	log := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err != nil {
		log.Warn("cannot open log file", "err", err)
		return log, nil
	}
	return log, lf
}
