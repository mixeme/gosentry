package ui

import (
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/storage"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// newTestStore builds a Store rooted in a temp directory. It is separate from
// newTestService so tests that need a non-default Config (or their own jobs)
// can adjust it before handing it to app.NewService.
func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	return &storage.Store{
		Paths: storage.Paths{
			ExecutablePath: filepath.Join(dir, "gosentry"),
			AppDir:         dir,
			ConfigPath:     filepath.Join(dir, "gosentry.json"),
			JobsDir:        dir,
			JobsPath:       filepath.Join(dir, "jobs.json"),
			LogsDir:        filepath.Join(dir, "logs"),
		},
		Config: domain.Config{
			JobsFile:              "jobs.json",
			LogsDir:               "logs",
			MaxLogFiles:           100,
			MaxLogAgeDays:         30,
			ExecutionMode:         domain.ExecutionModeParallel,
			OverlapPolicy:         domain.OverlapPolicySkip,
			DefaultTimeoutSeconds: 30,
			KeepRunningInTray:     true,
			NotifyOnFailure:       true,
		},
	}
}

func newTestService(t *testing.T) *app.Service {
	t.Helper()
	return app.NewService(newTestStore(t), nil)
}

// TestMainViewFitsTheDefaultWindowSize is the regression guard for F1: the
// assembled content must fit within the window size the app asks for, so Fyne
// never silently widens the window past it. The store's ConfigPath is
// deliberately long so the test also covers F3 — the config path label must
// not grow the Settings tab's minimum width with it.
func TestMainViewFitsTheDefaultWindowSize(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	w := testApp.NewWindow("test")
	defer w.Close()

	store := newTestStore(t)
	store.Paths.ConfigPath = filepath.Join(t.TempDir(), "a-deliberately-long-directory-name-to-stress-the-config-path-label", "gosentry.json")
	svc := app.NewService(store, nil)
	defer svc.Stop()

	content, _ := newMainView(w, svc)
	min := content.MinSize()
	if min.Width > defaultWindowWidth || min.Height > defaultWindowHeight {
		t.Errorf("content.MinSize() = %v, want within %vx%v", min, defaultWindowWidth, defaultWindowHeight)
	}
}

// historyTable returns the History tab's table. It is the only widget.Table the
// main view builds, so the search does not need to know the tab order.
func historyTable(t *testing.T, content fyne.CanvasObject) *widget.Table {
	t.Helper()
	tabs, ok := content.(*container.AppTabs)
	if !ok {
		t.Fatal("main view is not the expected AppTabs container")
	}
	for _, item := range tabs.Items {
		found := findFirst(item.Content, func(o fyne.CanvasObject) bool {
			_, ok := o.(*widget.Table)
			return ok
		})
		if found != nil {
			return found.(*widget.Table)
		}
	}
	t.Fatal("main view has no history table")
	return nil
}

// TestMainViewRecordStartupAddsHistoryRow covers the second return value of
// newMainView. run.go calls it once per launch with a different windowShown
// flag depending on whether the app started into the tray, and that call is the
// only thing that puts the startup receipt into History — so both the wording
// and the fact that the table is redrawn are worth pinning. Building the full
// tab set and setting it as the window content is a side benefit: no other test
// assembles all three tabs together.
func TestMainViewRecordStartupAddsHistoryRow(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	w := testApp.NewWindow("test")
	defer w.Close()

	svc := newTestService(t)
	defer svc.Stop()

	content, recordStartup := newMainView(w, svc)
	w.SetContent(content)

	table := historyTable(t, content)
	if rows, _ := table.Length(); rows != 0 {
		t.Fatalf("history rows before startup = %d, want 0", rows)
	}

	recordStartup(1500*time.Millisecond, true)
	recordStartup(20*time.Millisecond, false)

	rows, _ := table.Length()
	if rows != 2 {
		t.Fatalf("history rows after two startup records = %d, want 2", rows)
	}

	// Read the rows back through the table's own cell callbacks, which is what
	// the redraw does; a value only in the events slice would not prove the
	// table was refreshed with it.
	cell := table.CreateCell()
	cellText := func(row, col int) string {
		table.UpdateCell(widget.TableCellID{Row: row, Col: col}, cell)
		return cell.(*widget.Label).Text
	}
	if got := cellText(0, 2); got != "Application" {
		t.Errorf("startup row job = %q, want %q", got, "Application")
	}
	if got := cellText(0, 3); got != "Started" {
		t.Errorf("startup row state = %q, want %q", got, "Started")
	}
	if got := cellText(0, 4); got != "Window shown in 1.5s" {
		t.Errorf("windowed startup detail = %q, want %q", got, "Window shown in 1.5s")
	}
	if got := cellText(1, 4); got != "Started in tray in 20ms" {
		t.Errorf("tray startup detail = %q, want %q", got, "Started in tray in 20ms")
	}
}
