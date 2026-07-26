package ui

import (
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

// lastJobLogs returns a fresh slice of the most recent activity entries for the
// "Selected job activity" panel. Logs are stored newest-first (see
// app.Service.recordRun), so the leading entries are the latest; the result is
// capped at maxJobActivityRows.
func lastJobLogs(logs []event) []event {
	n := len(logs)
	if n > maxJobActivityRows {
		n = maxJobActivityRows
	}
	return append([]event(nil), logs[:n]...)
}

func filteredJobIndexes(jobs []job, folder string) []int {
	indexes := make([]int, 0, len(jobs))
	for index, current := range jobs {
		if folder == allFolders || filterValue(current.Folder) == folder {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func folderOptions(jobs []job) []string {
	// "All" and "No folder" are always present so the filter UI is stable even
	// before the user creates folders.
	options := []string{allFolders, noFolder}
	seen := map[string]bool{allFolders: true, noFolder: true}
	for _, current := range jobs {
		folder := strings.TrimSpace(current.Folder)
		if folder == "" || seen[folder] {
			continue
		}
		seen[folder] = true
		options = append(options, folder)
	}
	return options
}

func filterValue(folder string) string {
	if strings.TrimSpace(folder) == "" {
		return noFolder
	}
	return strings.TrimSpace(folder)
}

// nextJobListView returns the mode the view toggle switches to. Anything that
// is not compact reads as detailed, so unknown and legacy values flip to
// compact just as an explicit "detailed" does.
func nextJobListView(current domain.JobListView) domain.JobListView {
	if current.IsCompact() {
		return domain.JobListViewDetailed
	}
	return domain.JobListViewCompact
}

// viewToggleText labels the view toggle with the action it performs, not the
// current state — the same convention as the "Disable auto"/"Enable auto"
// button. It also keeps the on-disk strings away from the user.
func viewToggleText(current domain.JobListView) string {
	if current.IsCompact() {
		return "Detailed"
	}
	return "Compact"
}

func indexOfID(jobs []job, id int) int {
	for index, current := range jobs {
		if current.ID == id {
			return index
		}
	}
	return -1
}
