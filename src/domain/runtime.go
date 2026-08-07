package domain

import "time"

// JobRuntime is the transient execution state for a Job. It is never written to
// jobs.json: it is rebuilt from scratch each time GoSentry starts and is held in
// memory keyed by Job.ID for the lifetime of the process. Keeping it separate
// from Job is what lets the durable configuration file stay free of run records,
// status strings, and scheduling bookkeeping.
type JobRuntime struct {
	LastRun   string
	NextRun   string
	LastState string
	Output    string
	Logs      []RunRecord

	// NextDue is the next scheduled execution time, kept as time.Time for
	// scheduler comparisons. NextRun above is its formatted display string and is
	// the only form shown in the GUI.
	NextDue time.Time

	// PendingRuns counts scheduled occurrences that fired while a run was still
	// in flight under the "queue" overlap policy. executeRun drains the counter
	// by starting one deferred run after each completion.
	PendingRuns int

	// Execution-time statistics accumulated since the last process start.
	// Seeded from log files on startup; zero until then.
	RunCount       int
	FailCount      int
	LastDurationMS int64
	AvgDurationMS  int64
	MaxDurationMS  int64
	// TimedRunCount is the number of runs that contributed to AvgDurationMS.
	// Runs with no recorded duration (legacy logs, or sub-millisecond StartOnly
	// launches that round to 0) increment RunCount but not this. StartOnly runs
	// otherwise contribute their launch latency.
	TimedRunCount int
	// DurationSumMS is the running total of every timed run's duration.
	// AvgDurationMS is always DurationSumMS/TimedRunCount, computed fresh on each
	// update rather than folded incrementally — an incremental integer mean
	// truncates on every step, and the error compounds over the life of a job
	// that keeps running. A stored sum divided once per update matches the exact
	// sum/count average runner.aggregateLogStats computes when seeding from logs,
	// so the two no longer disagree about the same run history.
	DurationSumMS int64
}

// NewRuntime builds the initial runtime state for a freshly loaded or created
// job. Enabled jobs start "Ready" and wait for the scheduler to compute their
// first run; disabled jobs start "Paused".
func NewRuntime(job Job) *JobRuntime {
	runtime := &JobRuntime{
		LastRun: "Never",
		Output:  "No command output captured yet.",
	}
	if job.Enabled {
		runtime.LastState = "Ready"
		runtime.NextRun = "After start"
	} else {
		runtime.LastState = "Paused"
		runtime.NextRun = "Paused"
	}
	return runtime
}

// NewRuntimes builds a runtime map for a slice of jobs, keyed by Job.ID. It is
// the convenience entry point used when a whole jobs file has just been loaded.
func NewRuntimes(jobs []Job) map[int]*JobRuntime {
	runtimes := make(map[int]*JobRuntime, len(jobs))
	for _, job := range jobs {
		runtimes[job.ID] = NewRuntime(job)
	}
	return runtimes
}
