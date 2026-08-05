package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const notificationTimingLogName = "notify-timing.log"

// notificationTiming captures wall-clock points from a failed run through
// SendNotification. It does not include OS toast display latency — Fyne on
// Windows shows toasts via a separate PowerShell process after SendNotification
// returns.
type notificationTiming struct {
	JobName     string
	RunFinished time.Time
	EmittedAt   time.Time
	UIQueuedAt  time.Time
	AfterSendAt time.Time
}

func (t notificationTiming) formatLine() string {
	return fmt.Sprintf(
		"%s\tjob=%s\tms_after_run=%s\tms_fyne_do=%s\tms_send=%s\tms_app_total=%s\n",
		t.AfterSendAt.Format(time.RFC3339Nano),
		t.JobName,
		msBetween(t.RunFinished, t.EmittedAt),
		msBetween(t.EmittedAt, t.UIQueuedAt),
		msBetween(t.UIQueuedAt, t.AfterSendAt),
		msBetween(t.EmittedAt, t.AfterSendAt),
	)
}

func msBetween(from, to time.Time) string {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return "-"
	}
	return fmt.Sprintf("%d", to.Sub(from).Milliseconds())
}

func appendNotificationTimingLog(logsDir string, timing notificationTiming) error {
	if logsDir == "" {
		return nil
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(logsDir, notificationTimingLogName)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		if _, err := file.WriteString("# GoSentry failure-notification timing (app side only; OS toast delay is not included)\n" +
			"# columns: timestamp job ms_after_run ms_fyne_do ms_send ms_app_total\n"); err != nil {
			return err
		}
	}
	_, err = file.WriteString(timing.formatLine())
	return err
}
