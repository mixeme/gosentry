package ui

import (
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

// newStateForTest builds a jobsViewState over a Service holding the given jobs.
// No Fyne app is needed: the state is the view's model and touches no widgets.
func newStateForTest(t *testing.T, jobs []domain.Job) (*jobsViewState, *app.Service) {
	t.Helper()
	svc := app.NewService(newTestStore(t), jobs)
	t.Cleanup(svc.Stop)
	return newJobsViewState(svc), svc
}

func threeJobs() []domain.Job {
	return []domain.Job{
		{ID: 1, Name: "First", Folder: "Maintenance", Schedule: "@every 1m", Command: "echo one", Enabled: true},
		{ID: 2, Name: "Second", Schedule: "@every 2m", Command: "echo two", Enabled: true},
		{ID: 3, Name: "Third", Folder: "Reports", Schedule: "@every 3m", Command: "echo three", Enabled: true},
	}
}

func selectedName(t *testing.T, s *jobsViewState) string {
	t.Helper()
	current, ok := s.selected()
	if !ok {
		return ""
	}
	return current.Name
}

// TestJobsViewStateSelectsTheFirstJob pins the opening state: the first row is
// selected so the details pane is never blank when there is something to show.
func TestJobsViewStateSelectsTheFirstJob(t *testing.T) {
	s, _ := newStateForTest(t, threeJobs())
	if got := selectedName(t, s); got != "First" {
		t.Errorf("selected job = %q, want %q", got, "First")
	}
	if got := s.displayRow(); got != 0 {
		t.Errorf("displayRow = %d, want 0", got)
	}
}

func TestJobsViewStateEmptyListSelectsNothing(t *testing.T) {
	s, _ := newStateForTest(t, nil)
	if _, ok := s.selected(); ok {
		t.Error("an empty job list should leave nothing selected")
	}
	if got := s.displayRow(); got != -1 {
		t.Errorf("displayRow with nothing selected = %d, want -1", got)
	}
}

// TestJobsViewStateSelectionFollowsTheJobNotTheRow is the regression guard for
// the selection defect: the selection is a job ID, so a job removed above the
// selected one must not slide the selection onto its neighbour. The deletion
// goes through the Service rather than the Delete button, which is how the view
// learns about a job list that changed underneath it (a different jobs file
// adopted, or any other broad JobChanged).
func TestJobsViewStateSelectionFollowsTheJobNotTheRow(t *testing.T) {
	s, svc := newStateForTest(t, threeJobs())

	s.selectRow(2)
	if got := selectedName(t, s); got != "Third" {
		t.Fatalf("selected job after selecting row 2 = %q, want %q", got, "Third")
	}

	if err := svc.DeleteJob(1); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	s.sync()

	if got := selectedName(t, s); got != "Third" {
		t.Errorf("selected job after the first job was removed = %q, want it still on %q", got, "Third")
	}
	if got := s.displayRow(); got != 1 {
		t.Errorf("displayRow = %d, want the row %q moved to (1)", got, "Third")
	}
}

// TestJobsViewStateDropsSelectionWhenItsJobIsGone covers the other half: a
// selected job that no longer exists falls back to the first visible row instead
// of describing whichever job inherited its position.
func TestJobsViewStateDropsSelectionWhenItsJobIsGone(t *testing.T) {
	s, svc := newStateForTest(t, threeJobs())

	s.selectRow(1)
	if err := svc.DeleteJob(2); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	s.sync()

	if got := selectedName(t, s); got != "First" {
		t.Errorf("selected job after deleting the selected one = %q, want the fallback %q", got, "First")
	}
}

func TestJobsViewStateApplyFilter(t *testing.T) {
	s, _ := newStateForTest(t, threeJobs())

	// The selected job is in the folder being filtered to, so it stays selected.
	s.selectRow(2)
	s.applyFilter("Reports")
	if got := selectedName(t, s); got != "Third" {
		t.Errorf("selection after filtering to its own folder = %q, want %q", got, "Third")
	}
	if got := s.displayRow(); got != 0 {
		t.Errorf("displayRow inside the filter = %d, want 0", got)
	}

	// Filtering to a folder that hides it moves the selection to the first row
	// that folder does show.
	s.applyFilter("Maintenance")
	if got := selectedName(t, s); got != "First" {
		t.Errorf("selection after filtering it away = %q, want %q", got, "First")
	}

	// "No folder" matches the one job without one.
	s.applyFilter(noFolder)
	if got := selectedName(t, s); got != "Second" {
		t.Errorf("selection under the %q filter = %q, want %q", noFolder, got, "Second")
	}

	s.applyFilter(allFolders)
	if got := len(s.filtered); got != 3 {
		t.Errorf("rows under %q = %d, want 3", allFolders, got)
	}
}

// TestJobsViewStateEmptyFilterSelectsNothing pins that a filter matching no job
// is a filter choice, not an error state: nothing is selected, and nothing is
// highlighted either.
func TestJobsViewStateEmptyFilterSelectsNothing(t *testing.T) {
	s, _ := newStateForTest(t, []domain.Job{
		{ID: 1, Name: "First", Folder: "Maintenance", Schedule: "@every 1m", Command: "echo one", Enabled: true},
	})

	s.applyFilter(noFolder)
	if _, ok := s.selected(); ok {
		t.Error("a filter that matches nothing should leave nothing selected")
	}
	if got := s.displayRow(); got != -1 {
		t.Errorf("displayRow under an empty filter = %d, want -1", got)
	}

	// The selection comes back when the filter does.
	s.applyFilter(allFolders)
	if got := selectedName(t, s); got != "First" {
		t.Errorf("selection after clearing the filter = %q, want %q", got, "First")
	}
}

// TestJobsViewStateHiddenSelectionIsNotHighlighted covers the case the list
// widget cannot express: the selected job still exists but the filter hides it,
// so there is no row to highlight and displayRow must say so rather than fall
// back to row 0.
func TestJobsViewStateHiddenSelectionIsNotHighlighted(t *testing.T) {
	s, _ := newStateForTest(t, threeJobs())

	s.applyFilter("Maintenance")
	// Selecting by ID is how the create handler points the view at a job it just
	// made; here it reaches the state a hidden-but-selected job would be in.
	s.selectByID(3)
	if s.visible(3) {
		t.Fatal("job 3 should be hidden by the Maintenance filter")
	}
	if got := s.displayRow(); got != -1 {
		t.Errorf("displayRow for a hidden selection = %d, want -1", got)
	}
	if got := selectedName(t, s); got != "Third" {
		t.Errorf("selected job = %q, want it still %q", got, "Third")
	}
}

func TestJobsViewStateRuntimeIsNeverNil(t *testing.T) {
	s, _ := newStateForTest(t, threeJobs())
	if rt := s.runtime(99); rt == nil {
		t.Error("runtime for an unknown job returned nil, want an empty runtime")
	}
}

func TestJobsViewStateJobAtRejectsRowsOutsideTheFilter(t *testing.T) {
	s, _ := newStateForTest(t, threeJobs())
	s.applyFilter("Reports")
	if current, ok := s.jobAt(0); !ok || current.Name != "Third" {
		t.Errorf("jobAt(0) = (%q, %v), want (%q, true)", current.Name, ok, "Third")
	}
	if _, ok := s.jobAt(1); ok {
		t.Error("jobAt past the last filtered row should report no job")
	}
	if _, ok := s.jobAt(-1); ok {
		t.Error("jobAt(-1) should report no job")
	}
}
