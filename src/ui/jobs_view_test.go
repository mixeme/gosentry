package ui

import (
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
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

func TestNextJobListViewFlipsBothWays(t *testing.T) {
	cases := []struct {
		current, want domain.JobListView
	}{
		{domain.JobListViewDetailed, domain.JobListViewCompact},
		{domain.JobListViewCompact, domain.JobListViewDetailed},
		// Empty and unknown values read as detailed, so they flip to compact.
		{"", domain.JobListViewCompact},
		{"tiny", domain.JobListViewCompact},
	}
	for _, tc := range cases {
		if got := nextJobListView(tc.current); got != tc.want {
			t.Errorf("nextJobListView(%q) = %q, want %q", tc.current, got, tc.want)
		}
	}
}

// findFirst walks a widget tree depth-first and returns the first object the
// match function accepts. Tests use it to reach widgets newJobsView builds
// internally rather than returning.
func findFirst(root fyne.CanvasObject, match func(fyne.CanvasObject) bool) fyne.CanvasObject {
	if match(root) {
		return root
	}
	container, ok := root.(*fyne.Container)
	if !ok {
		return nil
	}
	for _, child := range container.Objects {
		if found := findFirst(child, match); found != nil {
			return found
		}
	}
	return nil
}

// jobsSidebar narrows the search to the left pane. The details panel has a
// widget.List of its own (the activity log), so a search from the whole view
// would find the wrong one.
func jobsSidebar(t *testing.T, content fyne.CanvasObject) fyne.CanvasObject {
	t.Helper()
	found := findFirst(content, func(o fyne.CanvasObject) bool {
		wrapper, ok := o.(*fyne.Container)
		if !ok {
			return false
		}
		_, ok = wrapper.Layout.(minWidthLayout)
		return ok
	})
	if found == nil {
		t.Fatal("jobs view has no fixed-width sidebar")
	}
	return found
}

func jobsList(t *testing.T, content fyne.CanvasObject) *widget.List {
	t.Helper()
	found := findFirst(jobsSidebar(t, content), func(o fyne.CanvasObject) bool {
		_, ok := o.(*widget.List)
		return ok
	})
	if found == nil {
		t.Fatal("jobs sidebar contains no list widget")
	}
	return found.(*widget.List)
}

func jobsViewToggle(t *testing.T, content fyne.CanvasObject) *widget.Button {
	t.Helper()
	found := findFirst(jobsSidebar(t, content), func(o fyne.CanvasObject) bool {
		button, ok := o.(*widget.Button)
		return ok && (button.Text == "Compact" || button.Text == "Detailed")
	})
	if found == nil {
		t.Fatal("jobs sidebar has no view toggle button")
	}
	return found.(*widget.Button)
}

// TestJobListViewToggleShrinksRowsAndPersists is the end-to-end guard for the
// compact view: one tap must shrink the list rows, relabel the button with the
// opposite action, and reach the config — and toggling back must undo all
// three. Row height is measured through List.CreateItem/UpdateItem because
// that is exactly what widget.List caches as the row height.
func TestJobListViewToggleShrinksRowsAndPersists(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()
	w := testApp.NewWindow("test")
	defer w.Close()

	store := newTestStore(t)
	jobs := []domain.Job{
		{ID: 1, Name: "Nightly backup", Folder: "Maintenance", Schedule: "@every 1m", Command: "echo hi", Enabled: true},
	}
	svc := app.NewService(store, jobs)
	defer svc.Stop()

	content, _ := newJobsView(w, svc)
	w.SetContent(content)

	list := jobsList(t, content)
	viewButton := jobsViewToggle(t, content)
	if viewButton.Text != "Compact" {
		t.Fatalf("a default config should open detailed: button text = %q, want %q", viewButton.Text, "Compact")
	}

	rowHeight := func() float32 {
		t.Helper()
		row := list.CreateItem()
		list.UpdateItem(0, row)
		return row.MinSize().Height
	}
	detailedHeight := rowHeight()

	test.Tap(viewButton)
	if store.Config.JobListView != domain.JobListViewCompact {
		t.Errorf("after tapping, Config.JobListView = %q, want %q", store.Config.JobListView, domain.JobListViewCompact)
	}
	if viewButton.Text != "Detailed" {
		t.Errorf("after tapping, button text = %q, want %q", viewButton.Text, "Detailed")
	}
	compactHeight := rowHeight()
	if compactHeight >= detailedHeight {
		t.Errorf("compact row height = %v, want less than detailed %v", compactHeight, detailedHeight)
	}

	test.Tap(viewButton)
	if store.Config.JobListView != domain.JobListViewDetailed {
		t.Errorf("after tapping back, Config.JobListView = %q, want %q", store.Config.JobListView, domain.JobListViewDetailed)
	}
	if viewButton.Text != "Compact" {
		t.Errorf("after tapping back, button text = %q, want %q", viewButton.Text, "Compact")
	}
	if got := rowHeight(); got != detailedHeight {
		t.Errorf("row height after switching back = %v, want the original %v", got, detailedHeight)
	}
}

// TestJobListViewCompactConfigOpensCompact checks the persisted preference is
// honoured at build time, not just after a tap.
func TestJobListViewCompactConfigOpensCompact(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()
	w := testApp.NewWindow("test")
	defer w.Close()

	store := newTestStore(t)
	store.Config.JobListView = domain.JobListViewCompact
	svc := app.NewService(store, nil)
	defer svc.Stop()

	content, _ := newJobsView(w, svc)
	w.SetContent(content)

	if got := jobsViewToggle(t, content).Text; got != "Detailed" {
		t.Errorf("button text for a compact config = %q, want %q", got, "Detailed")
	}
}

func TestViewToggleTextNamesTheAction(t *testing.T) {
	cases := []struct {
		current domain.JobListView
		want    string
	}{
		{domain.JobListViewDetailed, "Compact"},
		{domain.JobListViewCompact, "Detailed"},
		{"", "Compact"},
		{"tiny", "Compact"},
	}
	for _, tc := range cases {
		if got := viewToggleText(tc.current); got != tc.want {
			t.Errorf("viewToggleText(%q) = %q, want %q", tc.current, got, tc.want)
		}
	}
}
