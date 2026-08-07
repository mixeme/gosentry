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
	// DurationSumMS is the running total AvgDurationMS was computed from, folded
	// into JobRuntime.DurationSumMS so app.updateStats continues the same exact
	// sum instead of restarting from a value it would have to reverse-multiply.
	DurationSumMS int64
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

	byID := make(map[int][]logSummary)
	byName := make(map[string][]logSummary)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".log") {
			continue
		}
		summary := readLogSummary(filepath.Join(logsDir, name))
		summary.name = name
		if summary.hasJobID {
			byID[summary.jobID] = append(byID[summary.jobID], summary)
			continue
		}
		base := name[:len(name)-len(".log")]
		idx := strings.Index(base, "_")
		if idx < 0 {
			continue
		}
		byName[base[idx+1:]] = append(byName[base[idx+1:]], summary)
	}

	for _, job := range jobs {
		files := byID[job.ID]
		if len(files) == 0 {
			files = byName[sanitizeFileName(job.Name)]
		}
		if len(files) == 0 {
			continue
		}
		// The timestamp prefix sorts chronologically, so a lexical sort by file
		// name puts the oldest first; keep the newest maxFiles to honor the
		// retention bound.
		sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
		if maxFiles > 0 && len(files) > maxFiles {
			files = files[len(files)-maxFiles:]
		}
		result[job.ID] = aggregateLogStats(files)
	}
	return result
}

// aggregateLogStats folds the already-read header of each log file (oldest
// first) into one SeededStats. Files lacking a duration line contribute to the
// run/fail counts but not to the duration aggregates.
func aggregateLogStats(files []logSummary) SeededStats {
	var stats SeededStats
	var durationSum int64
	var durationCount int
	for _, file := range files {
		stats.RunCount++
		if file.state == "Failed" {
			stats.FailCount++
		}
		if file.hasDuration {
			// Files are oldest first, so the last assignment is the newest run.
			stats.LastDurationMS = file.durationMS
			if file.durationMS > stats.MaxDurationMS {
				stats.MaxDurationMS = file.durationMS
			}
			durationSum += file.durationMS
			durationCount++
		}
	}
	if durationCount > 0 {
		stats.TimedRunCount = durationCount
		stats.DurationSumMS = durationSum
		stats.AvgDurationMS = durationSum / int64(durationCount)
	}
	return stats
}

// logSummary is everything SeedStats needs from one run log: the file name it
// sorts by, which job wrote it, how the run ended, and how long it took.
type logSummary struct {
	name        string
	jobID       int
	hasJobID    bool
	state       string
	durationMS  int64
	hasDuration bool
}

// readLogSummary reads the job_id, state, and duration fields from a log file's
// header (the lines before the first blank line) in a single pass, so seeding
// opens each log once rather than once to find its job and again to read its
// result. The has* flags report whether a well-formed line was present,
// distinguishing a legacy log written before the field existed from one that
// genuinely recorded a zero value. An unreadable file yields a zero summary,
// which falls back to matching by the job name in the file name.
func readLogSummary(path string) logSummary {
	var summary logSummary
	file, err := os.Open(path)
	if err != nil {
		return summary
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break // end of header
		}
		if rest, ok := strings.CutPrefix(line, "job_id: "); ok {
			if id, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil {
				summary.jobID = id
				summary.hasJobID = true
			}
		} else if rest, ok := strings.CutPrefix(line, "state: "); ok {
			summary.state = strings.TrimSpace(rest)
		} else if rest, ok := strings.CutPrefix(line, "duration: "); ok {
			if value, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64); err == nil {
				summary.durationMS = value
				summary.hasDuration = true
			}
		}
	}
	return summary
}
