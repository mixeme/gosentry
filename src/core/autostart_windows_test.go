//go:build windows

package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRegistryRunValue(t *testing.T) {
	output := `
HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run
    PySentry    REG_SZ    "D:\Apps\PySentry\pysentry.exe"
`
	value, ok := parseRegistryRunValue(output)
	if !ok {
		t.Fatal("expected registry value to parse")
	}
	if value != `D:\Apps\PySentry\pysentry.exe` {
		t.Fatalf("unexpected value: %q", value)
	}
}

func TestSameWindowsPathIgnoresCaseAndQuotes(t *testing.T) {
	if !sameWindowsPath(`"D:\Apps\PySentry\pysentry.exe"`, `d:\apps\pysentry\pysentry.exe`) {
		t.Fatal("expected paths to match")
	}
}

func TestSameWindowsPathHandlesSpaces(t *testing.T) {
	if !sameWindowsPath(`"D:\Local Git\GoSentry\gosentry.exe"`, `d:\local git\gosentry\gosentry.exe`) {
		t.Fatal("expected paths with spaces to match")
	}
}

func TestStartupShortcutPathUsesUserStartupFolder(t *testing.T) {
	t.Setenv("APPDATA", `C:\Users\mixem\AppData\Roaming`)

	path, err := startupShortcutPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `C:\Users\mixem\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Startup\GoSentry.lnk`
	if path != expected {
		t.Fatalf("unexpected shortcut path: %q", path)
	}
}

func TestCreateStartupShortcutHandlesSpaces(t *testing.T) {
	tempDir := t.TempDir()
	shortcutPath := filepath.Join(tempDir, "GoSentry test.lnk")
	targetPath := filepath.Join(tempDir, "Program Files", "GoSentry", "gosentry.exe")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("test"), 0644); err != nil {
		t.Fatalf("create target file: %v", err)
	}

	if err := createStartupShortcut(shortcutPath, targetPath, ""); err != nil {
		t.Fatalf("create shortcut: %v", err)
	}

	actual, arguments, err := readShortcut(shortcutPath)
	if err != nil {
		t.Fatalf("read shortcut: %v", err)
	}
	if !sameWindowsPath(actual, targetPath) {
		t.Fatalf("shortcut target mismatch: got %q want %q", actual, targetPath)
	}
	if arguments != StartInTrayArgument {
		t.Fatalf("shortcut arguments mismatch: got %q want %q", arguments, StartInTrayArgument)
	}
}
