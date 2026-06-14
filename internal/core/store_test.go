package core

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestJobsYAMLDoesNotPersistRuntimeNoise(t *testing.T) {
	jobs := []Job{
		{
			ID:        1,
			Name:      "Clean job",
			Schedule:  "@every 10s",
			Command:   echoCommand("ok"),
			Enabled:   true,
			LastRun:   "2026-06-14 12:00:00",
			NextRun:   "2026-06-14 12:00:10",
			LastState: "OK",
			Output:    "stdout: ok",
			Logs: []RunRecord{
				{Time: "2026-06-14 12:00:00", JobName: "Clean job", Output: "stdout: ok"},
			},
		},
	}

	data, err := yaml.Marshal(JobsFile{Jobs: jobs})
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, unwanted := range []string{"last_run", "next_run", "last_state", "activity", "last_output", "stdout"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("jobs yaml should not contain %q:\n%s", unwanted, text)
		}
	}
}
