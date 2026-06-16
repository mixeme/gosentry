package core

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.yaml.in/yaml/v4"
)

type Store struct {
	Paths  Paths
	Config Config
}

func OpenStore() (*Store, []Job, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return nil, nil, err
	}

	store := &Store{Paths: paths}
	config, err := loadOrCreateConfig(paths)
	if err != nil {
		return nil, nil, err
	}
	store.Config = config
	store.applyConfigPaths()
	// Save the config after loading so missing defaults are written back. This
	// rewrites old or hand-edited files into the current clean schema without
	// forcing the user to delete them manually.
	if err := store.SaveConfig(); err != nil {
		return nil, nil, err
	}

	jobs, err := loadOrCreateJobs(store.Paths.JobsPath)
	if err != nil {
		return nil, nil, err
	}
	normalizeJobs(jobs)
	// Jobs are also rewritten after normalization. That keeps jobs.yaml compact:
	// only durable job definitions remain, because runtime fields are tagged
	// yaml:"-" in the model.
	if err := store.SaveJobs(jobs); err != nil {
		return nil, nil, err
	}
	return store, jobs, nil
}

func (s *Store) SaveConfig() error {
	s.applyConfigPaths()
	if err := os.MkdirAll(s.Paths.AppDir, 0o755); err != nil {
		return err
	}
	return writeYAML(s.Paths.ConfigPath, s.Config)
}

func (s *Store) SaveJobs(jobs []Job) error {
	if err := os.MkdirAll(s.Paths.JobsDir, 0o755); err != nil {
		return err
	}
	return writeYAML(s.Paths.JobsPath, JobsFile{Jobs: jobs})
}

func loadOrCreateConfig(paths Paths) (Config, error) {
	// Defaults favor a portable installation: settings and jobs begin next to the
	// executable, while logs are grouped under a dedicated subdirectory.
	config := Config{
		JobsDir:           ".",
		LogsDir:           "logs",
		MaxLogFiles:       100,
		MaxLogAgeDays:     30,
		StartOnLogin:      false,
		KeepRunningInTray: true,
		NotifyOnFailure:   true,
	}

	if _, err := os.Stat(paths.ConfigPath); errors.Is(err, os.ErrNotExist) {
		return config, writeYAML(paths.ConfigPath, config)
	}

	data, err := os.ReadFile(paths.ConfigPath)
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(config.JobsDir) == "" {
		// Empty paths are treated as missing values rather than intentional root
		// directories. This avoids accidentally writing jobs to unexpected places.
		config.JobsDir = "."
	}
	if strings.TrimSpace(config.LogsDir) == "" {
		config.LogsDir = "logs"
	}
	if config.MaxLogFiles <= 0 {
		config.MaxLogFiles = 100
	}
	if config.MaxLogAgeDays <= 0 {
		config.MaxLogAgeDays = 30
	}
	return config, nil
}

func loadOrCreateJobs(path string) ([]Job, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		// The first run creates harmless sample jobs so a new user can immediately
		// see scheduled and manual execution without inventing a command.
		jobs := defaultJobs()
		normalizeJobs(jobs)
		return jobs, writeYAML(path, JobsFile{Jobs: jobs})
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file JobsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return file.Jobs, nil
}

func normalizeJobs(jobs []Job) {
	next := 1
	for index := range jobs {
		job := &jobs[index]
		if job.ID <= 0 {
			// IDs are assigned only when absent. Existing IDs stay stable because
			// History and future log associations use them to identify jobs.
			job.ID = next
		}
		if job.ID >= next {
			next = job.ID + 1
		}
		if strings.TrimSpace(job.Name) == "" {
			job.Name = "Untitled job"
		}
		if strings.TrimSpace(job.Schedule) == "" {
			job.Schedule = "@every 1m"
		}
		if strings.TrimSpace(job.Command) == "" {
			// An empty command would fail in a confusing way. A safe echo command
			// gives the user something observable and harmless instead.
			job.Command = echoCommand("PySentry job ran")
		}
		if job.LastRun == "" {
			job.LastRun = "Never"
		}
		if job.Output == "" {
			job.Output = "No command output captured yet."
		}
		if job.Enabled {
			job.LastState = "Ready"
			job.NextRun = "After start"
		} else {
			job.LastState = "Paused"
			job.NextRun = "Paused"
		}
		// Runtime fields are reconstructed each time the app starts. Persisted run
		// records live in log files, not in jobs.yaml, to keep the jobs file easy
		// to review and edit by hand.
		job.Logs = nil
	}
}

func resolveJobsDir(appDir string, jobsDir string) string {
	return resolveConfiguredDir(appDir, jobsDir)
}

func resolveConfiguredDir(appDir string, dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	// Relative paths are resolved against the executable directory, not the
	// process working directory. This matches ResolvePaths and keeps shortcuts,
	// Explorer launches, and terminal launches consistent.
	return filepath.Clean(filepath.Join(appDir, dir))
}

func (s *Store) applyConfigPaths() {
	s.Paths.JobsDir = resolveConfiguredDir(s.Paths.AppDir, s.Config.JobsDir)
	s.Paths.JobsPath = filepath.Join(s.Paths.JobsDir, JobsFileName)
	s.Paths.LogsDir = resolveConfiguredDir(s.Paths.AppDir, s.Config.LogsDir)
}

func writeYAML(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	// WriteFile replaces the full file instead of patching it in place. For small
	// YAML files this is simpler and prevents stale keys from older versions from
	// lingering after the schema changes.
	return os.WriteFile(path, data, 0o644)
}

func defaultJobs() []Job {
	return []Job{
		{
			ID:       1,
			Name:     "Hello scheduler",
			Folder:   "Examples",
			Schedule: "@every 1m",
			Command:  echoCommand("PySentry test job: scheduler is alive"),
			Enabled:  true,
		},
		{
			ID:       2,
			Name:     "Write timestamp",
			Folder:   "Examples",
			Schedule: "*/1 * * * *",
			Command:  echoCommand("PySentry test job: timestamp command ran"),
			Enabled:  true,
		},
		{
			ID:       3,
			Name:     "Paused sample",
			Schedule: "@every 1m",
			Command:  echoCommand("This paused sample should not run until enabled"),
			Enabled:  false,
		},
	}
}

func echoCommand(message string) string {
	if runtime.GOOS == "windows" {
		return "echo " + message
	}
	// POSIX shells need quotes for messages with spaces. Single quotes inside the
	// message are escaped using the standard close-quote/backslash/reopen pattern.
	return "echo '" + strings.ReplaceAll(message, "'", "'\\''") + "'"
}
