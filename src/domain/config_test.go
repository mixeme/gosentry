package domain

import "testing"

func TestAutostartArguments(t *testing.T) {
	if got := AutostartArguments(true); got != StartInTrayArgument {
		t.Errorf("AutostartArguments(true) = %q, want %q", got, StartInTrayArgument)
	}
	if got := AutostartArguments(false); got != "" {
		t.Errorf("AutostartArguments(false) = %q, want empty", got)
	}
}

func TestResolveStartHidden(t *testing.T) {
	cases := []struct {
		cli, keep, want bool
	}{
		{true, true, true},
		{true, false, false},
		{false, true, false},
		{false, false, false},
	}
	for _, tc := range cases {
		if got := ResolveStartHidden(tc.cli, tc.keep); got != tc.want {
			t.Errorf("ResolveStartHidden(%v, %v) = %v, want %v", tc.cli, tc.keep, got, tc.want)
		}
	}
}

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
