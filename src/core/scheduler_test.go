package core

import (
	"testing"
	"time"
)

func TestNextRunTimeSupportsEvery(t *testing.T) {
	from := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	next, ok := nextRunTime("@every 10s", from)
	if !ok {
		t.Fatal("expected @every schedule to parse")
	}
	if want := from.Add(10 * time.Second); !next.Equal(want) {
		t.Fatalf("expected %s, got %s", want, next)
	}
}

func TestNextRunTimeSupportsCron(t *testing.T) {
	from := time.Date(2026, 6, 14, 12, 3, 0, 0, time.UTC)
	next, ok := nextRunTime("*/5 * * * *", from)
	if !ok {
		t.Fatal("expected cron schedule to parse")
	}
	want := time.Date(2026, 6, 14, 12, 5, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("expected %s, got %s", want, next)
	}
}
