package domain

// StartInTrayArgument is written to the Windows Startup shortcut so autostart
// can keep the scheduler running without flashing the main window. Manual
// launches omit this flag and open the normal window.
const StartInTrayArgument = "--start-in-tray"

// Config is stored in gosentry.yaml next to the program. It contains only
// application-level choices: where to read jobs from, where to write logs, and
// how the desktop shell should behave.
type Config struct {
	JobsDir           string `yaml:"jobs_dir"`
	LogsDir           string `yaml:"logs_dir"`
	MaxLogFiles       int    `yaml:"max_log_files"`
	MaxLogAgeDays     int    `yaml:"max_log_age_days"`
	StartOnLogin      bool   `yaml:"start_on_login"`
	KeepRunningInTray bool   `yaml:"keep_running_in_tray"`
	NotifyOnFailure   bool   `yaml:"notify_on_failure"`
}

// JobsFile is the on-disk shape of jobs.yaml. Wrapping the slice in a top-level
// object leaves room for future metadata without breaking the basic file format.
type JobsFile struct {
	Jobs []Job `yaml:"jobs"`
}
