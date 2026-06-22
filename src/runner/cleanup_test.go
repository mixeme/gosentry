package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeLogFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func setModTime(t *testing.T, path string, age time.Duration) {
	t.Helper()
	mt := time.Now().Add(-age)
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupLogsMissingDirReturnsNil(t *testing.T) {
	err := CleanupLogs(filepath.Join(t.TempDir(), "nonexistent"), 100, 30)
	if err != nil {
		t.Errorf("missing dir should return nil, got %v", err)
	}
}

func TestCleanupLogsRemovesFilesPastMaxAge(t *testing.T) {
	dir := t.TempDir()
	old := writeLogFile(t, dir, "old.log")
	recent := writeLogFile(t, dir, "recent.log")
	setModTime(t, old, 31*24*time.Hour)   // 31 days old → past the 30-day limit
	setModTime(t, recent, 5*24*time.Hour) // 5 days old → within limit

	if err := CleanupLogs(dir, 100, 30); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("file older than maxAgeDays should be deleted")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Errorf("file within maxAgeDays should be kept: %v", err)
	}
}

func TestCleanupLogsKeepsFilesWithinAgeLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 3; i++ {
		path := writeLogFile(t, dir, fmt.Sprintf("job_%d.log", i))
		setModTime(t, path, time.Duration(i)*24*time.Hour)
	}

	if err := CleanupLogs(dir, 100, 30); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Errorf("expected 3 files kept within age limit, got %d", len(entries))
	}
}

// TestCleanupLogsByCountDeletesOldest verifies the count-based policy: when more
// than maxFiles log files exist the oldest (by modification time) are removed.
// maxAgeDays=0 disables age-based cleanup so the test exercises count only.
func TestCleanupLogsByCountDeletesOldest(t *testing.T) {
	dir := t.TempDir()
	// Create 5 files; i=0 is newest (1 day old), i=4 is oldest (5 days old).
	var paths []string
	for i := 0; i < 5; i++ {
		path := writeLogFile(t, dir, fmt.Sprintf("job_%03d.log", i))
		setModTime(t, path, time.Duration(i+1)*24*time.Hour)
		paths = append(paths, path)
	}

	if err := CleanupLogs(dir, 3, 0); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Errorf("expected 3 files after count cleanup, got %d", len(entries))
	}
	// The 3 newest files (paths[0..2]) must survive.
	for _, kept := range paths[:3] {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("newest file %s should be kept: %v", filepath.Base(kept), err)
		}
	}
	// The 2 oldest files (paths[3..4]) must be removed.
	for _, deleted := range paths[3:] {
		if _, err := os.Stat(deleted); !os.IsNotExist(err) {
			t.Errorf("oldest file %s should have been deleted", filepath.Base(deleted))
		}
	}
}

func TestCleanupLogsNonLogFilesNotDeleted(t *testing.T) {
	dir := t.TempDir()
	logFile := writeLogFile(t, dir, "job.log")
	notALog := writeLogFile(t, dir, "notes.txt")
	// Both are old enough that age-based cleanup would remove them if it applied.
	setModTime(t, logFile, 35*24*time.Hour)
	setModTime(t, notALog, 35*24*time.Hour)

	if err := CleanupLogs(dir, 100, 30); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(logFile); !os.IsNotExist(err) {
		t.Error("old .log file should be deleted")
	}
	if _, err := os.Stat(notALog); err != nil {
		t.Errorf(".txt file should not be deleted: %v", err)
	}
}

func TestCleanupLogsSubdirsNotDeleted(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "archive.log") // name looks like a log but is a dir
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	setModTime(t, subdir, 60*24*time.Hour)

	if err := CleanupLogs(dir, 100, 30); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(subdir); err != nil {
		t.Errorf("subdirectory should not be deleted: %v", err)
	}
}

// TestCleanupLogsZeroLimitsDisableBothPolicies confirms that maxFiles=0 disables
// count-based cleanup and maxAgeDays=0 disables age-based cleanup independently.
func TestCleanupLogsZeroLimitsDisableBothPolicies(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		path := writeLogFile(t, dir, fmt.Sprintf("job_%d.log", i))
		setModTime(t, path, 60*24*time.Hour) // very old
	}

	if err := CleanupLogs(dir, 0, 0); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 5 {
		t.Errorf("expected all 5 files kept with both limits disabled, got %d", len(entries))
	}
}
