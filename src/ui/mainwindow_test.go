package ui

import (
	"path/filepath"
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/storage"

	"fyne.io/fyne/v2/test"
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

func TestMainViewBuilds(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	w := testApp.NewWindow("test")
	defer w.Close()

	svc := newTestService(t)
	defer svc.Stop()

	content, recordStartup := newMainView(w, svc)
	if content == nil {
		t.Fatal("newMainView returned nil content")
	}
	w.SetContent(content)
	recordStartup(0, true)
}
