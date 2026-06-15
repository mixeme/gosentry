//go:build windows

package core

import "testing"

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
