package core

import (
	"os"
	"path/filepath"
)

const (
	ConfigFileName = "pysentry.yaml"
	JobsFileName   = "jobs.yaml"
)

type Paths struct {
	AppDir     string
	ConfigPath string
	JobsDir    string
	JobsPath   string
	LogsDir    string
}

func ResolvePaths() (Paths, error) {
	executable, err := os.Executable()
	if err != nil {
		return Paths{}, err
	}

	appDir := filepath.Dir(executable)
	configPath := filepath.Join(appDir, ConfigFileName)
	return Paths{
		AppDir:     appDir,
		ConfigPath: configPath,
		JobsDir:    appDir,
		JobsPath:   filepath.Join(appDir, JobsFileName),
	}, nil
}
