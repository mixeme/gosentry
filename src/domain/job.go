package domain

// Job is the user-visible scheduled command. It contains only durable
// configuration: every field is persisted to jobs.yaml. Transient execution
// state (last run, next run, command output, in-memory activity) lives in a
// separate JobRuntime so the jobs file stays a clean, hand-editable record of
// configuration and never mixes in process-lifetime bookkeeping.
type Job struct {
	ID               int    `yaml:"id"`
	Name             string `yaml:"name"`
	Folder           string `yaml:"folder,omitempty"`
	Schedule         string `yaml:"schedule"`
	Command          string `yaml:"command"`
	Arguments        string `yaml:"arguments,omitempty"`
	SuccessExitCodes string `yaml:"success_exit_codes,omitempty"`
	StartOnly        bool   `yaml:"start_only,omitempty"`
	Enabled          bool   `yaml:"enabled"`
}
