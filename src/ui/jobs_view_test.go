package ui

import (
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

func TestFilterValue(t *testing.T) {
	cases := []struct{ input, want string }{
		{"", noFolder},
		{"   ", noFolder},
		{"Maintenance", "Maintenance"},
		{"  Reports  ", "Reports"},
	}
	for _, tc := range cases {
		if got := filterValue(tc.input); got != tc.want {
			t.Errorf("filterValue(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFolderOptionsAlwaysIncludesSentinels(t *testing.T) {
	opts := folderOptions(nil)
	if len(opts) < 2 || opts[0] != allFolders || opts[1] != noFolder {
		t.Errorf("folderOptions(nil) = %v, want [%q %q ...]", opts, allFolders, noFolder)
	}
}

func TestFolderOptionsAppendsUniqueFolders(t *testing.T) {
	jobs := []domain.Job{
		{Folder: "Maintenance"},
		{Folder: ""},           // no folder → not a named folder
		{Folder: "  Backups "}, // trimmed to "Backups"
		{Folder: "Maintenance"}, // duplicate → not added again
	}
	opts := folderOptions(jobs)
	// Expected: All, No folder, Maintenance, Backups — 4 entries, no duplicates.
	if len(opts) != 4 {
		t.Errorf("expected 4 options, got %v", opts)
	}
	has := map[string]bool{}
	for _, o := range opts {
		has[o] = true
	}
	for _, want := range []string{allFolders, noFolder, "Maintenance", "Backups"} {
		if !has[want] {
			t.Errorf("expected option %q in %v", want, opts)
		}
	}
}

func TestFilteredJobIndexesAll(t *testing.T) {
	jobs := []domain.Job{
		{Folder: "Maintenance"},
		{Folder: ""},
		{Folder: "Reports"},
	}
	got := filteredJobIndexes(jobs, allFolders)
	if len(got) != 3 {
		t.Errorf("allFolders filter: got %d indexes, want 3", len(got))
	}
}

func TestFilteredJobIndexesByNamedFolder(t *testing.T) {
	jobs := []domain.Job{
		{Folder: "Maintenance"}, // index 0
		{Folder: ""},            // index 1
		{Folder: "Maintenance"}, // index 2
		{Folder: "Reports"},     // index 3
	}
	got := filteredJobIndexes(jobs, "Maintenance")
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("Maintenance filter: got %v, want [0 2]", got)
	}
}

func TestFilteredJobIndexesNoFolder(t *testing.T) {
	jobs := []domain.Job{
		{Folder: "Maintenance"}, // index 0 — excluded
		{Folder: ""},            // index 1 — no folder → included
		{Folder: "  "},          // index 2 — blank → included
	}
	got := filteredJobIndexes(jobs, noFolder)
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("noFolder filter: got %v, want [1 2]", got)
	}
}

func TestFilteredJobIndexesEmptySlice(t *testing.T) {
	got := filteredJobIndexes(nil, allFolders)
	if len(got) != 0 {
		t.Errorf("empty job list should return empty indexes, got %v", got)
	}
}
