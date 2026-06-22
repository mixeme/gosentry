package runner

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func CleanupLogs(logsDir string, maxFiles int, maxAgeDays int) error {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	type logFile struct {
		path    string
		modTime time.Time
	}
	var logs []logFile
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	for _, entry := range entries {
		// Only GoSentry run logs are managed here. Directories and non-.log files
		// are intentionally ignored so the user can keep notes or other artifacts
		// in the same folder without the cleanup policy deleting them.
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			continue
		}
		path := filepath.Join(logsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if maxAgeDays > 0 && info.ModTime().Before(cutoff) {
			// Cleanup is best-effort: failing to delete one file should not block
			// the scheduler from running future jobs.
			_ = os.Remove(path)
			continue
		}
		logs = append(logs, logFile{path: path, modTime: info.ModTime()})
	}

	if maxFiles <= 0 || len(logs) <= maxFiles {
		return nil
	}
	sort.Slice(logs, func(i int, j int) bool {
		// Newest files are kept first, then everything after maxFiles is removed.
		// This matches the user's expectation that the most recent failures and
		// command output remain available for investigation.
		return logs[i].modTime.After(logs[j].modTime)
	})
	for _, old := range logs[maxFiles:] {
		_ = os.Remove(old.path)
	}
	return nil
}
