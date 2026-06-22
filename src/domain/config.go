package domain

// StartInTrayArgument is written to the Windows Startup shortcut so autostart
// can keep the scheduler running without flashing the main window. Manual
// launches omit this flag and open the normal window.
const StartInTrayArgument = "--start-in-tray"

// Config is stored in gosentry.json next to the program. It contains only
// application-level choices: where to read jobs from, where to write logs, and
// how the desktop shell should behave.
type Config struct {
	JobsDir           string `json:"jobs_dir"`
	LogsDir           string `json:"logs_dir"`
	MaxLogFiles       int    `json:"max_log_files"`
	MaxLogAgeDays     int    `json:"max_log_age_days"`
	StartOnLogin      bool   `json:"start_on_login,omitempty"`
	KeepRunningInTray bool   `json:"keep_running_in_tray,omitempty"`
	NotifyOnFailure   bool   `json:"notify_on_failure,omitempty"`
}

// JobsFile is the on-disk shape of jobs.json. Wrapping the slice in a top-level
// object leaves room for future metadata without breaking the basic file format.
type JobsFile struct {
	Jobs []Job `json:"jobs"`
}
