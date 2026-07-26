package domain

// Job is the user-visible scheduled command. It contains only durable
// configuration: every field is persisted to jobs.json. Transient execution
// state (last run, next run, command output, in-memory activity) lives in a
// separate JobRuntime so the jobs file stays a clean, hand-editable record of
// configuration and never mixes in process-lifetime bookkeeping.
type Job struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Folder        string `json:"folder,omitempty"`
	Schedule      string `json:"schedule"`
	Command       string `json:"command"`
	Arguments     string `json:"arguments,omitempty"`
	StartOnly     bool   `json:"start_only,omitempty"`
	Enabled       bool   `json:"enabled"`
	OverlapPolicy string `json:"overlap_policy,omitempty"`
	// TimeoutSeconds bounds how long a run may take before it is killed. It is a
	// pointer so the three states stay distinguishable on disk: absent (nil)
	// means "inherit the global Config.DefaultTimeoutSeconds", mirroring
	// OverlapPolicy's empty string; an explicit 0 means "no timeout" and does
	// not inherit; a positive value is the per-job limit in seconds. The
	// inherited global default may itself be 0, also meaning no timeout.
	// normalizeJobs must leave nil untouched rather than backfilling a value.
	TimeoutSeconds *int `json:"timeout_seconds,omitempty"`
}

// TimeoutSecondsPtr returns a pointer suitable for Job.TimeoutSeconds. It exists
// because nil (inherit) and an explicit 0 (no timeout) are different states, so
// callers cannot just assign an int.
func TimeoutSecondsPtr(seconds int) *int {
	return &seconds
}
