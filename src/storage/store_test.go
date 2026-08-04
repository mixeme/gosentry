package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

func TestJobsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")

	original := []domain.Job{
		{
			ID:        7,
			Name:      "Backup data",
			Folder:    "Maintenance",
			Schedule:  "0 2 * * *",
			Command:   "/usr/bin/backup",
			Arguments: "--compress\n--verbose",
			StartOnly: true,
			Enabled:   true,
		},
	}

	if err := writeJSON(path, domain.JobsFile{Jobs: original}); err != nil {
		t.Fatal(err)
	}

	got, err := loadOrCreateJobs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 job, got %d", len(got))
	}

	g, w := got[0], original[0]
	if g.ID != w.ID {
		t.Errorf("ID: got %d, want %d", g.ID, w.ID)
	}
	if g.Name != w.Name {
		t.Errorf("Name: got %q, want %q", g.Name, w.Name)
	}
	if g.Folder != w.Folder {
		t.Errorf("Folder: got %q, want %q", g.Folder, w.Folder)
	}
	if g.Schedule != w.Schedule {
		t.Errorf("Schedule: got %q, want %q", g.Schedule, w.Schedule)
	}
	if g.Command != w.Command {
		t.Errorf("Command: got %q, want %q", g.Command, w.Command)
	}
	if g.Arguments != w.Arguments {
		t.Errorf("Arguments: got %q, want %q", g.Arguments, w.Arguments)
	}
	if g.StartOnly != w.StartOnly {
		t.Errorf("StartOnly: got %v, want %v", g.StartOnly, w.StartOnly)
	}
	if g.Enabled != w.Enabled {
		t.Errorf("Enabled: got %v, want %v", g.Enabled, w.Enabled)
	}

	// Runtime state no longer lives on Job at all (it moved to domain.JobRuntime),
	// so there is nothing transient that could survive the save→load round-trip.
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		AppDir:     dir,
		ConfigPath: filepath.Join(dir, ConfigFileName),
	}

	want := domain.Config{
		JobsFile:          "/custom/jobs/team.json",
		LogsDir:           "/custom/logs",
		MaxLogFiles:       50,
		MaxLogAgeDays:     14,
		StartOnLogin:      true,
		KeepRunningInTray: true,
		NotifyOnFailure:   true,
	}
	if err := writeJSON(paths.ConfigPath, want); err != nil {
		t.Fatal(err)
	}

	got, err := loadOrCreateConfig(paths)
	if err != nil {
		t.Fatal(err)
	}

	if got.JobsFile != want.JobsFile {
		t.Errorf("JobsFile: got %q, want %q", got.JobsFile, want.JobsFile)
	}
	if got.LogsDir != want.LogsDir {
		t.Errorf("LogsDir: got %q, want %q", got.LogsDir, want.LogsDir)
	}
	if got.MaxLogFiles != want.MaxLogFiles {
		t.Errorf("MaxLogFiles: got %d, want %d", got.MaxLogFiles, want.MaxLogFiles)
	}
	if got.MaxLogAgeDays != want.MaxLogAgeDays {
		t.Errorf("MaxLogAgeDays: got %d, want %d", got.MaxLogAgeDays, want.MaxLogAgeDays)
	}
	if got.StartOnLogin != want.StartOnLogin {
		t.Errorf("StartOnLogin: got %v, want %v", got.StartOnLogin, want.StartOnLogin)
	}
	if got.KeepRunningInTray != want.KeepRunningInTray {
		t.Errorf("KeepRunningInTray: got %v, want %v", got.KeepRunningInTray, want.KeepRunningInTray)
	}
	if got.NotifyOnFailure != want.NotifyOnFailure {
		t.Errorf("NotifyOnFailure: got %v, want %v", got.NotifyOnFailure, want.NotifyOnFailure)
	}
}

func TestNormalizeJobsFillsDefaults(t *testing.T) {
	jobs := []domain.Job{
		{Enabled: true},
		{Enabled: false},
		{ID: 5, Name: "Kept", Schedule: "*/10 * * * *", Enabled: true},
	}

	normalizeJobs(jobs)

	// Blank enabled job gets default name, schedule, and command.
	// normalizeJobs only fills durable configuration now; runtime status is built
	// separately by domain.NewRuntime.
	if jobs[0].ID != 1 {
		t.Errorf("first auto ID: got %d, want 1", jobs[0].ID)
	}
	if jobs[0].Name != "Untitled job" {
		t.Errorf("default name: got %q, want 'Untitled job'", jobs[0].Name)
	}
	if jobs[0].Schedule != "@every 1m" {
		t.Errorf("default schedule: got %q, want '@every 1m'", jobs[0].Schedule)
	}

	// Pre-set fields survive normalization unchanged.
	if jobs[2].ID != 5 {
		t.Errorf("pre-set ID should be preserved: got %d, want 5", jobs[2].ID)
	}
}

func TestLoadOrCreateConfigCreatesDefaultsOnFirstRun(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		AppDir:     dir,
		ConfigPath: filepath.Join(dir, ConfigFileName),
	}

	got, err := loadOrCreateConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if got.JobsFile != "jobs.json" {
		t.Errorf("default JobsFile = %q, want 'jobs.json'", got.JobsFile)
	}
	if got.LogsDir != "logs" {
		t.Errorf("default LogsDir = %q, want 'logs'", got.LogsDir)
	}
	if got.MaxLogFiles != 100 {
		t.Errorf("default MaxLogFiles = %d, want 100", got.MaxLogFiles)
	}
	if got.MaxLogAgeDays != 30 {
		t.Errorf("default MaxLogAgeDays = %d, want 30", got.MaxLogAgeDays)
	}
	if got.DefaultTimeoutSeconds != 0 {
		t.Errorf("default DefaultTimeoutSeconds = %d, want 0 (no timeout)", got.DefaultTimeoutSeconds)
	}
	if got.Theme != domain.ThemeGoSentry {
		t.Errorf("default Theme = %q, want %q", got.Theme, domain.ThemeGoSentry)
	}
	if got.JobListView != domain.JobListViewDetailed {
		t.Errorf("default JobListView = %q, want %q", got.JobListView, domain.JobListViewDetailed)
	}
	// The function must have written the defaults to gosentry.json.
	if _, err := os.Stat(paths.ConfigPath); err != nil {
		t.Errorf("gosentry.json should have been created: %v", err)
	}
}

// TestLoadOrCreateConfigKeepsZeroTimeoutOnReload guards the "0 = no timeout"
// setting against being normalized away when an existing gosentry.json is read
// back. Loading must not treat 0 as a missing value.
func TestLoadOrCreateConfigKeepsZeroTimeoutOnReload(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		AppDir:     dir,
		ConfigPath: filepath.Join(dir, ConfigFileName),
	}

	// First call writes the defaults (DefaultTimeoutSeconds = 0) to disk.
	if _, err := loadOrCreateConfig(paths); err != nil {
		t.Fatal(err)
	}
	// Second call takes the "file exists" branch, where normalization runs.
	reloaded, err := loadOrCreateConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.DefaultTimeoutSeconds != 0 {
		t.Errorf("reloaded DefaultTimeoutSeconds = %d, want 0 (no timeout)", reloaded.DefaultTimeoutSeconds)
	}
}

// TestLoadOrCreateConfigMigratesJobsDir covers a gosentry.json written before
// the setting named a file: the old jobs_dir keeps pointing at the same jobs
// file, and the retired key is dropped so it is not written back.
func TestLoadOrCreateConfigMigratesJobsDir(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		AppDir:     dir,
		ConfigPath: filepath.Join(dir, ConfigFileName),
	}
	legacy := map[string]any{
		"jobs_dir":         filepath.Join(dir, "shared"),
		"logs_dir":         "logs",
		"max_log_files":    100,
		"max_log_age_days": 30,
	}
	if err := writeJSON(paths.ConfigPath, legacy); err != nil {
		t.Fatal(err)
	}

	got, err := loadOrCreateConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "shared", JobsFileName)
	if got.JobsFile != want {
		t.Errorf("migrated JobsFile: got %q, want %q", got.JobsFile, want)
	}
	if got.JobsDir != "" {
		t.Errorf("legacy JobsDir should be cleared, got %q", got.JobsDir)
	}

	// The migrated config must not carry the retired key once it is saved.
	store := &Store{Paths: paths, Config: got}
	if err := store.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(paths.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "jobs_dir") {
		t.Errorf("saved config should not contain jobs_dir:\n%s", data)
	}
}

// TestLoadJobsFileReportsMissingWithoutCreating covers the loader the Settings
// tab uses to decide between adopting a jobs file and writing the current jobs
// to it: a missing file is reported as "not found" rather than an error, and —
// unlike the startup path — is not seeded with sample jobs.
func TestLoadJobsFileReportsMissingWithoutCreating(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nothing-here.json")

	jobs, found, err := LoadJobsFile(missing)
	if err != nil {
		t.Fatalf("missing file should not be an error: %v", err)
	}
	if found || jobs != nil {
		t.Errorf("missing file: got found=%v jobs=%+v, want false/nil", found, jobs)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Error("LoadJobsFile must not create the file it was asked about")
	}

	// An existing file comes back normalized, so a hand-written jobs file gains
	// its IDs and defaults before the application adopts it.
	path := filepath.Join(dir, "hand-written.json")
	if err := writeJSON(path, domain.JobsFile{Jobs: []domain.Job{{Name: "No ID"}}}); err != nil {
		t.Fatal(err)
	}
	jobs, found, err = LoadJobsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !found || len(jobs) != 1 {
		t.Fatalf("existing file: got found=%v jobs=%+v, want true and one job", found, jobs)
	}
	if jobs[0].ID != 1 || jobs[0].Schedule == "" || jobs[0].Command == "" {
		t.Errorf("loaded job should be normalized, got %+v", jobs[0])
	}
}

// TestApplyConfigPathsDerivesJobsDir checks that the jobs file drives both
// resolved paths: relative values resolve against the program folder, and the
// containing directory comes from the file name the user chose.
func TestApplyConfigPathsDerivesJobsDir(t *testing.T) {
	dir := t.TempDir()
	store := &Store{
		Paths:  Paths{AppDir: dir},
		Config: domain.Config{JobsFile: filepath.Join("shared", "team.json"), LogsDir: "logs"},
	}

	store.applyConfigPaths()

	if want := filepath.Join(dir, "shared", "team.json"); store.Paths.JobsPath != want {
		t.Errorf("JobsPath: got %q, want %q", store.Paths.JobsPath, want)
	}
	if want := filepath.Join(dir, "shared"); store.Paths.JobsDir != want {
		t.Errorf("JobsDir: got %q, want %q", store.Paths.JobsDir, want)
	}
}

// TestJobTimeoutRoundTripsThreeStates pins the on-disk encoding that keeps
// "inherit" and "no timeout" distinguishable: nil is omitted entirely, while an
// explicit 0 is written and read back as a set value.
func TestJobTimeoutRoundTripsThreeStates(t *testing.T) {
	jobs := []domain.Job{
		{ID: 1, Name: "Inherit", TimeoutSeconds: nil},
		{ID: 2, Name: "No timeout", TimeoutSeconds: domain.TimeoutSecondsPtr(0)},
		{ID: 3, Name: "Own", TimeoutSeconds: domain.TimeoutSecondsPtr(45)},
	}

	data, err := json.Marshal(domain.JobsFile{Jobs: jobs})
	if err != nil {
		t.Fatal(err)
	}
	if want := `"timeout_seconds":0`; !strings.Contains(string(data), want) {
		t.Fatalf("explicit zero timeout should be written as %s:\n%s", want, data)
	}

	var got domain.JobsFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Jobs[0].TimeoutSeconds != nil {
		t.Errorf("unset timeout should stay nil, got %d", *got.Jobs[0].TimeoutSeconds)
	}
	if got.Jobs[1].TimeoutSeconds == nil || *got.Jobs[1].TimeoutSeconds != 0 {
		t.Errorf("explicit zero timeout should survive the round trip, got %v", got.Jobs[1].TimeoutSeconds)
	}
	if got.Jobs[2].TimeoutSeconds == nil || *got.Jobs[2].TimeoutSeconds != 45 {
		t.Errorf("per-job timeout should survive the round trip, got %v", got.Jobs[2].TimeoutSeconds)
	}
}

func TestJobsJSONDoesNotPersistRuntimeNoise(t *testing.T) {
	// Job carries only durable configuration; runtime state lives in
	// domain.JobRuntime and is never marshalled. This guards against a future
	// runtime field accidentally being added back onto Job with a json tag.
	jobs := []domain.Job{
		{
			ID:       1,
			Name:     "Clean job",
			Schedule: "@every 10s",
			Command:  echoCommand("ok"),
			Enabled:  true,
		},
	}

	data, err := json.Marshal(domain.JobsFile{Jobs: jobs})
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, unwanted := range []string{"last_run", "next_run", "last_state", "activity", "last_output", "stdout"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("jobs json should not contain %q:\n%s", unwanted, text)
		}
	}
}
