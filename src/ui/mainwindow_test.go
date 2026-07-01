package ui

import (
	"path/filepath"
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/app"
	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/storage"

	"fyne.io/fyne/v2/test"
)

func newTestService(t *testing.T) *app.Service {
	t.Helper()
	dir := t.TempDir()
	store := &storage.Store{
		Paths: storage.Paths{
			ExecutablePath: filepath.Join(dir, "gosentry"),
			AppDir:         dir,
			ConfigPath:     filepath.Join(dir, "gosentry.json"),
			JobsDir:        dir,
			JobsPath:       filepath.Join(dir, "jobs.json"),
			LogsDir:        filepath.Join(dir, "logs"),
		},
		Config: domain.Config{
			JobsDir:         ".",
			LogsDir:         "logs",
			MaxLogFiles:     100,
			MaxLogAgeDays:   30,
			ExecutionMode:   domain.ExecutionModeParallel,
			OverlapPolicy:   domain.OverlapPolicySkip,
			KeepRunningInTray: true,
			NotifyOnFailure:   true,
		},
	}
	return app.NewService(store, nil)
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
