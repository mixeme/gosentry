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

func TestDisplaySuccessExitCodes(t *testing.T) {
	if got := DisplaySuccessExitCodes("  "); got != "0" {
		t.Errorf("empty codes = %q, want %q", got, "0")
	}
	if got := DisplaySuccessExitCodes(" 0,1 "); got != "0,1" {
		t.Errorf("codes = %q, want %q", got, "0,1")
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
