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
