package app

import (
	"fmt"
	"os"
	"sync"
)

// Keeps at most four 10 MiB JSON log segments. Collector follows file fingerprints.
type rotatingLog struct {
	mu   sync.Mutex
	path string
	file *os.File
	size int64
}

func newRotatingLog(path string) (*rotatingLog, error) {
	w := &rotatingLog{path: path}
	err := w.open()
	return w, err
}
func (w *rotatingLog) open() error {
	f, e := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if e != nil {
		return e
	}
	stat, e := f.Stat()
	if e != nil {
		f.Close()
		return e
	}
	w.file = f
	w.size = stat.Size()
	return nil
}
func (w *rotatingLog) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size+int64(len(p)) > 10<<20 {
		if e := w.file.Close(); e != nil {
			return 0, e
		}
		for n := 2; n >= 0; n-- {
			source := w.path
			if n > 0 {
				source = fmt.Sprintf("%s.%d", w.path, n)
			}
			_ = os.Rename(source, fmt.Sprintf("%s.%d", w.path, n+1))
		}
		if e := w.open(); e != nil {
			return 0, e
		}
	}
	n, e := w.file.Write(p)
	w.size += int64(n)
	return n, e
}
