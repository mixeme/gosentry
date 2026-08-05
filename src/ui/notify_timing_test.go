package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotificationTimingFormatLine(t *testing.T) {
	runFinished := time.Date(2026, 8, 5, 23, 0, 0, 0, time.Local)
	emitted := runFinished.Add(15 * time.Millisecond)
	uiQueued := emitted.Add(4 * time.Millisecond)
	afterSend := uiQueued.Add(2 * time.Millisecond)

	line := notificationTiming{
		JobName:     "Failure notification test",
		RunFinished: runFinished,
		EmittedAt:   emitted,
		UIQueuedAt:  uiQueued,
		AfterSendAt: afterSend,
	}.formatLine()

	if !strings.Contains(line, "job=Failure notification test") {
		t.Fatalf("line = %q, want job name", line)
	}
	for _, want := range []string{"ms_after_run=15", "ms_fyne_do=4", "ms_send=2", "ms_app_total=6"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line = %q, want substring %q", line, want)
		}
	}
}

func TestAppendNotificationTimingLogWritesHeaderAndRow(t *testing.T) {
	dir := t.TempDir()
	timing := notificationTiming{
		JobName:     "demo",
		RunFinished: time.Now().Add(-10 * time.Millisecond),
		EmittedAt:   time.Now().Add(-5 * time.Millisecond),
		UIQueuedAt:  time.Now().Add(-2 * time.Millisecond),
		AfterSendAt: time.Now(),
	}
	if err := appendNotificationTimingLog(dir, timing); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, notificationTimingLogName))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "# GoSentry failure-notification timing") {
		t.Fatalf("log = %q, want header", text)
	}
	if !strings.Contains(text, "job=demo") {
		t.Fatalf("log = %q, want timing row", text)
	}
}
