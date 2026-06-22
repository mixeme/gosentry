package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

func writeRunLog(logsDir string, job domain.Job, trigger string, state string, detail string, output string, started time.Time) string {
	if strings.TrimSpace(logsDir) == "" {
		return ""
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return ""
	}
	// The timestamp comes first so a plain directory listing is naturally sorted
	// by run time. The job name is included for human scanning, but sanitized to
	// avoid characters that are invalid on Windows or awkward on shells.
	fileName := started.Format("20060102-150405") + "_" + sanitizeFileName(job.Name) + ".log"
	path := filepath.Join(logsDir, fileName)
	content := fmt.Sprintf("time: %s\njob_id: %d\njob_name: %s\ntrigger: %s\nstate: %s\ndetail: %s\ncommand: %s\narguments: %s\nstart_only: %t\n\n%s\n",
		started.Format("2006-01-02 15:04:05"), job.ID, job.Name, trigger, state, detail, job.Command, logArguments(job.Arguments), job.StartOnly, output)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ""
	}
	return path
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
