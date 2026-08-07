package runner

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUniqueLogPathAvoidsCollision pins the fix for two runs of the same job
// landing on the same second: without disambiguation the second write would
// silently overwrite the first.
func TestUniqueLogPathAvoidsCollision(t *testing.T) {
	dir := t.TempDir()
	const name = "20260101-120000_job.log"

	first := uniqueLogPath(dir, name)
	if first != filepath.Join(dir, name) {
		t.Fatalf("first call: got %q, want the plain name", first)
	}
	if err := os.WriteFile(first, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}

	second := uniqueLogPath(dir, name)
	if second == first {
		t.Fatalf("second call returned the same path as an existing file: %q", second)
	}
	if err := os.WriteFile(second, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}

	third := uniqueLogPath(dir, name)
	if third == first || third == second {
		t.Fatalf("third call collided with an existing file: %q (existing: %q, %q)", third, first, second)
	}
}
