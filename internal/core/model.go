package core

import "time"

type Config struct {
	JobsDir           string `yaml:"jobs_dir"`
	LogsDir           string `yaml:"logs_dir"`
	MaxLogFiles       int    `yaml:"max_log_files"`
	MaxLogAgeDays     int    `yaml:"max_log_age_days"`
	KeepRunningInTray bool   `yaml:"keep_running_in_tray"`
	NotifyOnFailure   bool   `yaml:"notify_on_failure"`
}

type JobsFile struct {
	Jobs []Job `yaml:"jobs"`
}

type Job struct {
	ID        int         `yaml:"id"`
	Name      string      `yaml:"name"`
	Folder    string      `yaml:"folder,omitempty"`
	Schedule  string      `yaml:"schedule"`
	Command   string      `yaml:"command"`
	Enabled   bool        `yaml:"enabled"`
	LastRun   string      `yaml:"last_run,omitempty"`
	NextRun   string      `yaml:"next_run,omitempty"`
	LastState string      `yaml:"last_state,omitempty"`
	Logs      []RunRecord `yaml:"activity,omitempty"`
	Output    string      `yaml:"last_output,omitempty"`

	nextDue time.Time
}

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
