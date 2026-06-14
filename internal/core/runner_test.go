package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunJobWritesLogFile(t *testing.T) {
	logsDir := t.TempDir()
	job := Job{
		ID:      42,
		Name:    "Hello Test",
		Command: echoCommand("hello from test"),
	}

	record := RunJob(context.Background(), &job, "Manual", logsDir)
	if record.LogFile == "" {
		t.Fatal("expected log file path")
	}
	if filepath.Dir(record.LogFile) != logsDir {
		t.Fatalf("expected log in %q, got %q", logsDir, record.LogFile)
	}
	if !strings.Contains(filepath.Base(record.LogFile), "Hello_Test") {
		t.Fatalf("expected job name in log filename, got %q", record.LogFile)
	}

	data, err := os.ReadFile(record.LogFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"trigger: Manual", "job_name: Hello Test", "hello from test"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected log content to contain %q, got:\n%s", want, content)
		}
	}
}
