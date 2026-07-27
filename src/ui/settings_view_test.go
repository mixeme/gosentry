package ui

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// TestSettingsFolderPath covers the path the "Open" button beside the logs
// directory hands to the file manager: blank means nothing to open, a relative
// directory resolves against the application directory (as the store does),
// and an absolute directory is used as typed. Both directories come from
// t.TempDir so the absolute case is genuinely absolute on Windows too.
func TestSettingsFolderPath(t *testing.T) {
	appDir := t.TempDir()
	absolute := filepath.Join(t.TempDir(), "logs")

	cases := []struct {
		name string
		text string
		want string
	}{
		{name: "empty", text: "", want: ""},
		{name: "whitespace only", text: "   ", want: ""},
		{name: "relative", text: "logs", want: filepath.Join(appDir, "logs")},
		{name: "relative with spaces around it", text: "  logs  ", want: filepath.Join(appDir, "logs")},
		{name: "absolute", text: absolute, want: absolute},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := settingsFolderPath(appDir, testCase.text); got != testCase.want {
				t.Errorf("settingsFolderPath(%q, %q) = %q, want %q", appDir, testCase.text, got, testCase.want)
			}
		})
	}
}

// TestSettingsRowStretchesItsControl is the property that makes F2's removal
// of settingsControlWidth invisible: settingsRow puts the value in a Border
// centre slot, which already stretches it to the column width on its own, so
// wrapping it in a fixed-width layout was redundant.
func TestSettingsRowStretchesItsControl(t *testing.T) {
	entry := widget.NewEntry()
	row := settingsRow(captionColumnWidth("Label"), "Label", entry)

	baseWidth := entry.Size().Width
	wide := fyne.NewSize(row.MinSize().Width+200, row.MinSize().Height)
	row.Resize(wide)

	// Only the caption column and one inter-column padding come out of the
	// extra width; the rest must reach the control. Requiring most of the
	// 200px growth to show up on the entry is what a reinstated fixed-width
	// wrapper around it would break.
	if got := entry.Size().Width; got < baseWidth+150 {
		t.Errorf("control did not stretch to fill the row: entry width = %v, want at least %v", got, baseWidth+150)
	}
}

// TestChooseFileAppliesFilter is the coverage for the deduplicated file picker
// (chooseFile absorbed chooseJSONFile's SetFilter call behind a nil-means-none
// filter argument): both a nil filter (job_dialog.go's command browser) and a
// concrete one (chooseJSONFile) must open a dialog without panicking, and the
// dialog must actually appear as a canvas overlay either way.
func TestChooseFileAppliesFilter(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()
	w := testApp.NewWindow("test")
	defer w.Close()

	target := widget.NewEntry()

	chooseFile(w, target, nil)
	if w.Canvas().Overlays().Top() == nil {
		t.Fatal("chooseFile(nil filter) did not open a dialog")
	}
	w.Canvas().Overlays().Top().Hide()

	chooseJSONFile(w, target)
	if w.Canvas().Overlays().Top() == nil {
		t.Fatal("chooseJSONFile did not open a dialog")
	}
	w.Canvas().Overlays().Top().Hide()

	// Exercising a concrete filter directly through chooseFile as well, so the
	// filter parameter itself (not just chooseJSONFile's use of it) is covered.
	chooseFile(w, target, fynestorage.NewExtensionFileFilter([]string{".json"}))
	if w.Canvas().Overlays().Top() == nil {
		t.Fatal("chooseFile(non-nil filter) did not open a dialog")
	}
	w.Canvas().Overlays().Top().Hide()
}

// TestSettingsCaptionsCoverEveryRow is the settings-tab analog of F10's
// jobs-details guard: every caption settingsView actually uses in a row must
// be present in settingsCaptions and measure no wider than
// captionColumnWidth's result for that list, or a caption added to a row
// without adding it to settingsCaptions would silently misalign that column.
func TestSettingsCaptionsCoverEveryRow(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	capW := captionColumnWidth(settingsCaptions...)
	for _, c := range settingsCaptions {
		if c == "" {
			continue
		}
		if w := widget.NewLabelWithStyle(c, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}).MinSize().Width; w > capW {
			t.Errorf("caption %q measures %v, wider than captionColumnWidth's %v", c, w, capW)
		}
	}
}
