package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

func loadOrCreateConfig(paths Paths) (domain.Config, error) {
	// Defaults favor a portable installation: settings and jobs begin next to the
	// executable, while logs are grouped under a dedicated subdirectory.
	config := domain.DefaultConfig()

	if _, err := os.Stat(paths.ConfigPath); errors.Is(err, os.ErrNotExist) {
		return config, writeJSON(paths.ConfigPath, config)
	}

	data, err := os.ReadFile(paths.ConfigPath)
	if err != nil {
		return domain.Config{}, err
	}
	// Clearing the default first keeps "the file sets jobs_file" distinguishable
	// from "the file omits it", which the jobs_dir migration below depends on.
	// The fallbacks restore a value in either case.
	config.JobsFile = ""
	if err := json.Unmarshal(data, &config); err != nil {
		return domain.Config{}, err
	}

	// A config written before the setting named a file carries jobs_dir instead
	// of jobs_file. Keep its meaning by appending the fixed name that version
	// used, then drop the old key so the file is rewritten in the current shape.
	if strings.TrimSpace(config.JobsFile) == "" && strings.TrimSpace(config.JobsDir) != "" {
		config.JobsFile = filepath.Join(config.JobsDir, JobsFileName)
	}
	config.JobsDir = ""
	if strings.TrimSpace(config.JobsFile) == "" {
		// Empty paths are treated as missing values rather than intentional root
		// directories. This avoids accidentally writing jobs to unexpected places.
		config.JobsFile = JobsFileName
	}
	if strings.TrimSpace(config.LogsDir) == "" {
		config.LogsDir = "logs"
	}
	// MaxLogFiles and MaxLogAgeDays are deliberately not normalized: 0 means
	// "keep everything" (see runner.CleanupLogs), not a missing value, so
	// backfilling it here would make that choice impossible to persist. A config
	// written before either field existed already carries 0 from json.Unmarshal
	// leaving the DefaultConfig() value in config untouched, so old files still
	// pick up 100 / 30 without an explicit backfill.
	if config.ExecutionMode == "" {
		config.ExecutionMode = domain.ExecutionModeParallel
	}
	if config.OverlapPolicy == "" {
		config.OverlapPolicy = domain.OverlapPolicySkip
	}
	// DefaultTimeoutSeconds is deliberately not normalized: 0 is a meaningful
	// value ("no timeout"), not a missing one, so backfilling it here would make
	// the setting impossible to persist. Negative values are rejected by
	// app.validateConfig before they can be saved.
	if config.Theme == "" {
		config.Theme = domain.ThemeGoSentry
	}
	if config.Theme == "default" {
		config.Theme = domain.ThemeSystem
	}
	return config, nil
}
