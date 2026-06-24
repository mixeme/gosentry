package app

import (
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

func TestStatusText(t *testing.T) {
	tests := []struct {
		name    string
		job     domain.Job
		runtime *domain.JobRuntime
		want    string
	}{
		{"disabled is paused", domain.Job{Enabled: false}, &domain.JobRuntime{LastState: "Running"}, "Paused"},
		{"enabled shows runtime state", domain.Job{Enabled: true}, &domain.JobRuntime{LastState: "Success"}, "Success"},
		{"enabled with nil runtime is empty", domain.Job{Enabled: true}, nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := StatusText(tc.job, tc.runtime); got != tc.want {
				t.Errorf("StatusText = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEventText(t *testing.T) {
	withLog := domain.RunRecord{
		Time: "2026-06-19 12:00:00", Trigger: "Schedule", JobName: "Build",
		State: "Success", Detail: "ok", LogFile: "build.log",
	}
	if got, want := EventText(withLog), "2026-06-19 12:00:00  Schedule  Build  Success  ok  build.log"; got != want {
		t.Errorf("EventText with log = %q, want %q", got, want)
	}

	noLog := domain.RunRecord{
		Time: "2026-06-19 12:00:00", Trigger: "Manual", JobName: "Build",
		State: "Success", Detail: "ok",
	}
	if got, want := EventText(noLog), "2026-06-19 12:00:00  Manual  Build  Success  ok"; got != want {
		t.Errorf("EventText without log = %q, want %q", got, want)
	}

	// An empty trigger is shown as "Unknown".
	blank := domain.RunRecord{Time: "t", JobName: "J", State: "S", Detail: "d"}
	if got, want := EventText(blank), "t  Unknown  J  S  d"; got != want {
		t.Errorf("EventText blank trigger = %q, want %q", got, want)
	}
}

func TestDisplayFolder(t *testing.T) {
	if got := DisplayFolder("   "); got != "(No folder)" {
		t.Errorf("blank folder = %q, want %q", got, "(No folder)")
	}
	if got := DisplayFolder("  Reports  "); got != "Reports" {
		t.Errorf("folder = %q, want %q", got, "Reports")
	}
}

func TestDisplayArguments(t *testing.T) {
	if got := DisplayArguments(""); got != "(none)" {
		t.Errorf("empty args = %q, want %q", got, "(none)")
	}
	if got := DisplayArguments("  -v  "); got != "-v" {
		t.Errorf("args = %q, want %q", got, "-v")
	}
}

func TestDisplayRunMode(t *testing.T) {
	if got := DisplayRunMode(domain.Job{StartOnly: true}); got != "Start only" {
		t.Errorf("start-only = %q, want %q", got, "Start only")
	}
	if got := DisplayRunMode(domain.Job{StartOnly: false}); got != "Wait for completion" {
		t.Errorf("wait = %q, want %q", got, "Wait for completion")
	}
}

func TestDisplayInvocation(t *testing.T) {
	if got := DisplayInvocation(domain.Job{Command: "echo"}); got != "echo" {
		t.Errorf("no args = %q, want %q", got, "echo")
	}
	// Arguments are appended with spacing and their newlines collapsed to spaces.
	job := domain.Job{Command: "echo", Arguments: "  hi\nthere  "}
	if got, want := DisplayInvocation(job), "echo    hi there"; got != want {
		t.Errorf("with args = %q, want %q", got, want)
	}
}

func TestDisplayIndex(t *testing.T) {
	indexes := []int{4, 7, 2}
	if got := DisplayIndex(indexes, 7); got != 1 {
		t.Errorf("DisplayIndex(7) = %d, want 1", got)
	}
	// A jobIndex not present returns 0.
	if got := DisplayIndex(indexes, 99); got != 0 {
		t.Errorf("DisplayIndex(missing) = %d, want 0", got)
	}
}

func TestDisplayStats(t *testing.T) {
	// Zero RunCount → sentinel string.
	if got := DisplayStats(nil); got != "No runs recorded" {
		t.Errorf("nil runtime = %q, want %q", got, "No runs recorded")
	}
	if got := DisplayStats(&domain.JobRuntime{}); got != "No runs recorded" {
		t.Errorf("zero runtime = %q, want %q", got, "No runs recorded")
	}

	rt := &domain.JobRuntime{
		RunCount:       5,
		FailCount:      2,
		LastDurationMS: 450,
		AvgDurationMS:  380,
		MaxDurationMS:  520,
	}
	want := "5 runs, 2 failed, last 450 ms, avg 380 ms, max 520 ms"
	if got := DisplayStats(rt); got != want {
		t.Errorf("DisplayStats = %q, want %q", got, want)
	}

	// Zero failures are included in the output (not hidden).
	rtNoFail := &domain.JobRuntime{RunCount: 3, FailCount: 0, LastDurationMS: 100, AvgDurationMS: 90, MaxDurationMS: 110}
	wantNoFail := "3 runs, 0 failed, last 100 ms, avg 90 ms, max 110 ms"
	if got := DisplayStats(rtNoFail); got != wantNoFail {
		t.Errorf("DisplayStats no-fail = %q, want %q", got, wantNoFail)
	}
}

func TestEventLine(t *testing.T) {
	withLog := domain.RunRecord{
		Time: "2026-06-19 12:00:00", Trigger: "Schedule", JobName: "Build",
		State: "Success", Detail: "ok", LogFile: "/home/user/logs/build-20260619.log",
	}
	if got, want := EventLine(withLog), "2026-06-19 12:00:00  Schedule  Build  Success  ok  build-20260619.log"; got != want {
		t.Errorf("EventLine with log = %q, want %q", got, want)
	}

	noLog := domain.RunRecord{
		Time: "2026-06-19 12:00:00", Trigger: "Manual", JobName: "Build",
		State: "Success", Detail: "ok",
	}
	if got, want := EventLine(noLog), "2026-06-19 12:00:00  Manual  Build  Success  ok"; got != want {
		t.Errorf("EventLine without log = %q, want %q", got, want)
	}

	// An empty trigger is shown as "Unknown".
	blank := domain.RunRecord{Time: "t", JobName: "J", State: "S", Detail: "d", LogFile: "/path/to/file.log"}
	if got, want := EventLine(blank), "t  Unknown  J  S  d  file.log"; got != want {
		t.Errorf("EventLine blank trigger = %q, want %q", got, want)
	}
}
