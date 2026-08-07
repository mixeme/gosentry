package domain

// RunRecord represents one visible activity item. Scheduled and manual command
// output is also written to a log file; the in-memory Output copy exists so the
// latest run can be displayed without reopening the log on every repaint.
type RunRecord struct {
	Time       string
	JobID      int
	JobName    string
	Trigger    string
	State      string
	Detail     string
	LogFile    string
	Output     string
	DurationMS int64
}
