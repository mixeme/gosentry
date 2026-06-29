package runner

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

// SeededStats are the aggregate execution-time statistics reconstructed from a
// job's existing log files at startup. The fields mirror the run-time counters
// on domain.JobRuntime so the caller can fold them in directly.
type SeededStats struct {
	RunCount       int
	FailCount      int
	LastDurationMS int64
	AvgDurationMS  int64
	MaxDurationMS  int64
	TimedRunCount  int
}

// SeedStats scans logsDir once and reconstructs per-job execution-time
// statistics from the log files written by previous runs, keyed by Job.ID.
//
// Log files are matched primarily by the job_id header line writeRunLog writes.
// When that header is absent (legacy logs), files fall back to the sanitized
// job-name suffix in the filename. For each job only the newest maxFiles
// matching logs are parsed, mirroring the retention policy that CleanupLogs
// enforces; a maxFiles of zero or less means "no bound". The duration and state
// are read from each log's header. Logs written before duration tracking existed
// carry no duration line: those are tolerated — they still count toward RunCount
// and FailCount but are left out of the duration aggregates (last/avg/max) so a
// missing duration cannot masquerade as a zero-millisecond run.
//
// A missing or unreadable logs directory yields an empty map rather than an
// error: seeding is best-effort and must never block startup.
func SeedStats(logsDir string, jobs []domain.Job, maxFiles int) map[int]SeededStats {
	result := make(map[int]SeededStats, len(jobs))
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return result
	}

	byID := make(map[int][]string)
	byName := make(map[string][]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".log") {
			continue
		}
		path := filepath.Join(logsDir, name)
		if jobID, ok := readLogJobID(path); ok {
			byID[jobID] = append(byID[jobID], name)
			continue
		}
		base := name[:len(name)-len(".log")]
		idx := strings.Index(base, "_")
		if idx < 0 {
			continue
		}
		byName[base[idx+1:]] = append(byName[base[idx+1:]], name)
	}

	for _, job := range jobs {
		files := byID[job.ID]
		if len(files) == 0 {
			files = byName[sanitizeFileName(job.Name)]
		}
		if len(files) == 0 {
			continue
		}
		// The timestamp prefix sorts chronologically, so a lexical sort puts the
		// oldest first; keep the newest maxFiles to honor the retention bound.
		sort.Strings(files)
		if maxFiles > 0 && len(files) > maxFiles {
			files = files[len(files)-maxFiles:]
		}
		result[job.ID] = aggregateLogStats(logsDir, files)
	}
	return result
}

// aggregateLogStats folds the header of each log file (oldest first) into one
// SeededStats. Files lacking a duration line contribute to the run/fail counts
// but not to the duration aggregates.
func aggregateLogStats(logsDir string, files []string) SeededStats {
	var stats SeededStats
	var durationSum int64
	var durationCount int
	for _, file := range files {
		state, durationMS, hasDuration := readLogHeader(filepath.Join(logsDir, file))
		stats.RunCount++
		if state == "Failed" {
			stats.FailCount++
		}
		if hasDuration {
			// Files are oldest first, so the last assignment is the newest run.
			stats.LastDurationMS = durationMS
			if durationMS > stats.MaxDurationMS {
				stats.MaxDurationMS = durationMS
			}
			durationSum += durationMS
			durationCount++
		}
	}
	if durationCount > 0 {
		stats.TimedRunCount = durationCount
		stats.AvgDurationMS = durationSum / int64(durationCount)
	}
	return stats
}

// readLogJobID reads the job_id field from a log file header.
func readLogJobID(path string) (int, bool) {
	file, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		if rest, ok := strings.CutPrefix(line, "job_id: "); ok {
			id, err := strconv.Atoi(strings.TrimSpace(rest))
			if err != nil {
				return 0, false
			}
			return id, true
		}
	}
	return 0, false
}

// readLogHeader reads the "state" and "duration" fields from a log file's
// header (the lines before the first blank line). hasDuration reports whether a
// well-formed duration line was present, distinguishing a legacy duration-less
// log from one that genuinely recorded a zero-millisecond run.
func readLogHeader(path string) (state string, durationMS int64, hasDuration bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break // end of header
		}
		if rest, ok := strings.CutPrefix(line, "state: "); ok {
			state = strings.TrimSpace(rest)
		} else if rest, ok := strings.CutPrefix(line, "duration: "); ok {
			if value, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64); err == nil {
				durationMS = value
				hasDuration = true
			}
		}
	}
	return state, durationMS, hasDuration
}
