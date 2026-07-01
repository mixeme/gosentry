package ui

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2/widget"
)

func TestLastJobLogsCapsAndCopies(t *testing.T) {
	logs := []event{
		{Time: "1", JobName: "a"},
		{Time: "2", JobName: "b"},
		{Time: "3", JobName: "c"},
		{Time: "4", JobName: "d"},
	}
	got := lastJobLogs(logs)
	if len(got) != maxJobActivityRows {
		t.Fatalf("len = %d, want %d", len(got), maxJobActivityRows)
	}
	for i, want := range []string{"1", "2", "3"} {
		if got[i].Time != want {
			t.Errorf("got[%d].Time = %q, want %q", i, got[i].Time, want)
		}
	}
	logs[0].Time = "mutated"
	if got[0].Time == "mutated" {
		t.Error("lastJobLogs must return a defensive copy")
	}
}

func TestLastJobLogsEmpty(t *testing.T) {
	if got := lastJobLogs(nil); len(got) != 0 {
		t.Errorf("nil input: got %v, want empty", got)
	}
}

func TestIndexOfID(t *testing.T) {
	jobs := []job{
		{ID: 10, Name: "A"},
		{ID: 20, Name: "B"},
	}
	if got := indexOfID(jobs, 20); got != 1 {
		t.Errorf("found: got %d, want 1", got)
	}
	if got := indexOfID(jobs, 99); got != -1 {
		t.Errorf("missing: got %d, want -1", got)
	}
	if got := indexOfID(nil, 1); got != -1 {
		t.Errorf("empty slice: got %d, want -1", got)
	}
}

func TestCollectActivityMergesAndSorts(t *testing.T) {
	jobs := []job{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B"},
	}
	runtimes := map[int]*domain.JobRuntime{
		1: {Logs: []domain.RunRecord{{Time: "2026-01-02 10:00:00", JobID: 1}}},
		2: {Logs: []domain.RunRecord{{Time: "2026-01-01 09:00:00", JobID: 2}}},
	}
	got := collectActivity(jobs, runtimes)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Time != "2026-01-01 09:00:00" || got[1].Time != "2026-01-02 10:00:00" {
		t.Errorf("sort order = %v, want ascending by Time", got)
	}
}

func TestCollectActivitySkipsMissingRuntimes(t *testing.T) {
	jobs := []job{{ID: 1, Name: "A"}}
	if got := collectActivity(jobs, nil); len(got) != 0 {
		t.Errorf("nil runtimes: got %v, want empty", got)
	}
}

func TestHistoryCellText(t *testing.T) {
	events := []event{{
		Time:    "2026-06-01 12:00:00",
		Trigger: "",
		JobName: "Job",
		State:   "OK",
		Detail:  "done",
		LogFile: `/logs/20260601-120000_Job.log`,
	}}
	cases := []struct {
		col  int
		want string
	}{
		{0, "2026-06-01 12:00:00"},
		{1, "Unknown"},
		{2, "Job"},
		{3, "OK"},
		{4, "done"},
		{5, "20260601-120000_Job.log"},
	}
	for _, tc := range cases {
		got := historyCellText(widget.TableCellID{Row: 0, Col: tc.col}, events)
		if got != tc.want {
			t.Errorf("col %d: got %q, want %q", tc.col, got, tc.want)
		}
	}
	if got := historyCellText(widget.TableCellID{Row: -1, Col: 0}, events); got != "" {
		t.Errorf("header row: got %q, want empty", got)
	}
	if got := historyCellText(widget.TableCellID{Row: 99, Col: 0}, events); got != "" {
		t.Errorf("out of range row: got %q, want empty", got)
	}
}

func TestLogFileName(t *testing.T) {
	cases := []struct{ path, want string }{
		{"", ""},
		{"   ", ""},
		{`C:\logs\run.log`, "run.log"},
		{"/var/logs/2026/job.log", "job.log"},
		{"plain.log", "plain.log"},
	}
	for _, tc := range cases {
		if got := logFileName(tc.path); got != tc.want {
			t.Errorf("logFileName(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestNewEventUsesConsistentTimestampShape(t *testing.T) {
	ev := newEvent(1, "Job", "OK", "detail")
	if _, err := time.Parse("2006-01-02 15:04:05", ev.Time); err != nil {
		t.Errorf("timestamp %q is not in expected layout: %v", ev.Time, err)
	}
	if ev.Trigger != "UI" || ev.JobID != 1 || ev.JobName != "Job" {
		t.Errorf("unexpected event fields: %+v", ev)
	}
}
