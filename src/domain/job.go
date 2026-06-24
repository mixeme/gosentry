package domain

// Job is the user-visible scheduled command. It contains only durable
// configuration: every field is persisted to jobs.json. Transient execution
// state (last run, next run, command output, in-memory activity) lives in a
// separate JobRuntime so the jobs file stays a clean, hand-editable record of
// configuration and never mixes in process-lifetime bookkeeping.
type Job struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Folder           string `json:"folder,omitempty"`
	Schedule         string `json:"schedule"`
	Command          string `json:"command"`
	Arguments        string `json:"arguments,omitempty"`
	StartOnly        bool   `json:"start_only,omitempty"`
	Enabled          bool   `json:"enabled"`
	OverlapPolicy    string `json:"overlap_policy,omitempty"`
}
