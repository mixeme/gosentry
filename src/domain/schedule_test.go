package domain

import (
	"testing"
	"time"
)

func TestParseRejectsInvalidSchedules(t *testing.T) {
	cases := []struct {
		spec string
		desc string
	}{
		{"", "empty string"},
		{"   ", "whitespace only"},
		{"@every", "bare @every without duration"},
		{"@every ", "@every with trailing space but no duration"},
		{"@every xyz", "invalid @every duration string"},
		{"@every -1s", "negative @every duration"},
		{"@every 0s", "zero @every duration"},
		{"not-a-cron", "invalid cron expression"},
		{"60 * * * *", "cron minute out of range"},
		{"* * * *", "too few cron fields"},
	}
	for _, tc := range cases {
		if _, err := Parse(tc.spec); err == nil {
			t.Errorf("Parse(%q) [%s]: expected error, got nil", tc.spec, tc.desc)
		}
		if err := Validate(tc.spec); err == nil {
			t.Errorf("Validate(%q) [%s]: expected error, got nil", tc.spec, tc.desc)
		}
	}
}

func TestParseEveryInterval(t *testing.T) {
	from := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	s, err := Parse("@every 10s")
	if err != nil {
		t.Fatalf("Parse(@every 10s): unexpected error: %v", err)
	}
	if got, want := s.Next(from), from.Add(10*time.Second); !got.Equal(want) {
		t.Fatalf("Next: got %s, want %s", got, want)
	}
}

func TestParseEveryTrimsSurroundingWhitespace(t *testing.T) {
	from := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	s, err := Parse("  @every 90m  ")
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	if got, want := s.Next(from), from.Add(90*time.Minute); !got.Equal(want) {
		t.Fatalf("Next: got %s, want %s", got, want)
	}
}

func TestParseCronExpression(t *testing.T) {
	from := time.Date(2026, 6, 14, 12, 3, 0, 0, time.UTC)
	s, err := Parse("*/5 * * * *")
	if err != nil {
		t.Fatalf("Parse(*/5 * * * *): unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 14, 12, 5, 0, 0, time.UTC)
	if got := s.Next(from); !got.Equal(want) {
		t.Fatalf("Next: got %s, want %s", got, want)
	}
}

func TestParseCronDescriptor(t *testing.T) {
	from := time.Date(2026, 6, 14, 12, 3, 0, 0, time.UTC)
	s, err := Parse("@daily")
	if err != nil {
		t.Fatalf("Parse(@daily): unexpected error: %v", err)
	}
	want := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if got := s.Next(from); !got.Equal(want) {
		t.Fatalf("Next: got %s, want %s", got, want)
	}
}

func TestValidateAcceptsValidSchedules(t *testing.T) {
	for _, spec := range []string{"@every 1s", "*/5 * * * *", "0 9 * * 1", "@hourly"} {
		if err := Validate(spec); err != nil {
			t.Errorf("Validate(%q): unexpected error: %v", spec, err)
		}
	}
}

func TestZeroScheduleNextIsZero(t *testing.T) {
	var s Schedule
	if got := s.Next(time.Now()); !got.IsZero() {
		t.Fatalf("zero Schedule Next: got %s, want zero time", got)
	}
}

func TestStringReturnsTrimmedSpec(t *testing.T) {
	s, err := Parse("  */5 * * * *  ")
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	if got, want := s.String(), "*/5 * * * *"; got != want {
		t.Fatalf("String: got %q, want %q", got, want)
	}
}
