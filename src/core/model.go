package core

import "time"

// StartInTrayArgument is written to the Windows Startup shortcut so autostart
// can keep the scheduler running without flashing the main window. Manual
// launches omit this flag and open the normal window.
const StartInTrayArgument = "--start-in-tray"

// Config is stored in pysentry.yaml next to the program. It contains only
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

// Job is the user-visible scheduled command.
//
// Fields with yaml:"-" are deliberately runtime-only. They are useful in the GUI
// while PySentry is running, but writing them to jobs.yaml would make the jobs
// file noisy and would mix durable configuration with transient execution state.
type Job struct {
	ID        int         `yaml:"id"`
	Name      string      `yaml:"name"`
	Folder    string      `yaml:"folder,omitempty"`
	Schedule  string      `yaml:"schedule"`
	Command   string      `yaml:"command"`
	Enabled   bool        `yaml:"enabled"`
	LastRun   string      `yaml:"-"`
	NextRun   string      `yaml:"-"`
	LastState string      `yaml:"-"`
	Logs      []RunRecord `yaml:"-"`
	Output    string      `yaml:"-"`

	// nextDue is kept as time.Time for scheduler comparisons. The formatted
	// NextRun string above exists only for display in the GUI and YAML rewriting
	// must not persist it.
	nextDue time.Time
}

// RunRecord represents one visible activity item. Scheduled and manual command
// output is also written to a log file; the in-memory Output copy exists so the
// latest run can be displayed without reopening the log on every repaint.
type RunRecord struct {
	Time    string `yaml:"time"`
	JobID   int    `yaml:"job_id"`
	JobName string `yaml:"job_name"`
	Trigger string `yaml:"trigger,omitempty"`
	State   string `yaml:"state"`
	Detail  string `yaml:"detail"`
	LogFile string `yaml:"log_file,omitempty"`
	Output  string `yaml:"output,omitempty"`
}
