package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

type Store struct {
	Paths  Paths
	Config domain.Config
}

// PeekKeepRunningInTray reads keep_running_in_tray from gosentry.json for startup
// decisions that must run before app.Open(). On error it returns the built-in
// default.
func PeekKeepRunningInTray() bool {
	paths, err := ResolvePaths()
	if err != nil {
		return domain.DefaultConfig().KeepRunningInTray
	}
	config, err := loadOrCreateConfig(paths)
	if err != nil {
		return domain.DefaultConfig().KeepRunningInTray
	}
	return config.KeepRunningInTray
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
	config := domain.DefaultConfig()

	if _, err := os.Stat(paths.ConfigPath); errors.Is(err, os.ErrNotExist) {
		return config, writeJSON(paths.ConfigPath, config)
	}

	data, err := os.ReadFile(paths.ConfigPath)
	if err != nil {
		return domain.Config{}, err
	}
	// Clearing the default first keeps "the file sets jobs_file" distinguishable
	// from "the file omits it", which the jobs_dir migration below depends on.
	// The fallbacks restore a value in either case.
	config.JobsFile = ""
	if err := json.Unmarshal(data, &config); err != nil {
		return domain.Config{}, err
	}

	// A config written before the setting named a file carries jobs_dir instead
	// of jobs_file. Keep its meaning by appending the fixed name that version
	// used, then drop the old key so the file is rewritten in the current shape.
	if strings.TrimSpace(config.JobsFile) == "" && strings.TrimSpace(config.JobsDir) != "" {
		config.JobsFile = filepath.Join(config.JobsDir, JobsFileName)
	}
	config.JobsDir = ""
	if strings.TrimSpace(config.JobsFile) == "" {
		// Empty paths are treated as missing values rather than intentional root
		// directories. This avoids accidentally writing jobs to unexpected places.
		config.JobsFile = JobsFileName
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
	if config.ExecutionMode == "" {
		config.ExecutionMode = domain.ExecutionModeParallel
	}
	if config.OverlapPolicy == "" {
		config.OverlapPolicy = domain.OverlapPolicySkip
	}
	// DefaultTimeoutSeconds is deliberately not normalized: 0 is a meaningful
	// value ("no timeout"), not a missing one, so backfilling it here would make
	// the setting impossible to persist. Negative values are rejected by
	// app.validateConfig before they can be saved.
	if config.Theme == "" {
		config.Theme = domain.ThemeGoSentry
	}
	if config.Theme == "default" {
		config.Theme = domain.ThemeSystem
	}
	return config, nil
}

// LoadJobsFile reads and normalizes the job definitions at path. The bool
// reports whether the file was there: a missing file is not an error but the
// answer to "is this file already a jobs file?", which is what the Settings tab
// needs when the user points the application at a different jobs file.
func LoadJobsFile(path string) ([]domain.Job, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var file domain.JobsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, false, err
	}
	normalizeJobs(file.Jobs)
	return file.Jobs, true, nil
}

func loadOrCreateJobs(path string) ([]domain.Job, error) {
	jobs, found, err := LoadJobsFile(path)
	if err != nil {
		return nil, err
	}
	if found {
		return jobs, nil
	}
	// Seed sample jobs so a new user can immediately see scheduled and manual
	// execution without inventing a command. The failure sample stays disabled
	// so it does not spam notifications; Run now still works for testing.
	jobs = defaultJobs()
	normalizeJobs(jobs)
	return jobs, writeJSON(path, domain.JobsFile{Jobs: jobs})
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

// ResolveConfiguredPath turns a file or directory path from the config into the
// absolute path the application will actually use. It is exported so callers
// outside storage — the settings tab, which opens the configured logs folder —
// apply the same rule to a path the user has typed but not yet saved.
func ResolveConfiguredPath(appDir string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	// Relative paths are resolved against the executable directory, not the
	// process working directory. This matches ResolvePaths and keeps shortcuts,
	// Explorer launches, and terminal launches consistent.
	return filepath.Clean(filepath.Join(appDir, path))
}

func (s *Store) applyConfigPaths() {
	// The jobs file is configured as a whole path; its directory is derived so
	// SaveJobs can create the folder when the user points at a new location.
	s.Paths.JobsPath = ResolveConfiguredPath(s.Paths.AppDir, s.Config.JobsFile)
	s.Paths.JobsDir = filepath.Dir(s.Paths.JobsPath)
	s.Paths.LogsDir = ResolveConfiguredPath(s.Paths.AppDir, s.Config.LogsDir)
}

func writeJSON(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	// A trailing newline keeps the file friendly to editors and diff tools that
	// expect text files to end with one.
	data = append(data, '\n')
	return writeFileAtomic(dir, path, data, 0o644)
}

// writeFileAtomic writes data to a temp file in dir, syncs it, then renames it
// over path. Rename is atomic within a volume on both supported platforms, so
// a crash, a power loss, or the process being killed mid-write can never leave
// path holding a truncated or empty file the way a direct os.WriteFile could.
func writeFileAtomic(dir, path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	// Any failure past this point must remove the temp file rather than leave
	// it behind for the next write to trip over.
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	success = true
	return nil
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
		{
			ID:       4,
			Name:     "Failure notification test",
			Folder:   "Examples",
			Schedule: "@every 1m",
			Command:  failCommand(),
			Enabled:  false,
		},
	}
}

func failCommand() string {
	if runtime.GOOS == "windows" {
		return "exit /b 1"
	}
	return "exit 1"
}

func echoCommand(message string) string {
	if runtime.GOOS == "windows" {
		return "echo " + message
	}
	// POSIX shells need quotes for messages with spaces. Single quotes inside the
	// message are escaped using the standard close-quote/backslash/reopen pattern.
	return "echo '" + strings.ReplaceAll(message, "'", "'\\''") + "'"
}
