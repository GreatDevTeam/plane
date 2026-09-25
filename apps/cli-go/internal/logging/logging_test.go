package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempLogDir points config.Path (and so logDir) at a fresh temp directory, and makes sure
// no file handle from a previous test leaks into this one.
func withTempLogDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	Close()
	t.Cleanup(Close)
	return filepath.Join(dir, "plane-cli", "logs")
}

func TestWarnAndErrorWriteToLogFile(t *testing.T) {
	logDirPath := withTempLogDir(t)
	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	Warn("something odd: %d", 42)
	Error("request failed: %s", "timeout")

	data, err := os.ReadFile(filepath.Join(logDirPath, logFileName))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "[WARN] something odd: 42") {
		t.Errorf("log missing WARN line, got:\n%s", data)
	}
	if !strings.Contains(string(data), "[ERROR] request failed: timeout") {
		t.Errorf("log missing ERROR line, got:\n%s", data)
	}
}

func TestWriteBeforeInitIsANoop(t *testing.T) {
	withTempLogDir(t)
	// Init deliberately not called: Warn/Error must not panic or create anything.
	Warn("dropped")
	Error("also dropped")
}

func TestRotationKeepsOldContentInNumberedFile(t *testing.T) {
	logDirPath := withTempLogDir(t)
	origSize := maxFileSize
	maxFileSize = 1024 // shrink so one write forces a rotation
	t.Cleanup(func() { maxFileSize = origSize })

	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	Warn("%s", strings.Repeat("a", 900))
	Warn("%s", strings.Repeat("b", 900)) // pushes the active file past maxFileSize, rotating it

	rotated, err := os.ReadFile(filepath.Join(logDirPath, logFileName+".1"))
	if err != nil {
		t.Fatalf("read rotated file: %v", err)
	}
	if !strings.Contains(string(rotated), strings.Repeat("a", 900)) {
		t.Error("rotated file does not hold the pre-rotation content")
	}
	active, err := os.ReadFile(filepath.Join(logDirPath, logFileName))
	if err != nil {
		t.Fatalf("read active file: %v", err)
	}
	if !strings.Contains(string(active), strings.Repeat("b", 900)) {
		t.Error("active file does not hold the post-rotation line")
	}
}

func TestFileCountNeverExceedsMax(t *testing.T) {
	logDirPath := withTempLogDir(t)
	origSize, origFiles := maxFileSize, maxFiles
	maxFileSize = 200
	maxFiles = 4
	t.Cleanup(func() { maxFileSize, maxFiles = origSize, origFiles })

	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// Each line is ~180 bytes, well past maxFileSize on its own, so every call rotates —
	// far more rotations than maxFiles allows, to exercise the oldest-file eviction.
	for i := 0; i < 20; i++ {
		Warn("%s", strings.Repeat("x", 180))
	}

	entries, err := os.ReadDir(logDirPath)
	if err != nil {
		t.Fatalf("read log dir: %v", err)
	}
	if len(entries) > maxFiles {
		t.Errorf("found %d log files, want at most maxFiles (%d)", len(entries), maxFiles)
	}
	beyondMax := filepath.Join(logDirPath, logFileName+".4") // maxFiles is 4: .1/.2/.3 + active
	if _, err := os.Stat(beyondMax); err == nil {
		t.Errorf("a file beyond maxFiles-1 rotations survived: %s", beyondMax)
	}
}
