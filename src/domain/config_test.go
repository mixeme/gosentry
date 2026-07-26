package domain

import "testing"

// TestJobListViewIsCompact pins the normalization rule: only the exact
// "compact" value selects the one-line rows, so empty and unrecognised values
// (including configs written before the field existed) keep the detailed look.
func TestJobListViewIsCompact(t *testing.T) {
	cases := []struct {
		view JobListView
		want bool
	}{
		{JobListViewCompact, true},
		{JobListViewDetailed, false},
		{"", false},
		{"Compact", false},
		{"tiny", false},
	}
	for _, tc := range cases {
		if got := tc.view.IsCompact(); got != tc.want {
			t.Errorf("JobListView(%q).IsCompact() = %v, want %v", tc.view, got, tc.want)
		}
	}
}

func TestDefaultConfigUsesDetailedJobList(t *testing.T) {
	if got := DefaultConfig().JobListView; got != JobListViewDetailed {
		t.Errorf("default JobListView = %q, want %q", got, JobListViewDetailed)
	}
}
