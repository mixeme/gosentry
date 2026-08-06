package ui

import (
	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

// jobsViewState is the model behind the Jobs tab: the snapshot of the Service's
// jobs and runtimes, the folder filter, and the selection. The widgets in
// jobsView read it and never keep a second copy of any of it.
//
// The selection is a job ID, not an index into the snapshot. Every path that
// changes the job list replaces that snapshot underneath the view — create,
// delete, and edit do it from this view's own handlers, but adopting a different
// jobs file does it from the Service, and the view only learns about it through
// the refresh that JobsLoaded triggers. An index that outlives its snapshot then
// points at whichever job happens to sit there now, so the details pane
// describes one job while the list highlights another. Indexes are derived from
// the ID at render time instead (selectedIndex, displayRow).
type jobsViewState struct {
	svc      *app.Service
	jobs     []job
	runtimes map[int]*domain.JobRuntime
	folder   string
	// selectedID is 0 when nothing is selected; job IDs start at 1.
	selectedID int
	// filtered holds the indexes into jobs that the folder filter shows, in list
	// row order: filtered[row] is the index of the job drawn in that row.
	filtered []int
}

func newJobsViewState(svc *app.Service) *jobsViewState {
	s := &jobsViewState{
		svc:      svc,
		runtimes: map[int]*domain.JobRuntime{},
		folder:   allFolders,
	}
	s.sync()
	return s
}

// sync re-reads the Service snapshot, re-applies the folder filter, and
// re-resolves the selection against the new list. It is the only place the view
// reads job state from the Service.
func (s *jobsViewState) sync() {
	s.jobs = s.svc.Jobs()
	clear(s.runtimes)
	for _, current := range s.jobs {
		if rt := s.svc.Runtime(current.ID); rt != nil {
			s.runtimes[current.ID] = rt
		}
	}
	s.filtered = filteredJobIndexes(s.jobs, s.folder)
	s.resolveSelection()
}

// applyFilter switches the folder filter, keeping the current selection when the
// new filter still shows it. A filter that matches nothing — "No folder" with no
// such job — is a real filter choice, not an error state, so it simply leaves
// nothing selected.
func (s *jobsViewState) applyFilter(folder string) {
	s.folder = folder
	s.filtered = filteredJobIndexes(s.jobs, s.folder)
	if !s.visible(s.selectedID) {
		s.selectedID = 0
	}
	s.resolveSelection()
}

// resolveSelection drops a selection whose job is gone and falls back to the
// first visible row, so the details pane never describes a job the current
// snapshot no longer holds.
func (s *jobsViewState) resolveSelection() {
	if s.selectedID != 0 && indexOfID(s.jobs, s.selectedID) < 0 {
		s.selectedID = 0
	}
	if s.selectedID == 0 && len(s.filtered) > 0 {
		s.selectedID = s.jobs[s.filtered[0]].ID
	}
}

// selectByID records the selection directly, for handlers that know the job they
// want selected (a newly created job, for instance) rather than its row.
func (s *jobsViewState) selectByID(id int) {
	s.selectedID = id
}

// selectRow records the selection from a list row, which is what widget.List
// reports through OnSelected.
func (s *jobsViewState) selectRow(row int) {
	current, ok := s.jobAt(row)
	if !ok {
		s.selectedID = 0
		return
	}
	s.selectedID = current.ID
}

// selected returns the selected job, or false when nothing is selected.
func (s *jobsViewState) selected() (job, bool) {
	index := s.selectedIndex()
	if index < 0 {
		return job{}, false
	}
	return s.jobs[index], true
}

// selectedIndex resolves the selected ID to an index into the current snapshot,
// or -1 when nothing is selected.
func (s *jobsViewState) selectedIndex() int {
	if s.selectedID == 0 {
		return -1
	}
	return indexOfID(s.jobs, s.selectedID)
}

// displayRow maps the selection onto a list row, or -1 when nothing is selected
// or the filter hides the selected job — so a caller unselects rather than
// highlighting an unrelated row.
func (s *jobsViewState) displayRow() int {
	index := s.selectedIndex()
	if index < 0 || !s.visible(s.selectedID) {
		return -1
	}
	return app.DisplayIndex(s.filtered, index)
}

// jobAt returns the job drawn in the given list row.
func (s *jobsViewState) jobAt(row int) (job, bool) {
	if row < 0 || row >= len(s.filtered) {
		return job{}, false
	}
	return s.jobs[s.filtered[row]], true
}

// runtime returns a job's runtime, or an empty one when the Service has none
// yet, so callers can read it without a nil check.
func (s *jobsViewState) runtime(id int) *domain.JobRuntime {
	if rt := s.runtimes[id]; rt != nil {
		return rt
	}
	return &domain.JobRuntime{}
}

// visible reports whether the folder filter shows the given job.
func (s *jobsViewState) visible(id int) bool {
	if id == 0 {
		return false
	}
	for _, index := range s.filtered {
		if s.jobs[index].ID == id {
			return true
		}
	}
	return false
}
