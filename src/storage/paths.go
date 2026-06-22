package storage

import (
	"os"
	"path/filepath"
)

const (
	// The config file stays beside the executable so the portable build behaves
	// predictably: moving the program folder moves its settings with it.
	ConfigFileName = "gosentry.yaml"
	// Older builds were named PySentry. Keep the old config name readable during
	// the rename window so portable installations can start once and rewrite the
	// settings to gosentry.yaml without manual file copying.
	LegacyConfigFileName = "pysentry.yaml"
	// Jobs are kept in a separate YAML file because the user can choose a
	// different jobs directory, while application settings remain local to the
	// installed/copied program.
	JobsFileName = "jobs.yaml"
)

// Paths contains both the physical program location and the resolved runtime
// storage locations. Keeping resolved paths in one struct prevents the GUI and
// scheduler from interpreting relative directories differently.
type Paths struct {
	ExecutablePath string
	AppDir         string
	ConfigPath     string
	JobsDir        string
	JobsPath       string
	LogsDir        string
	DesktopIcon    string
}

func ResolvePaths() (Paths, error) {
	// os.Executable is used instead of the current working directory because GUI
	// apps are often launched from Explorer, a tray shortcut, or a desktop file.
	// In those cases the working directory can be surprising, but the executable
	// path is stable and matches the "portable app folder" storage model.
	executable, err := os.Executable()
	if err != nil {
		return Paths{}, err
	}

	appDir := filepath.Dir(executable)
	configPath := filepath.Join(appDir, ConfigFileName)
	return Paths{
		ExecutablePath: executable,
		AppDir:         appDir,
		ConfigPath:     configPath,
		JobsDir:        appDir,
		JobsPath:       filepath.Join(appDir, JobsFileName),
	}, nil
}
