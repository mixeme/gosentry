package ui

import (
	"sort"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
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

// logColumnMinWidth/logColumnMaxWidth bound the dynamically sized Log column.
// The minimum keeps the column readable when names are short or absent; the
// maximum stops a single very long file name from dominating the table (the
// table still scrolls horizontally past it).
const (
	logColumnMinWidth = 240
	logColumnMaxWidth = 520
	logColumnPadding  = 24
)

// logColumnWidth measures the widest Log cell value so the column can be sized
// to fit its content. Fyne tables do not auto-size columns, so without this the
// fixed width clips file names like "20260601-100000_SomeJobName.log".
func logColumnWidth(events []event) float32 {
	width := float32(logColumnMinWidth)
	for _, current := range events {
		text := logFileName(current.LogFile)
		if text == "" {
			continue
		}
		w := fyne.MeasureText(text, theme.TextSize(), fyne.TextStyle{}).Width + logColumnPadding
		if w > width {
			width = w
		}
	}
	if width > logColumnMaxWidth {
		width = logColumnMaxWidth
	}
	return width
}

// historyHeader is a bold tappable label used in the History table header row.
// In Fyne 2.7+ OnSelected is not fired for header cells (Row < 0), so the sort
// toggle is wired through the Tappable interface instead.
type historyHeader struct {
	widget.BaseWidget
	label    *widget.Label
	OnTapped func()
}

func newHistoryHeader() *historyHeader {
	h := &historyHeader{label: widget.NewLabel("")}
	h.label.TextStyle = fyne.TextStyle{Bold: true}
	h.label.Wrapping = fyne.TextTruncate
	h.ExtendBaseWidget(h)
	return h
}

func (h *historyHeader) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(h.label)
}

func (h *historyHeader) Tapped(*fyne.PointEvent) {
	if h.OnTapped != nil {
		h.OnTapped()
	}
}

func (h *historyHeader) SetText(text string) {
	h.label.SetText(text)
}

func newHistoryView(events *[]event) (*fyne.Container, func()) {
	descending := false
	headerText := func(id widget.TableCellID) string {
		headers := []string{"Time", "Trigger", "Job", "State", "Detail", "Log"}
		if id.Row < 0 && id.Col == 0 {
			if descending {
				return "Time ▼"
			}
			return "Time ▲"
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
		return newHistoryHeader()
	}
	table.UpdateHeader = func(id widget.TableCellID, item fyne.CanvasObject) {
		h := item.(*historyHeader)
		h.SetText(headerText(id))
		if id.Row < 0 && id.Col == 0 {
			h.OnTapped = func() {
				descending = !descending
				table.Refresh()
			}
		} else {
			h.OnTapped = nil
		}
		h.Refresh()
	}
	table.OnSelected = func(id widget.TableCellID) {
		table.Unselect(id)
	}
	table.SetColumnWidth(0, 150)
	table.SetColumnWidth(1, 90)
	table.SetColumnWidth(2, 170)
	table.SetColumnWidth(3, 90)
	table.SetColumnWidth(4, 260)
	table.SetColumnWidth(5, logColumnWidth(*events))

	// refresh recomputes the content-fit Log column width before redrawing, so
	// newly recorded events with longer file names widen the column instead of
	// being truncated.
	refresh := func() {
		table.SetColumnWidth(5, logColumnWidth(*events))
		table.Refresh()
	}
	return container.NewPadded(table), refresh
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
