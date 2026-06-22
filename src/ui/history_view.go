package ui

import (
	"sort"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func newEvent(jobID int, jobName string, state string, detail string) event {
	// Use the same timestamp shape as command run records so the History tab is
	// visually consistent across startup, UI actions, manual runs, and schedules.
	return event{
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		JobID:   jobID,
		JobName: jobName,
		Trigger: "UI",
		State:   state,
		Detail:  detail,
	}
}

func collectActivity(jobs []job, runtimes map[int]*domain.JobRuntime) []event {
	var events []event
	for _, current := range jobs {
		// At startup this is usually empty because jobs.yaml does not persist
		// runtime logs. The function still centralizes the merge for future
		// history loading from log metadata.
		if rt := runtimes[current.ID]; rt != nil {
			events = append(events, rt.Logs...)
		}
	}
	sort.SliceStable(events, func(left int, right int) bool {
		return events[left].Time < events[right].Time
	})
	return events
}

func newHistoryView(events *[]event) *fyne.Container {
	descending := false
	headerText := func(id widget.TableCellID) string {
		headers := []string{"Time", "Trigger", "Job", "State", "Detail", "Log"}
		if id.Row < 0 && id.Col == 0 {
			if descending {
				return "Time desc"
			}
			return "Time asc"
		}
		if id.Row < 0 && id.Col >= 0 && id.Col < len(headers) {
			return headers[id.Col]
		}
		return ""
	}
	sortedEvents := func() []event {
		result := append([]event(nil), (*events)...)
		sort.SliceStable(result, func(left int, right int) bool {
			if descending {
				return result[left].Time > result[right].Time
			}
			return result[left].Time < result[right].Time
		})
		return result
	}

	table := widget.NewTable(
		func() (int, int) {
			return len(*events), 6
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Wrapping = fyne.TextTruncate
			return label
		},
		func(id widget.TableCellID, item fyne.CanvasObject) {
			label := item.(*widget.Label)
			label.SetText(historyCellText(id, sortedEvents()))
			label.TextStyle = fyne.TextStyle{}
			label.Refresh()
		},
	)
	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject {
		label := widget.NewLabel("")
		label.Wrapping = fyne.TextTruncate
		return label
	}
	table.UpdateHeader = func(id widget.TableCellID, item fyne.CanvasObject) {
		label := item.(*widget.Label)
		label.SetText(headerText(id))
		label.TextStyle = fyne.TextStyle{Bold: true}
		label.Refresh()
	}
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row < 0 && id.Col == 0 {
			descending = !descending
			table.Refresh()
		}
		table.Unselect(id)
	}
	table.SetColumnWidth(0, 150)
	table.SetColumnWidth(1, 90)
	table.SetColumnWidth(2, 170)
	table.SetColumnWidth(3, 90)
	table.SetColumnWidth(4, 260)
	table.SetColumnWidth(5, 240)
	return container.NewPadded(table)
}

func historyCellText(id widget.TableCellID, events []event) string {
	if id.Row < 0 || id.Row >= len(events) {
		return ""
	}
	current := events[id.Row]
	trigger := current.Trigger
	if trigger == "" {
		trigger = "Unknown"
	}
	switch id.Col {
	case 0:
		return current.Time
	case 1:
		return trigger
	case 2:
		return current.JobName
	case 3:
		return current.State
	case 4:
		return current.Detail
	case 5:
		return logFileName(current.LogFile)
	default:
		return ""
	}
}

func logFileName(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, "\\", "/")
	if slash := strings.LastIndex(path, "/"); slash >= 0 {
		return path[slash+1:]
	}
	return path
}
