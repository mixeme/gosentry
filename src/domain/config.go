package domain

// StartInTrayArgument is written to the Windows Startup shortcut so autostart
// can keep the scheduler running without flashing the main window. Manual
// launches omit this flag and open the normal window.
const StartInTrayArgument = "--start-in-tray"

// ExecutionMode controls whether due jobs run concurrently or one at a time.
type ExecutionMode string

const (
	// ExecutionModeParallel allows all due jobs to start simultaneously.
	ExecutionModeParallel ExecutionMode = "parallel"
	// ExecutionModeSequential runs due jobs one after another, in order.
	ExecutionModeSequential ExecutionMode = "sequential"
)

// Theme selects the application's visual appearance. It is a UI-only choice with
// no effect on scheduling; it is stored in Config so it persists across launches
// alongside the other desktop-shell preferences.
type Theme string

const (
	// ThemeDefault keeps Fyne's built-in theme — the original look.
	ThemeDefault Theme = "default"
	// ThemeGoSentry applies the branded teal/amber theme derived from the logo
	// and app icon.
	ThemeGoSentry Theme = "gosentry"
)

// JobListView selects how densely the Jobs tab renders its sidebar list. Like
// Theme it is a UI-only choice with no effect on scheduling; it lives in Config
// so the user's preference survives a restart.
type JobListView string

const (
	// JobListViewDetailed is the three-line row: name, metadata, status.
	JobListViewDetailed JobListView = "detailed"
	// JobListViewCompact is the one-line row: name on the left, status on the
	// right, so many more jobs fit without scrolling.
	JobListViewCompact JobListView = "compact"
)

// IsCompact reports whether the compact rendering is selected. Only the exact
// "compact" value counts, so empty, legacy, and unrecognised values all read as
// detailed — every consumer normalizes them the same way.
func (v JobListView) IsCompact() bool {
	return v == JobListViewCompact
}

// OverlapPolicy decides what happens when a job's next run fires while the
// previous run is still active.
type OverlapPolicy string

const (
	// OverlapPolicySkip discards the new run when the job is already running.
	OverlapPolicySkip OverlapPolicy = "skip"
	// OverlapPolicyQueue holds the new run and starts it as soon as the current
	// run finishes.
	OverlapPolicyQueue OverlapPolicy = "queue"
)

// Config is stored in gosentry.json next to the program. It contains only
// application-level choices: where to read jobs from, where to write logs, and
// how the desktop shell should behave.
type Config struct {
	// JobsFile is the full path of the JSON file holding the job definitions,
	// file name included, so the user can keep jobs under any name they like. A
	// relative path is resolved against the program folder.
	JobsFile string `json:"jobs_file"`
	// JobsDir is the pre-0.15 setting that named only the directory, with the
	// file name fixed to jobs.json. It is still read so an older gosentry.json
	// keeps working: storage.loadOrCreateConfig turns it into JobsFile and
	// clears it, so the field disappears from the file on the next save.
	JobsDir           string        `json:"jobs_dir,omitempty"`
	LogsDir           string        `json:"logs_dir"`
	MaxLogFiles       int           `json:"max_log_files"`
	MaxLogAgeDays     int           `json:"max_log_age_days"`
	StartOnLogin      bool          `json:"start_on_login,omitempty"`
	KeepRunningInTray bool          `json:"keep_running_in_tray,omitempty"`
	NotifyOnFailure   bool          `json:"notify_on_failure,omitempty"`
	ExecutionMode     ExecutionMode `json:"execution_mode,omitempty"`
	OverlapPolicy     OverlapPolicy `json:"overlap_policy,omitempty"`
	// DefaultTimeoutSeconds is the run timeout applied to jobs that leave their
	// own Job.TimeoutSeconds unset. 0 (the default) means no timeout: such jobs
	// run to completion however long that takes. It is written even when 0 —
	// omitempty would hide a deliberate choice from the hand-editable config.
	DefaultTimeoutSeconds int  `json:"default_timeout_seconds"`
	Paused                bool `json:"paused,omitempty"`
	// Theme selects the visual appearance. Empty is treated as ThemeGoSentry so
	// configs written before this field existed pick up the branded look.
	Theme Theme `json:"theme,omitempty"`
	// JobListView selects the Jobs list density. Empty is treated as
	// JobListViewDetailed so configs written before this field existed keep the
	// current three-line rows.
	JobListView JobListView `json:"job_list_view,omitempty"`
}

// DefaultConfig returns the built-in default settings. It is the config used
// when gosentry.json does not yet exist, and is also what the Settings UI
// offers to restore via its "Defaults" button.
func DefaultConfig() Config {
	return Config{
		JobsFile:              "jobs.json",
		LogsDir:               "logs",
		MaxLogFiles:           100,
		MaxLogAgeDays:         30,
		StartOnLogin:          false,
		KeepRunningInTray:     true,
		NotifyOnFailure:       true,
		ExecutionMode:         ExecutionModeParallel,
		OverlapPolicy:         OverlapPolicySkip,
		Theme:                 ThemeGoSentry,
		JobListView:           JobListViewDetailed,
		DefaultTimeoutSeconds: 0,
	}
}

// JobsFile is the on-disk shape of jobs.json. Wrapping the slice in a top-level
// object leaves room for future metadata without breaking the basic file format.
type JobsFile struct {
	Jobs []Job `json:"jobs"`
}
