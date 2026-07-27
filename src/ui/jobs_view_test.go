package ui

import (
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
		{Folder: ""},            // no folder → not a named folder
		{Folder: "  Backups "},  // trimmed to "Backups"
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

// jobsSplit returns the view's master/detail split. newJobsView assembles the
// panel as container.NewHSplit(sidebar, details), so the two panes are reached
// through Leading and Trailing.
func jobsSplit(t *testing.T, content fyne.CanvasObject) *container.Split {
	t.Helper()
	split, ok := content.(*container.Split)
	if !ok {
		t.Fatal("jobs view is not the expected Split container")
	}
	return split
}

// jobsSidebar narrows the search to the left pane. The details panel has a
// widget.List of its own (the activity log), so a search from the whole view
// would find the wrong one.
func jobsSidebar(t *testing.T, content fyne.CanvasObject) fyne.CanvasObject {
	t.Helper()
	return jobsSplit(t, content).Leading
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

// jobsToolbar finds the add/edit/run/pause/delete button row inside the
// sidebar, identified by its first child being the "New job" button.
func jobsToolbar(t *testing.T, content fyne.CanvasObject) fyne.CanvasObject {
	t.Helper()
	found := findFirst(jobsSidebar(t, content), func(o fyne.CanvasObject) bool {
		wrapper, ok := o.(*fyne.Container)
		if !ok || len(wrapper.Objects) == 0 {
			return false
		}
		button, ok := wrapper.Objects[0].(*widget.Button)
		return ok && button.Text == "New job"
	})
	if found == nil {
		t.Fatal("jobs sidebar has no toolbar row")
	}
	return found
}

// jobsToolbarButton returns the toolbar button with the given caption.
func jobsToolbarButton(t *testing.T, content fyne.CanvasObject, text string) *widget.Button {
	t.Helper()
	found := findFirst(jobsToolbar(t, content), func(o fyne.CanvasObject) bool {
		button, ok := o.(*widget.Button)
		return ok && button.Text == text
	})
	if found == nil {
		t.Fatalf("jobs toolbar has no %q button", text)
	}
	return found.(*widget.Button)
}

// jobsDetails narrows the search to the right pane (see jobsSidebar).
func jobsDetails(t *testing.T, content fyne.CanvasObject) fyne.CanvasObject {
	t.Helper()
	return jobsSplit(t, content).Trailing
}

// jobsDetailsActivity returns the "Selected job activity" list, the only
// widget.List in the details pane.
func jobsDetailsActivity(t *testing.T, content fyne.CanvasObject) *widget.List {
	t.Helper()
	found := findFirst(jobsDetails(t, content), func(o fyne.CanvasObject) bool {
		_, ok := o.(*widget.List)
		return ok
	})
	if found == nil {
		t.Fatal("details pane has no activity list")
	}
	return found.(*widget.List)
}

// jobsDetailsTitle reads the details pane's heading, which detailsPanel builds
// as the first bold label in the pane.
func jobsDetailsTitle(t *testing.T, content fyne.CanvasObject) string {
	t.Helper()
	found := findFirst(jobsDetails(t, content), func(o fyne.CanvasObject) bool {
		label, ok := o.(*widget.Label)
		return ok && label.TextStyle.Bold
	})
	if found == nil {
		t.Fatal("details pane has no title label")
	}
	return found.(*widget.Label).Text
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

// TestJobsSidebarWidthIsItsContent is the regression guard for F7: nothing
// but the sidebar's own content (here, the toolbar row) should impose a
// width floor on it.
func TestJobsSidebarWidthIsItsContent(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()
	w := testApp.NewWindow("test")
	defer w.Close()

	store := newTestStore(t)
	svc := app.NewService(store, nil)
	defer svc.Stop()

	content, _ := newJobsView(w, svc)
	w.SetContent(content)

	sidebarWidth := jobsSidebar(t, content).MinSize().Width
	toolbarWidth := jobsToolbar(t, content).MinSize().Width
	if sidebarWidth != toolbarWidth {
		t.Errorf("sidebar MinSize().Width = %v, want it to equal the toolbar row's %v", sidebarWidth, toolbarWidth)
	}
}

// TestJobsSplitOpensAtTheSidebarWidth is the guard for the derived initial
// offset (F15): at the default window width the divider must open at the
// sidebar's own width — enough that the toolbar is never born clipped, and no
// more, since every extra pixel is taken from the details pane. Split's own
// clamp guarantees the lower bound, so the upper bound is what actually proves
// the offset was derived rather than left at the 0.5 default.
func TestJobsSplitOpensAtTheSidebarWidth(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()
	w := testApp.NewWindow("test")
	defer w.Close()

	store := newTestStore(t)
	svc := app.NewService(store, nil)
	defer svc.Stop()

	content, _ := newJobsView(w, svc)
	w.SetContent(content)

	split := jobsSplit(t, content)
	split.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))

	want := split.Leading.MinSize().Width
	got := split.Leading.Size().Width
	// One pixel of slack for the float32 round trip through the offset ratio.
	if got < want || got > want+1 {
		t.Errorf("leading pane opens at %v, want its content minimum %v", got, want)
	}
	if trailing := split.Trailing.Size().Width; trailing < split.Trailing.MinSize().Width {
		t.Errorf("trailing pane opens at %v, below its minimum %v", trailing, split.Trailing.MinSize().Width)
	}
}

// TestToolbarButtonRedrawsRowAndDetails is the regression guard for F12: the
// toolbar handlers no longer re-read the service or refresh the list
// themselves, so refreshView alone has to re-snapshot the jobs and repopulate
// the details pane. If it ever stops doing either, the row renders a stale
// status and the details lose the selection — neither is a compile error.
func TestToolbarButtonRedrawsRowAndDetails(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()
	w := testApp.NewWindow("test")
	defer w.Close()

	store := newTestStore(t)
	jobs := []domain.Job{
		{ID: 1, Name: "First", Schedule: "@every 1m", Command: "echo one", Enabled: true},
		{ID: 2, Name: "Second", Schedule: "@every 2m", Command: "echo two", Enabled: true},
	}
	svc := app.NewService(store, jobs)
	defer svc.Stop()

	content, _ := newJobsView(w, svc)
	w.SetContent(content)

	list := jobsList(t, content)
	// Row layout: VBox(nameLine, meta, status), nameLine = Border(name, inlineStatus).
	rowText := func(id int) (name string, status string) {
		t.Helper()
		row := list.CreateItem().(*fyne.Container)
		list.UpdateItem(id, row)
		nameLine := row.Objects[0].(*fyne.Container)
		return nameLine.Objects[0].(*widget.Label).Text, row.Objects[2].(*widget.Label).Text
	}

	activity := jobsDetailsActivity(t, content)

	list.Select(1)
	if got := jobsDetailsTitle(t, content); got != "Second" {
		t.Fatalf("details title after selecting row 1 = %q, want %q", got, "Second")
	}
	if _, status := rowText(1); status == "Paused" {
		t.Fatal("the second job should start enabled")
	}
	if got := activity.Length(); got != 0 {
		t.Fatalf("activity rows before the tap = %d, want 0", got)
	}

	test.Tap(jobsToolbarButton(t, content, "Pause"))

	if svc.Jobs()[1].Enabled {
		t.Fatal("tapping Pause did not reach the service")
	}
	name, status := rowText(1)
	if name != "Second" || status != "Paused" {
		t.Errorf("row 1 after Pause = (%q, %q), want (%q, %q)", name, status, "Second", "Paused")
	}
	if got := jobsDetailsTitle(t, content); got != "Second" {
		t.Errorf("details title after Pause = %q, want the selection kept at %q", got, "Second")
	}
	// The pause writes an activity record. Seeing it here is what proves
	// refreshView repopulated the details pane rather than leaving the panel on
	// the snapshot it held before the tap.
	if got := activity.Length(); got != 1 {
		t.Errorf("activity rows after the tap = %d, want the pause record", got)
	}
}

// TestDetailCaptionWidthCoversEveryCaption is the guard that makes the single
// metadataRows list self-enforcing (F10): every caption it returns must
// measure no wider than captionColumnWidth's result for that same list, or a
// row added to metadataRows without updating the width measurement would
// silently truncate.
func TestDetailCaptionWidthCoversEveryCaption(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	d := newDetailsPanel(job{}, &domain.JobRuntime{}, domain.OverlapPolicySkip, 0)
	specs := d.metadataRows()
	captions := make([]string, len(specs))
	for i, spec := range specs {
		captions[i] = spec.caption
	}
	capW := captionColumnWidth(captions...)
	for _, c := range captions {
		if w := widget.NewLabelWithStyle(c, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}).MinSize().Width; w > capW {
			t.Errorf("caption %q measures %v, wider than captionColumnWidth's %v", c, w, capW)
		}
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
