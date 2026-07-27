package ui

import (
	"errors"
	"net/url"
	"runtime/debug"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/platform/filemanager"
	"gitea.mixdep.ru/mix/gosentry/src/storage"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

func fyneVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dependency := range info.Deps {
		if dependency.Path == "fyne.io/fyne/v2" {
			if dependency.Replace != nil && dependency.Replace.Version != "" {
				return dependency.Replace.Version
			}
			if dependency.Version != "" {
				return dependency.Version
			}
			return "local"
		}
	}
	return "unknown"
}

func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		return &url.URL{}
	}
	return parsed
}

// chooseFile opens a file picker that writes the chosen path into target. A
// nil filter offers every file, as job_dialog.go's command browser wants;
// chooseJSONFile passed an extension filter here before the two were merged,
// since they differed only by that one call.
func chooseFile(w fyne.Window, target *widget.Entry, filter fynestorage.FileFilter) {
	fileDialog := dialog.NewFileOpen(func(uri fyne.URIReadCloser, err error) {
		if err != nil || uri == nil {
			return
		}
		target.SetText(uri.URI().Path())
	}, w)
	if filter != nil {
		fileDialog.SetFilter(filter)
	}
	fileDialog.Resize(fyne.NewSize(900, 640))
	fileDialog.Show()
}

// chooseJSONFile is chooseFile restricted to .json files, used for the jobs
// file so the picker does not list every file in the folder. The entry stays
// editable, which is how a path to a file that does not exist yet is entered.
func chooseJSONFile(w fyne.Window, target *widget.Entry) {
	chooseFile(w, target, fynestorage.NewExtensionFileFilter([]string{".json"}))
}

func chooseFolder(w fyne.Window, target *widget.Entry) {
	folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		target.SetText(uri.Path())
	}, w)
	// The default folder picker can be cramped on Windows. A larger size makes
	// long paths readable and avoids forcing the user to resize it every time.
	folderDialog.Resize(fyne.NewSize(900, 640))
	folderDialog.Show()
}

// settingsFolderPath resolves what a directory field currently points at,
// applying the same relative-path rule the store uses when it loads the config
// so the folder that opens is the one the setting would use. Blank text has no
// folder to open and yields an empty path.
func settingsFolderPath(appDir string, text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	return storage.ResolveConfiguredPath(appDir, trimmed)
}

// openFolder reveals dir in the desktop file manager. A folder that is not set
// or cannot be opened (most often: it does not exist yet, because the logs
// directory is created on the first run) is reported in a dialog rather than
// leaving the button looking dead.
func openFolder(w fyne.Window, dir string) {
	if dir == "" {
		dialog.ShowError(errors.New("no folder is set"), w)
		return
	}
	if err := filemanager.Open(dir); err != nil {
		dialog.ShowError(err, w)
	}
}
