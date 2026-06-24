package runner

import (
	"os"
	"path/filepath"
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

// writeTestLog writes a minimal log file in the format writeRunLog produces.
// Pass durationMS < 0 to omit the duration line (legacy log simulation).
func writeTestLog(t *testing.T, dir, filename, state string, durationMS int64) {
	t.Helper()
	var content string
	if durationMS >= 0 {
		content = "state: " + state + "\nduration: " + itoa(durationMS) + "\n\n"
	} else {
		content = "state: " + state + "\n\n"
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

func TestSeedStatsBasic(t *testing.T) {
	dir := t.TempDir()
	job := domain.Job{ID: 1, Name: "Build"}
	name := sanitizeFileName(job.Name)

	writeTestLog(t, dir, "20260601-100000_"+name+".log", "OK", 200)
	writeTestLog(t, dir, "20260601-110000_"+name+".log", "Failed", 400)
	writeTestLog(t, dir, "20260601-120000_"+name+".log", "OK", 600)

	result := SeedStats(dir, []domain.Job{job}, 0)
	s, ok := result[job.ID]
	if !ok {
		t.Fatal("expected stats for job 1")
	}
	if s.RunCount != 3 {
		t.Errorf("RunCount = %d, want 3", s.RunCount)
	}
	if s.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", s.FailCount)
	}
	if s.LastDurationMS != 600 {
		t.Errorf("LastDurationMS = %d, want 600", s.LastDurationMS)
	}
	if s.MaxDurationMS != 600 {
		t.Errorf("MaxDurationMS = %d, want 600", s.MaxDurationMS)
	}
	// avg = (200+400+600)/3 = 400
	if s.AvgDurationMS != 400 {
		t.Errorf("AvgDurationMS = %d, want 400", s.AvgDurationMS)
	}
}

// TestSeedStatsDurationLessLegacyLog verifies that a log without a duration
// line still contributes to RunCount/FailCount but is excluded from duration
// aggregates, so a missing duration cannot masquerade as a 0 ms run.
func TestSeedStatsDurationLessLegacyLog(t *testing.T) {
	dir := t.TempDir()
	job := domain.Job{ID: 2, Name: "Deploy"}
	name := sanitizeFileName(job.Name)

	// Legacy log (no duration line).
	writeTestLog(t, dir, "20260601-080000_"+name+".log", "OK", -1)
	// Modern log with duration.
	writeTestLog(t, dir, "20260601-090000_"+name+".log", "OK", 300)

	result := SeedStats(dir, []domain.Job{job}, 0)
	s := result[job.ID]

	if s.RunCount != 2 {
		t.Errorf("RunCount = %d, want 2", s.RunCount)
	}
	if s.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", s.FailCount)
	}
	// Only the modern log has a duration — avg/last/max must reflect that single entry.
	if s.LastDurationMS != 300 {
		t.Errorf("LastDurationMS = %d, want 300", s.LastDurationMS)
	}
	if s.AvgDurationMS != 300 {
		t.Errorf("AvgDurationMS = %d, want 300", s.AvgDurationMS)
	}
}

// TestSeedStatsMaxFilesHonoured verifies that only the newest N logs are
// parsed when maxFiles is positive.
func TestSeedStatsMaxFilesHonoured(t *testing.T) {
	dir := t.TempDir()
	job := domain.Job{ID: 3, Name: "Cleanup"}
	name := sanitizeFileName(job.Name)

	// Write 3 logs; only the 2 newest should be counted (maxFiles=2).
	writeTestLog(t, dir, "20260601-060000_"+name+".log", "OK", 100)
	writeTestLog(t, dir, "20260601-070000_"+name+".log", "OK", 200)
	writeTestLog(t, dir, "20260601-080000_"+name+".log", "Failed", 300)

	result := SeedStats(dir, []domain.Job{job}, 2)
	s := result[job.ID]

	if s.RunCount != 2 {
		t.Errorf("RunCount = %d, want 2 (maxFiles=2)", s.RunCount)
	}
	if s.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", s.FailCount)
	}
}

// TestSeedStatsMissingDir yields an empty map and does not panic.
func TestSeedStatsMissingDir(t *testing.T) {
	result := SeedStats(filepath.Join(t.TempDir(), "no-such-dir"), []domain.Job{{ID: 1, Name: "J"}}, 0)
	if len(result) != 0 {
		t.Errorf("expected empty result for missing dir, got %v", result)
	}
}

// TestSeedStatsUnknownJobProducesNoEntry verifies that log files not matching
// any known job are silently ignored.
func TestSeedStatsUnknownJobProducesNoEntry(t *testing.T) {
	dir := t.TempDir()
	writeTestLog(t, dir, "20260601-100000_UnknownJob.log", "OK", 100)

	result := SeedStats(dir, []domain.Job{{ID: 1, Name: "KnownJob"}}, 0)
	if _, ok := result[1]; ok {
		t.Error("expected no entry for a job with no matching log files")
	}
}
