package domain

import "time"

// JobRuntime is the transient execution state for a Job. It is never written to
// jobs.yaml: it is rebuilt from scratch each time GoSentry starts and is held in
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

	// Pending is set when a run was skipped due to the overlap policy being
	// "queue". The scheduler will start this job as soon as the current run ends.
	Pending bool

	// Execution-time statistics accumulated since the last process start.
	// Seeded from log files on startup by T2.5; zero until then.
	RunCount     int
	FailCount    int
	LastDurationMS int64
	AvgDurationMS  int64
	MaxDurationMS  int64
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
