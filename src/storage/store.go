package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"go.yaml.in/yaml/v4"
)

type Store struct {
	Paths  Paths
	Config domain.Config
}

// yamlConfig and yamlJob / yamlJobsFile mirror the durable domain types using the
// yaml tags that pre-JSON-migration files carried. They exist only so the
// one-time import can parse a legacy gosentry.yaml / jobs.yaml; the domain types
// themselves stay JSON-only. Field layout must stay identical to the matching
// domain struct so the value conversions in importYAMLConfig / importYAMLJobs
// remain valid.
type yamlConfig struct {
	JobsDir           string `yaml:"jobs_dir"`
	LogsDir           string `yaml:"logs_dir"`
	MaxLogFiles       int    `yaml:"max_log_files"`
	MaxLogAgeDays     int    `yaml:"max_log_age_days"`
	StartOnLogin      bool   `yaml:"start_on_login,omitempty"`
	KeepRunningInTray bool   `yaml:"keep_running_in_tray,omitempty"`
	NotifyOnFailure   bool   `yaml:"notify_on_failure,omitempty"`
}

type yamlJob struct {
	ID               int    `yaml:"id"`
	Name             string `yaml:"name"`
	Folder           string `yaml:"folder,omitempty"`
	Schedule         string `yaml:"schedule"`
	Command          string `yaml:"command"`
	Arguments        string `yaml:"arguments,omitempty"`
	StartOnly bool `yaml:"start_only,omitempty"`
	Enabled          bool   `yaml:"enabled"`
}

type yamlJobsFile struct {
	Jobs []yamlJob `yaml:"jobs"`
}

func OpenStore() (*Store, []domain.Job, error) {
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
	// Jobs are also rewritten after normalization. That keeps jobs.json compact:
	// only durable job definitions remain, because runtime fields are tagged
	// json:"-" in the model.
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
	return writeJSON(s.Paths.ConfigPath, s.Config)
}

func (s *Store) SaveJobs(jobs []domain.Job) error {
	if err := os.MkdirAll(s.Paths.JobsDir, 0o755); err != nil {
		return err
	}
	return writeJSON(s.Paths.JobsPath, domain.JobsFile{Jobs: jobs})
}

func loadOrCreateConfig(paths Paths) (domain.Config, error) {
	// Defaults favor a portable installation: settings and jobs begin next to the
	// executable, while logs are grouped under a dedicated subdirectory.
	config := domain.Config{
		JobsDir:           ".",
		LogsDir:           "logs",
		MaxLogFiles:       100,
		MaxLogAgeDays:     30,
		StartOnLogin:      false,
		KeepRunningInTray: true,
		NotifyOnFailure:   true,
	}

	if _, err := os.Stat(paths.ConfigPath); errors.Is(err, os.ErrNotExist) {
		// No JSON config yet. Import a pre-migration gosentry.yaml once if it is
		// present; otherwise write the defaults so later starts read a normal JSON
		// file. The caller's SaveConfig rewrites whatever is loaded as gosentry.json.
		legacyPath := filepath.Join(paths.AppDir, legacyYAMLConfigFileName)
		imported, ok, err := importYAMLConfig(legacyPath, config)
		if err != nil {
			return domain.Config{}, err
		}
		if !ok {
			return config, writeJSON(paths.ConfigPath, config)
		}
		config = imported
	} else {
		data, err := os.ReadFile(paths.ConfigPath)
		if err != nil {
			return domain.Config{}, err
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return domain.Config{}, err
		}
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

func loadOrCreateJobs(path string) ([]domain.Job, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		// No JSON jobs file yet. Import a pre-migration jobs.yaml once if present;
		// otherwise seed harmless sample jobs so a new user can immediately see
		// scheduled and manual execution without inventing a command. Imported jobs
		// are returned unsaved here — the caller's SaveJobs rewrites them as
		// jobs.json after normalization.
		legacyPath := filepath.Join(filepath.Dir(path), legacyYAMLJobsFileName)
		imported, ok, err := importYAMLJobs(legacyPath)
		if err != nil {
			return nil, err
		}
		if ok {
			return imported, nil
		}
		jobs := defaultJobs()
		normalizeJobs(jobs)
		return jobs, writeJSON(path, domain.JobsFile{Jobs: jobs})
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file domain.JobsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return file.Jobs, nil
}

// importYAMLConfig reads a pre-migration gosentry.yaml into the current Config
// shape. It returns ok=false when the file is absent so the caller falls back to
// writing fresh defaults. The supplied base seeds the shadow struct so keys that
// the YAML omits keep their default value instead of becoming zero.
func importYAMLConfig(path string, base domain.Config) (domain.Config, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.Config{}, false, nil
	}
	if err != nil {
		return domain.Config{}, false, err
	}
	shadow := yamlConfig(base)
	if err := yaml.Unmarshal(data, &shadow); err != nil {
		return domain.Config{}, false, err
	}
	return domain.Config(shadow), true, nil
}

// importYAMLJobs reads a pre-migration jobs.yaml into durable domain jobs. It
// returns ok=false when the file is absent so the caller can seed default jobs.
func importYAMLJobs(path string) ([]domain.Job, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var file yamlJobsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, false, err
	}
	jobs := make([]domain.Job, len(file.Jobs))
	for i := range file.Jobs {
		jobs[i] = domain.Job(file.Jobs[i])
	}
	return jobs, true, nil
}

func normalizeJobs(jobs []domain.Job) {
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
			job.Command = echoCommand("GoSentry job ran")
		}
		job.Arguments = strings.TrimSpace(job.Arguments)
		// Runtime state (last run, next run, status, output, activity) is no longer
		// part of Job. It is reconstructed each time the app starts via
		// domain.NewRuntime, so normalizeJobs only touches durable configuration.
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

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	// A trailing newline keeps the file friendly to editors and diff tools that
	// expect text files to end with one.
	data = append(data, '\n')
	// WriteFile replaces the full file instead of patching it in place. For small
	// JSON files this is simpler and prevents stale keys from older versions from
	// lingering after the schema changes.
	return os.WriteFile(path, data, 0o644)
}

func defaultJobs() []domain.Job {
	return []domain.Job{
		{
			ID:       1,
			Name:     "Hello scheduler",
			Folder:   "Examples",
			Schedule: "@every 1m",
			Command:  echoCommand("GoSentry test job: scheduler is alive"),
			Enabled:  true,
		},
		{
			ID:       2,
			Name:     "Write timestamp",
			Folder:   "Examples",
			Schedule: "*/1 * * * *",
			Command:  echoCommand("GoSentry test job: timestamp command ran"),
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
