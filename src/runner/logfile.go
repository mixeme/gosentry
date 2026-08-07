package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

func writeRunLog(logsDir string, job domain.Job, trigger string, state string, detail string, output string, durationMS int64, started time.Time) (string, error) {
	if strings.TrimSpace(logsDir) == "" {
		return "", errors.New("logs directory is empty")
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return "", fmt.Errorf("create logs directory: %w", err)
	}
	// The timestamp comes first so a plain directory listing is naturally sorted
	// by run time. The job name is included for human scanning, but sanitized to
	// avoid characters that are invalid on Windows or awkward on shells.
	fileName := started.Format("20060102-150405") + "_" + sanitizeFileName(job.Name) + ".log"
	path := uniqueLogPath(logsDir, fileName)
	content := fmt.Sprintf("time: %s\njob_id: %d\njob_name: %s\ntrigger: %s\nstate: %s\ndetail: %s\nduration: %d\ncommand: %s\narguments: %s\nstart_only: %t\n\n%s\n",
		started.Format("2006-01-02 15:04:05"), job.ID, job.Name, trigger, state, detail, durationMS, job.Command, LogArguments(job.Arguments), job.StartOnly, output)
	if err := writeFileAtomic(logsDir, path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write log file: %w", err)
	}
	return path, nil
}

// writeFileAtomic writes data to a temp file in dir, then renames it over
// path. Rename is atomic within a volume on both supported platforms, so a
// crash or a killed process mid-write can never leave path holding a
// truncated log file the way a direct os.WriteFile could.
func writeFileAtomic(dir, path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	success = true
	return nil
}

// uniqueLogPath returns a path for fileName in dir, appending a disambiguating
// "-2", "-3", … suffix before the extension if the plain name is already
// taken. Two runs of the same job in the same second — a fast manual re-run,
// or a sub-second queue drain — would otherwise share one timestamp and the
// second write would silently overwrite the first.
func uniqueLogPath(dir, fileName string) string {
	path := filepath.Join(dir, fileName)
	if _, err := os.Stat(path); err != nil {
		return path
	}
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	for n := 2; ; n++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, n, ext))
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "job"
	}
	var builder strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "job"
	}
	return result
}
