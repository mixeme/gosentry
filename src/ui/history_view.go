package ui

import (
	"sort"
	"strings"
	"time"

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

// maxHistoryRows caps the session History list, the way app.maxJobLogs caps a
// job's own activity list. History is never persisted and every record carries
// the run's full captured output, so an app left running in the tray — the mode
// GoSentry is designed for — would otherwise hold every record of every run
// forever, and pay a full resort plus a full column-width rescan on each new
// one. One job on @every 10s produces ~8 600 records a day.
const maxHistoryRows = 1000

// historyLog is the session History: the capped record list plus the column
// widths measured from it. It exists so the widths can be folded in one record
// at a time instead of being recomputed from every row on every event, which
// is what made the per-event cost grow with the number of rows.
type historyLog struct {
	records []event
	widths  [6]float32
	// textSize and padding are the theme metrics widths were last measured at.
	// A theme change invalidates every measurement, so it forces a full rescan
	// rather than folding new records into stale numbers.
	textSize float32
	padding  float32
}

func newHistoryLog(records []event) *historyLog {
	h := &historyLog{records: trimHistory(records)}
	h.rescan()
	return h
}

// trimHistory drops the oldest records past the cap. The tail of the backing
// array is zeroed because a dropped record holds the run's whole output, which
// would otherwise stay reachable until the slice happens to be reallocated.
func trimHistory(records []event) []event {
	if len(records) <= maxHistoryRows {
		return records
	}
	kept := copy(records, records[len(records)-maxHistoryRows:])
	for i := kept; i < len(records); i++ {
		records[i] = event{}
	}
	return records[:kept]
}

// add appends one record and widens any content-measured column the record
// does not fit. Widths only ever grow within a theme: a column is never
// narrowed when a record ages out, because the rows still on screen were laid
// out against the wider value.
func (h *historyLog) add(record event) {
	h.records = trimHistory(append(h.records, record))
	if h.stale() {
		h.rescan()
		return
	}
	min, max := textColumnMinWidth(), textColumnMaxWidth()
	for i, value := range historyContentValues(record) {
		if width := textColumnWidth([]string{value}, min, max); width > h.widths[historyContentCols[i]] {
			h.widths[historyContentCols[i]] = width
		}
	}
}

// columnWidths returns the widths to apply to the table, rescanning every
// record only when the theme's text metrics have changed since the last scan.
func (h *historyLog) columnWidths() [6]float32 {
	if h.stale() {
		h.rescan()
	}
	return h.widths
}

func (h *historyLog) stale() bool {
	return theme.TextSize() != h.textSize || cellPadding() != h.padding
}

func (h *historyLog) rescan() {
	h.textSize, h.padding = theme.TextSize(), cellPadding()
	h.widths = historyColumnWidths(h.records)
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
	h.label.Truncation = fyne.TextTruncateClip
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

// historyHeaders are the History table's column captions, in column order. The
// Time caption is built per update because it carries the sort direction arrow.
var historyHeaders = [...]string{"Time", "Trigger", "Job", "State", "Detail", "Log"}

func newHistoryView(log *historyLog) (*fyne.Container, func()) {
	descending := false
	headerText := func(id widget.TableCellID) string {
		if id.Row < 0 && id.Col == 0 {
			if descending {
				return "Time ▼"
			}
			return "Time ▲"
		}
		if id.Row < 0 && id.Col >= 0 && id.Col < len(historyHeaders) {
			return historyHeaders[id.Col]
		}
		return ""
	}

	// rows is the sorted snapshot every callback below reads — both the length
	// callback and the cells, which must agree on the same slice. A full redraw
	// issues one update call per visible cell, so sorting inside the cell
	// callback re-sorted the whole event list a hundred times per Refresh.
	// resort() is therefore the only place the order changes, and it runs once
	// per redraw: at build time, on a sort toggle, and from refresh().
	var rows []event
	resort := func() {
		rows = append(rows[:0], log.records...)
		sort.SliceStable(rows, func(left int, right int) bool {
			if descending {
				return rows[left].Time > rows[right].Time
			}
			return rows[left].Time < rows[right].Time
		})
	}
	resort()

	table := widget.NewTable(
		func() (int, int) {
			return len(rows), len(historyHeaders)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateClip
			return label
		},
		func(id widget.TableCellID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(historyCellText(id, rows))
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
				resort()
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
	setColumnWidths := func() {
		for col, width := range log.columnWidths() {
			table.SetColumnWidth(col, width)
		}
	}
	setColumnWidths()

	// refresh re-reads the event list into the sorted snapshot and re-applies
	// the column widths before redrawing, so newly recorded events appear in
	// the current sort order and longer values widen their column instead of
	// being truncated. The widths come from historyLog, which folded each new
	// record in as it arrived — this does not rescan every row.
	refresh := func() {
		resort()
		setColumnWidths()
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
