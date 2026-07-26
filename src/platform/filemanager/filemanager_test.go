package filemanager

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The success path is deliberately not tested: it would pop a real file
// manager window on the machine running the suite. Only the guards that keep
// Open from launching anything are exercised here.

func TestOpenRejectsMissingFolder(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-folder")

	err := Open(missing)
	if err == nil {
		t.Fatal("Open on a missing folder returned nil, want an error")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not name the missing folder %q", err, missing)
	}
}

func TestOpenRejectsFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "gosentry.log")
	if err := os.WriteFile(file, []byte("log"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	err := Open(file)
	if err == nil {
		t.Fatal("Open on a file returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "not a folder") {
		t.Errorf("error %q does not report that the path is not a folder", err)
	}
}

// TestOpenCommandNamesPlatformHandler checks the supported platforms name a
// handler (an empty name makes Open report the action as unavailable) and that
// the directory is passed as a single argument, so spaces need no quoting.
func TestOpenCommandNamesPlatformHandler(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "log files")

	name, args := openCommand(dir)
	switch runtime.GOOS {
	case "windows":
		if name != "explorer" {
			t.Errorf("handler on windows = %q, want %q", name, "explorer")
		}
	case "linux":
		if name != "xdg-open" {
			t.Errorf("handler on linux = %q, want %q", name, "xdg-open")
		}
	default:
		if name != "" {
			t.Errorf("handler on %s = %q, want no handler", runtime.GOOS, name)
		}
		return
	}
	if len(args) != 1 || args[0] != filepath.Clean(dir) {
		t.Errorf("arguments = %q, want the single path %q", args, filepath.Clean(dir))
	}
}
