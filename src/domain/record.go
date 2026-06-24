package domain

// RunRecord represents one visible activity item. Scheduled and manual command
// output is also written to a log file; the in-memory Output copy exists so the
// latest run can be displayed without reopening the log on every repaint.
type RunRecord struct {
	Time       string `yaml:"time"`
	JobID      int    `yaml:"job_id"`
	JobName    string `yaml:"job_name"`
	Trigger    string `yaml:"trigger,omitempty"`
	State      string `yaml:"state"`
	Detail     string `yaml:"detail"`
	LogFile    string `yaml:"log_file,omitempty"`
	Output     string `yaml:"output,omitempty"`
	DurationMS int64  `yaml:"duration_ms,omitempty"`
}
