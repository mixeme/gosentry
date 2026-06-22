//go:build windows

package autostart

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

func TestSameWindowsPathIgnoresCaseAndQuotes(t *testing.T) {
	if !sameWindowsPath(`"D:\Apps\GoSentry\gosentry.exe"`, `d:\apps\gosentry\gosentry.exe`) {
		t.Fatal("expected paths to match")
	}
}

func TestSameWindowsPathHandlesSpaces(t *testing.T) {
	if !sameWindowsPath(`"D:\Local Git\GoSentry\gosentry.exe"`, `d:\local git\gosentry\gosentry.exe`) {
		t.Fatal("expected paths with spaces to match")
	}
}

func TestSameWindowsPathStripsExtendedLengthPrefix(t *testing.T) {
	if !sameWindowsPath(`\\?\D:\Apps\GoSentry\gosentry.exe`, `D:\Apps\GoSentry\gosentry.exe`) {
		t.Fatal("expected \\\\?\\-prefixed path to match plain path")
	}
}

func TestSameWindowsPathMatchesShortNameViaFilesystem(t *testing.T) {
	// Create a file inside a directory whose name contains a space. On NTFS
	// systems that have 8.3 name generation enabled, Windows also assigns a
	// short name to the directory (e.g. "Local~1"). WScript.Shell may return
	// the long form while os.Executable returns the short form (or vice versa).
	// Verify that sameWindowsPath treats both representations as equal.
	tempDir := t.TempDir()
	dirWithSpace := filepath.Join(tempDir, "Local Git")
	if err := os.MkdirAll(dirWithSpace, 0755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	longPath := filepath.Join(dirWithSpace, "gosentry.exe")
	if err := os.WriteFile(longPath, []byte("test"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	// GetShortPathName converts the long path to its 8.3 equivalent when 8.3
	// names are available; it returns the unchanged path otherwise.
	p16, err := syscall.UTF16PtrFromString(longPath)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}
	buf := make([]uint16, syscall.MAX_PATH)
	n, err := syscall.GetShortPathName(p16, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Skipf("GetShortPathName: %v", err)
	}
	shortPath := syscall.UTF16ToString(buf[:n])

	if !sameWindowsPath(longPath, shortPath) {
		t.Fatalf("sameWindowsPath(%q, %q) = false; want true", longPath, shortPath)
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

func TestCreateStartupShortcutHandlesCyrillicPath(t *testing.T) {
	tempDir := t.TempDir()
	shortcutPath := filepath.Join(tempDir, "GoSentry.lnk")
	targetPath := filepath.Join(tempDir, "Программы и драйвера", "GoSentry", "gosentry.exe")
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
	if arguments != domain.StartInTrayArgument {
		t.Fatalf("shortcut arguments mismatch: got %q want %q", arguments, domain.StartInTrayArgument)
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
	if arguments != domain.StartInTrayArgument {
		t.Fatalf("shortcut arguments mismatch: got %q want %q", arguments, domain.StartInTrayArgument)
	}
}
