package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"go.yaml.in/yaml/v4"
)

func TestJobsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.yaml")

	original := []domain.Job{
		{
			ID:               7,
			Name:             "Backup data",
			Folder:           "Maintenance",
			Schedule:         "0 2 * * *",
			Command:          "/usr/bin/backup",
			Arguments:        "--compress\n--verbose",
			SuccessExitCodes: "0,1",
			StartOnly:        true,
			Enabled:          true,
		},
	}

	if err := writeYAML(path, domain.JobsFile{Jobs: original}); err != nil {
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
	if g.SuccessExitCodes != w.SuccessExitCodes {
		t.Errorf("SuccessExitCodes: got %q, want %q", g.SuccessExitCodes, w.SuccessExitCodes)
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
		JobsDir:           "/custom/jobs",
		LogsDir:           "/custom/logs",
		MaxLogFiles:       50,
		MaxLogAgeDays:     14,
		StartOnLogin:      true,
		KeepRunningInTray: false,
		NotifyOnFailure:   false,
	}
	if err := writeYAML(paths.ConfigPath, want); err != nil {
		t.Fatal(err)
	}

	got, err := loadOrCreateConfig(paths)
	if err != nil {
		t.Fatal(err)
	}

	if got.JobsDir != want.JobsDir {
		t.Errorf("JobsDir: got %q, want %q", got.JobsDir, want.JobsDir)
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
		{ID: 5, Name: "Kept", Schedule: "*/10 * * * *", SuccessExitCodes: "0,1", Enabled: true},
	}

	normalizeJobs(jobs)

	// Blank enabled job gets default name, schedule, command, and exit codes.
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
	if jobs[0].SuccessExitCodes != "0" {
		t.Errorf("default exit codes: got %q, want '0'", jobs[0].SuccessExitCodes)
	}

	// Pre-set fields survive normalization unchanged.
	if jobs[2].ID != 5 {
		t.Errorf("pre-set ID should be preserved: got %d, want 5", jobs[2].ID)
	}
	if jobs[2].SuccessExitCodes != "0,1" {
		t.Errorf("pre-set exit codes should be preserved: got %q, want '0,1'", jobs[2].SuccessExitCodes)
	}
}

// TestLoadOrCreateConfigMigratesFromLegacy verifies that when gosentry.yaml is
// absent but pysentry.yaml exists the config is read from the legacy file. This
// lets portable installs that still carry a pysentry.yaml start without manual
// migration.
func TestLoadOrCreateConfigMigratesFromLegacy(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		AppDir:     dir,
		ConfigPath: filepath.Join(dir, ConfigFileName), // gosentry.yaml — not created
	}

	legacy := domain.Config{
		JobsDir:       "/legacy/jobs",
		LogsDir:       "/legacy/logs",
		MaxLogFiles:   77,
		MaxLogAgeDays: 13,
		StartOnLogin:  true,
	}
	if err := writeYAML(filepath.Join(dir, LegacyConfigFileName), legacy); err != nil {
		t.Fatal(err)
	}

	got, err := loadOrCreateConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if got.JobsDir != legacy.JobsDir {
		t.Errorf("JobsDir: got %q, want %q", got.JobsDir, legacy.JobsDir)
	}
	if got.LogsDir != legacy.LogsDir {
		t.Errorf("LogsDir: got %q, want %q", got.LogsDir, legacy.LogsDir)
	}
	if got.MaxLogFiles != legacy.MaxLogFiles {
		t.Errorf("MaxLogFiles: got %d, want %d", got.MaxLogFiles, legacy.MaxLogFiles)
	}
	if got.MaxLogAgeDays != legacy.MaxLogAgeDays {
		t.Errorf("MaxLogAgeDays: got %d, want %d", got.MaxLogAgeDays, legacy.MaxLogAgeDays)
	}
	if got.StartOnLogin != legacy.StartOnLogin {
		t.Errorf("StartOnLogin: got %v, want %v", got.StartOnLogin, legacy.StartOnLogin)
	}
}

// TestLoadOrCreateConfigCreatesDefaultsOnFirstRun verifies that the first run
// (no config files present) writes gosentry.yaml and returns sensible defaults.
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
	if got.JobsDir != "." {
		t.Errorf("default JobsDir = %q, want '.'", got.JobsDir)
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
	// The function must have written the defaults to gosentry.yaml.
	if _, err := os.Stat(paths.ConfigPath); err != nil {
		t.Errorf("gosentry.yaml should have been created: %v", err)
	}
}

func TestJobsYAMLDoesNotPersistRuntimeNoise(t *testing.T) {
	// Job carries only durable configuration; runtime state lives in
	// domain.JobRuntime and is never marshalled. This guards against a future
	// runtime field accidentally being added back onto Job with a yaml tag.
	jobs := []domain.Job{
		{
			ID:       1,
			Name:     "Clean job",
			Schedule: "@every 10s",
			Command:  echoCommand("ok"),
			Enabled:  true,
		},
	}

	data, err := yaml.Marshal(domain.JobsFile{Jobs: jobs})
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, unwanted := range []string{"last_run", "next_run", "last_state", "activity", "last_output", "stdout"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("jobs yaml should not contain %q:\n%s", unwanted, text)
		}
	}
}
