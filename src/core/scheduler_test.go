package core

import (
	"strings"
	"testing"
	"time"
)

func TestNextRunTimeSupportsEvery(t *testing.T) {
	from := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	next, ok := nextRunTime("@every 10s", from)
	if !ok {
		t.Fatal("expected @every schedule to parse")
	}
	if want := from.Add(10 * time.Second); !next.Equal(want) {
		t.Fatalf("expected %s, got %s", want, next)
	}
}

func TestNextRunTimeSupportsCron(t *testing.T) {
	from := time.Date(2026, 6, 14, 12, 3, 0, 0, time.UTC)
	next, ok := nextRunTime("*/5 * * * *", from)
	if !ok {
		t.Fatal("expected cron schedule to parse")
	}
	want := time.Date(2026, 6, 14, 12, 5, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("expected %s, got %s", want, next)
	}
}

func TestRunningOutputIncludesInvocation(t *testing.T) {
	started := time.Date(2026, 6, 17, 23, 40, 0, 0, time.Local)
	job := Job{
		Name:             "Backup",
		Command:          `C:\Program Files\FreeFileSync\FreeFileSync.exe`,
		Arguments:        `D:\Local\Jobs\Auto.ffs_batch`,
		SuccessExitCodes: "0,1",
	}

	output := runningOutput(job, "Manual", started)
	for _, want := range []string{
		"Running since 2026-06-17 23:40:00",
		"Manual",
		job.Command,
		job.Arguments,
		"0,1",
		"start_only",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected running output to contain %q, got:\n%s", want, output)
		}
	}
}
