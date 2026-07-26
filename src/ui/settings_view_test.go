package ui

import (
	"path/filepath"
	"testing"
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
