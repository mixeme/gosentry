package ui

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
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
	row := settingsRow("Label", entry)

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
