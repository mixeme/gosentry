package ui

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestLastJobLogsCapsAndCopies(t *testing.T) {
	logs := []event{
		{Time: "1", JobName: "a"},
		{Time: "2", JobName: "b"},
		{Time: "3", JobName: "c"},
		{Time: "4", JobName: "d"},
	}
	got := lastJobLogs(logs)
	if len(got) != maxJobActivityRows {
		t.Fatalf("len = %d, want %d", len(got), maxJobActivityRows)
	}
	for i, want := range []string{"1", "2", "3"} {
		if got[i].Time != want {
			t.Errorf("got[%d].Time = %q, want %q", i, got[i].Time, want)
		}
	}
	logs[0].Time = "mutated"
	if got[0].Time == "mutated" {
		t.Error("lastJobLogs must return a defensive copy")
	}
}

func TestLastJobLogsEmpty(t *testing.T) {
	if got := lastJobLogs(nil); len(got) != 0 {
		t.Errorf("nil input: got %v, want empty", got)
	}
}

func TestIndexOfID(t *testing.T) {
	jobs := []job{
		{ID: 10, Name: "A"},
		{ID: 20, Name: "B"},
	}
	if got := indexOfID(jobs, 20); got != 1 {
		t.Errorf("found: got %d, want 1", got)
	}
	if got := indexOfID(jobs, 99); got != -1 {
		t.Errorf("missing: got %d, want -1", got)
	}
	if got := indexOfID(nil, 1); got != -1 {
		t.Errorf("empty slice: got %d, want -1", got)
	}
}

func TestHistoryCellText(t *testing.T) {
	events := []event{{
		Time:    "2026-06-01 12:00:00",
		Trigger: "",
		JobName: "Job",
		State:   "OK",
		Detail:  "done",
		LogFile: `/logs/20260601-120000_Job.log`,
	}}
	cases := []struct {
		col  int
		want string
	}{
		{0, "2026-06-01 12:00:00"},
		{1, "Unknown"},
		{2, "Job"},
		{3, "OK"},
		{4, "done"},
		{5, "20260601-120000_Job.log"},
	}
	for _, tc := range cases {
		got := historyCellText(widget.TableCellID{Row: 0, Col: tc.col}, events)
		if got != tc.want {
			t.Errorf("col %d: got %q, want %q", tc.col, got, tc.want)
		}
	}
	if got := historyCellText(widget.TableCellID{Row: -1, Col: 0}, events); got != "" {
		t.Errorf("header row: got %q, want empty", got)
	}
	if got := historyCellText(widget.TableCellID{Row: 99, Col: 0}, events); got != "" {
		t.Errorf("out of range row: got %q, want empty", got)
	}
}

func TestLogFileName(t *testing.T) {
	cases := []struct{ path, want string }{
		{"", ""},
		{"   ", ""},
		{`C:\logs\run.log`, "run.log"},
		{"/var/logs/2026/job.log", "job.log"},
		{"plain.log", "plain.log"},
	}
	for _, tc := range cases {
		if got := logFileName(tc.path); got != tc.want {
			t.Errorf("logFileName(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestHistorySortToggleKeepsRowsInSync is the regression guard for F11: the
// table now reads one cached sorted snapshot instead of re-sorting inside every
// cell callback, so the length callback and the cells have to be refilled
// together. If either the sort toggle or refresh stops calling resort(), the
// row count and the cell contents disagree — which no compiler check catches.
func TestHistorySortToggleKeepsRowsInSync(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	events := []event{
		{Time: "2026-06-01 10:00:00", JobName: "A"},
		{Time: "2026-06-01 11:00:00", JobName: "B"},
		{Time: "2026-06-01 12:00:00", JobName: "C"},
	}
	log := newHistoryLog(events)
	content, refresh := newHistoryView(log)
	table, ok := content.Objects[0].(*widget.Table)
	if !ok {
		t.Fatal("history view does not wrap a table")
	}

	rowCount := func() int {
		t.Helper()
		rows, cols := table.Length()
		if cols != len(historyHeaders) {
			t.Errorf("column count = %d, want %d", cols, len(historyHeaders))
		}
		return rows
	}
	// Column 2 is the Job name, the field these fixtures vary.
	jobAt := func(row int) string {
		t.Helper()
		cell := table.CreateCell()
		table.UpdateCell(widget.TableCellID{Row: row, Col: 2}, cell)
		return cell.(*widget.Label).Text
	}
	// The sort toggle lives on the Time header cell, which is only wired up
	// when UpdateHeader runs for it.
	header := table.CreateHeader()
	table.UpdateHeader(widget.TableCellID{Row: -1, Col: 0}, header)
	timeHeader, ok := header.(*historyHeader)
	if !ok {
		t.Fatal("history table header is not a historyHeader")
	}
	assertOrder := func(when string, want ...string) {
		t.Helper()
		if got := rowCount(); got != len(want) {
			t.Fatalf("%s: row count = %d, want %d", when, got, len(want))
		}
		for row, name := range want {
			if got := jobAt(row); got != name {
				t.Errorf("%s: row %d = %q, want %q", when, row, got, name)
			}
		}
	}

	assertOrder("ascending", "A", "B", "C")

	test.Tap(timeHeader)
	assertOrder("descending", "C", "B", "A")

	// A new run arrives while the table is sorted newest-first: it must be
	// counted and placed in the order currently on screen, not the build-time one.
	log.add(event{Time: "2026-06-01 13:00:00", JobName: "D"})
	refresh()
	assertOrder("descending after refresh", "D", "C", "B", "A")

	test.Tap(timeHeader)
	assertOrder("ascending after refresh", "A", "B", "C", "D")
}

// TestHistoryCellTemplateIsPlainText guards the dropped per-cell TextStyle
// assignment: the template must already carry the zero style, since nothing
// resets it any more.
func TestHistoryCellTemplateIsPlainText(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	content, _ := newHistoryView(newHistoryLog(nil))
	table := content.Objects[0].(*widget.Table)
	label, ok := table.CreateCell().(*widget.Label)
	if !ok {
		t.Fatal("history cell template is not a label")
	}
	if label.TextStyle != (fyne.TextStyle{}) {
		t.Errorf("cell template TextStyle = %+v, want the zero value", label.TextStyle)
	}
}

// TestTextColumnWidthClamps covers the three shapes textColumnWidth has to
// handle: a sample narrower than min, one that lands between the bounds, and
// one wide enough to hit the max cap.
func TestTextColumnWidthClamps(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	min, max := float32(50), float32(120)
	if got := textColumnWidth([]string{"x"}, min, max); got != min {
		t.Errorf("below-min sample: got %v, want the floor %v", got, min)
	}
	inRange := textWidth("mid-sized value") + cellPadding()
	if inRange <= min || inRange >= max {
		t.Skip("fixture sample no longer lands strictly between the bounds under this theme")
	}
	if got := textColumnWidth([]string{"mid-sized value"}, min, max); got != inRange {
		t.Errorf("in-range sample: got %v, want %v", got, inRange)
	}
	if got := textColumnWidth([]string{strings.Repeat("0", 200)}, min, max); got != max {
		t.Errorf("above-max sample: got %v, want the cap %v", got, max)
	}
	if got := textColumnWidth(nil, min, max); got != min {
		t.Errorf("no samples: got %v, want the floor %v", got, min)
	}
}

// TestHistoryColumnsFitTheirContent guards F6/F14: every column must be at
// least as wide as its widest known or actually-present value, at the default
// theme and at a scaled one, so nothing that used to be a pixel constant
// clips again.
func TestHistoryColumnsFitTheirContent(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	rows := []event{
		{
			Time:    "2026-06-01 12:00:00",
			Trigger: "Schedule",
			JobName: "A moderately long job name for width testing",
			State:   "Jobs loaded",
			Detail:  "A somewhat longer detail message describing what happened",
			LogFile: `/logs/20260601-120000_SomeJobName.log`,
		},
	}

	check := func(when string) {
		t.Helper()
		widths := historyColumnWidths(rows)
		samples := [][]string{
			{historyTimeSample},
			historyTriggerSamples,
			{rows[0].JobName},
			historyStateSamples,
			{rows[0].Detail},
			{logFileName(rows[0].LogFile)},
		}
		min, max := textColumnMinWidth(), textColumnMaxWidth()
		for col, colSamples := range samples {
			want := textColumnWidth(colSamples, min, max)
			if col == 0 {
				want = textWidth(historyTimeSample) + cellPadding()
			}
			if widths[col] < want {
				t.Errorf("%s: column %d width = %v, want at least %v", when, col, widths[col], want)
			}
		}
	}

	check("default theme")
	testApp.Settings().SetTheme(test.NewTheme())
	check("scaled theme")
}

// TestHistoryLogCapsRecords is the regression guard for the unbounded History
// list: an app left in the tray records thousands of runs a day, each carrying
// the run's whole captured output, so the list must drop the oldest instead of
// growing forever.
func TestHistoryLogCapsRecords(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	log := newHistoryLog(nil)
	for i := 0; i < maxHistoryRows+25; i++ {
		log.add(event{Time: "t", JobName: "Job " + strconv.Itoa(i)})
	}
	if len(log.records) != maxHistoryRows {
		t.Fatalf("record count = %d, want capped at %d", len(log.records), maxHistoryRows)
	}
	if got, want := log.records[0].JobName, "Job 25"; got != want {
		t.Errorf("oldest kept record = %q, want %q — the cap must drop from the front", got, want)
	}
	last := log.records[len(log.records)-1].JobName
	if want := "Job " + strconv.Itoa(maxHistoryRows+24); last != want {
		t.Errorf("newest record = %q, want %q", last, want)
	}
	// A list handed in above the cap is trimmed too, not only one grown into it.
	oversized := make([]event, maxHistoryRows+10)
	if got := len(newHistoryLog(oversized).records); got != maxHistoryRows {
		t.Errorf("pre-filled log length = %d, want %d", got, maxHistoryRows)
	}
}

// TestHistoryLogWidthsMatchAFullScan pins the incremental column widths: while
// every measured record is still in the list, folding each one in as it
// arrives must give exactly what rescanning every row would, or the cheaper
// path would clip values the old one showed.
func TestHistoryLogWidthsMatchAFullScan(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	log := newHistoryLog(nil)
	for _, record := range []event{
		{Time: "1", JobName: "A", Detail: "short", LogFile: `/logs/a.log`},
		{Time: "2", JobName: "A moderately long job name", Detail: "a longer detail message", LogFile: `/logs/20260601-120000_SomeJobName.log`},
		{Time: "3", JobName: "B", Detail: "s", LogFile: `/logs/b.log`},
	} {
		log.add(record)
	}
	if got, want := log.columnWidths(), historyColumnWidths(log.records); got != want {
		t.Errorf("incremental widths = %v, want the full-scan widths %v", got, want)
	}
}

// TestHistoryLogWidthsDoNotShrinkWhenRecordsAgeOut covers the other half of the
// rule: widths only grow. Dropping the record that set a column's width must
// not narrow the column, because the rows on screen were laid out against it.
func TestHistoryLogWidthsDoNotShrinkWhenRecordsAgeOut(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	log := newHistoryLog(nil)
	log.add(event{Time: "1", JobName: "A job name long enough to widen its column"})
	widest := log.columnWidths()[2]
	for i := 0; i < maxHistoryRows; i++ {
		log.add(event{Time: "t", JobName: "x"})
	}
	if got := log.columnWidths()[2]; got != widest {
		t.Errorf("Job column width = %v after the wide record aged out, want it held at %v", got, widest)
	}
}

// TestHistoryLogRescansOnThemeChange guards the one case the incremental fold
// cannot handle: every stored width was measured at the old text size, so a
// theme change has to fall back to a full rescan.
func TestHistoryLogRescansOnThemeChange(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	log := newHistoryLog([]event{
		{Time: "1", JobName: "A moderately long job name", Detail: "a longer detail message"},
	})
	before := log.columnWidths()

	testApp.Settings().SetTheme(test.NewTheme())
	after := log.columnWidths()
	if after == before {
		t.Fatal("widths unchanged after a theme change; the fixture theme must alter text metrics")
	}
	if want := historyColumnWidths(log.records); after != want {
		t.Errorf("widths after theme change = %v, want the rescanned %v", after, want)
	}
}

func TestNewEventUsesConsistentTimestampShape(t *testing.T) {
	ev := newEvent(1, "Job", "OK", "detail")
	if _, err := time.Parse("2006-01-02 15:04:05", ev.Time); err != nil {
		t.Errorf("timestamp %q is not in expected layout: %v", ev.Time, err)
	}
	if ev.Trigger != "UI" || ev.JobID != 1 || ev.JobName != "Job" {
		t.Errorf("unexpected event fields: %+v", ev)
	}
}
