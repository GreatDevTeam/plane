// Package logging writes plane-cli's own warnings and errors to a small set of rotating local
// log files. The TUI takes over the whole terminal (tea.WithAltScreen) and its footer only ever
// shows the latest error, so this is the only place a problem survives past the next status
// line or a session that never reports it in the UI at all (see fetchBoardExtras, whose
// labels/members fetch has always failed silently).
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/makeplane/plane/apps/cli-go/internal/config"
)

// maxFileSize and maxFiles are vars rather than consts so tests can shrink them to keep rotation
// fast to exercise; production always runs with these defaults. logFileName.N is the rotation
// naming: logFileName is always the active file, .1 the most recently rotated, up to
// maxFiles-1 — one more rotation past that discards the oldest.
var (
	maxFileSize int64 = 10 * 1024 * 1024
	maxFiles          = 10
)

const logFileName = "plane-cli.log"

var (
	mu   sync.Mutex
	file *os.File
	size int64
)

// Init opens (creating if needed) the log directory and the active log file. Safe to call more
// than once — only the first call after process start (or after Close) does anything. A failure
// here is not fatal to the caller: logging is diagnostic only, so plane-cli runs without it
// rather than refusing to start.
func Init() error {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		return nil
	}
	dir, err := logDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, info, err := openActive(dir)
	if err != nil {
		return err
	}
	file = f
	size = info.Size()
	return nil
}

// Close flushes and closes the active log file. Safe to call when Init was never called, or
// more than once.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
		file = nil
	}
}

// Warn and Error each append one line to the active log file, rotating first if the line would
// push it past maxFileSize. Both are no-ops if Init was never called or failed — nothing else in
// plane-cli writes to disk on its behalf, and callers must not treat a logging failure as
// reason to change their own behavior.
func Warn(format string, args ...any)  { write("WARN", format, args...) }
func Error(format string, args ...any) { write("ERROR", format, args...) }

func write(level, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return
	}
	line := fmt.Sprintf("%s [%s] %s\n", time.Now().Format(time.RFC3339), level, fmt.Sprintf(format, args...))
	if size+int64(len(line)) > maxFileSize {
		if err := rotateLocked(); err != nil {
			return
		}
	}
	n, err := file.WriteString(line)
	if err == nil {
		size += int64(n)
	}
}

// rotateLocked shifts the active file to .1, .1 to .2, and so on up to maxFiles-1, discarding
// whatever already occupied the last slot, then opens a fresh active file. Called with mu held.
func rotateLocked() error {
	dir, err := logDir()
	if err != nil {
		return err
	}
	if file != nil {
		file.Close()
		file = nil
	}
	os.Remove(numberedPath(dir, maxFiles-1))
	for i := maxFiles - 2; i >= 1; i-- {
		os.Rename(numberedPath(dir, i), numberedPath(dir, i+1))
	}
	if err := os.Rename(filepath.Join(dir, logFileName), numberedPath(dir, 1)); err != nil && !os.IsNotExist(err) {
		return err
	}
	f, info, err := openActive(dir)
	if err != nil {
		return err
	}
	file = f
	size = info.Size()
	return nil
}

func openActive(dir string) (*os.File, os.FileInfo, error) {
	f, err := os.OpenFile(filepath.Join(dir, logFileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, info, nil
}

func numberedPath(dir string, n int) string {
	return filepath.Join(dir, fmt.Sprintf("%s.%d", logFileName, n))
}

// logDir is ~/.config/plane-cli/logs (honoring $XDG_CONFIG_HOME, like config.Path).
func logDir() (string, error) {
	cfgPath, err := config.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfgPath), "logs"), nil
}
