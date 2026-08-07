package storage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

type Store struct {
	Paths  Paths
	Config domain.Config
}

// PeekKeepRunningInTray reads keep_running_in_tray from gosentry.json for startup
// decisions that must run before app.Open(). On error it returns the built-in
// default.
//
// Despite the name, this can write: loadOrCreateConfig creates gosentry.json
// with defaults on first run, the same as OpenStore does moments later when
// app.Open() parses the now-existing file again. The double parse and the
// write-on-read are both harmless — the second read just sees the file the
// first one created — but worth knowing before adding a third startup path
// that also wants an early look at the config.
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

// PrepareSaveConfig re-resolves the derived paths from the current config and
// snapshots everything the write needs, returning the write itself as a closure.
// It exists so a caller that guards the Store with its own lock can do the file
// I/O — a marshal, an fsync, and a rename — after releasing that lock: the
// snapshot cannot change under the closure, so running it unlocked is safe.
// Prepared writes must be run in the order they were prepared, or an older
// snapshot can land on top of a newer one.
func (s *Store) PrepareSaveConfig() func() error {
	s.applyConfigPaths()
	dir := s.Paths.AppDir
	path := s.Paths.ConfigPath
	config := s.Config
	return func() error {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		return writeJSON(path, config)
	}
}

// PrepareSaveJobs is PrepareSaveConfig for the jobs file. The jobs slice is
// copied, so the caller may keep mutating its own slice as soon as this returns.
func (s *Store) PrepareSaveJobs(jobs []domain.Job) func() error {
	dir := s.Paths.JobsDir
	path := s.Paths.JobsPath
	snapshot := make([]domain.Job, len(jobs))
	copy(snapshot, jobs)
	return func() error {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		return writeJSON(path, domain.JobsFile{Jobs: snapshot})
	}
}

func (s *Store) SaveConfig() error {
	return s.PrepareSaveConfig()()
}

func (s *Store) SaveJobs(jobs []domain.Job) error {
	return s.PrepareSaveJobs(jobs)()
}

// ResolveConfiguredPath turns a file or directory path from the config into the
// absolute path the application will actually use. It is exported so callers
// outside storage — the settings tab, which opens the configured logs folder —
// apply the same rule to a path the user has typed but not yet saved.
func ResolveConfiguredPath(appDir string, path string) string {
	if filepath.IsAbs(path) {
		// Cleaned so two spellings of the same file (forward vs. backslashes, a
		// trailing separator) resolve to the same string. UpdateSettings compares
		// this against Paths.JobsPath to decide whether the jobs file is changing,
		// so an uncleaned path here could trigger a spurious adoption against the
		// file the app is already using.
		return filepath.Clean(path)
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
