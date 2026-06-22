package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// cronParser accepts standard five-field cron expressions (minute, hour, day of
// month, month, day of week) plus descriptors such as "@daily". It is the single
// source of truth for what GoSentry considers a valid cron schedule.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// everyPrefix marks the "@every <duration>" form, which is kept alongside cron
// because it is convenient for quick tests and for simple intervals that are
// awkward to express as five fields.
const everyPrefix = "@every "

// Schedule is a parsed, validated job schedule. It supports two forms:
//
//   - "@every <duration>" intervals (e.g. "@every 10s"), and
//   - standard five-field cron expressions (e.g. "*/5 * * * *").
//
// Parsing once and reusing the value avoids re-validating and re-parsing the
// same string on every scheduler tick. A zero Schedule is invalid; its Next
// method returns the zero time.
type Schedule struct {
	raw   string
	every time.Duration // > 0 when the schedule is an "@every" interval
	cron  cron.Schedule  // non-nil when the schedule is a cron expression
}

// Parse validates spec and returns a reusable Schedule. It returns an error
// describing why the schedule is unusable, which callers can surface to the user.
func Parse(spec string) (Schedule, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return Schedule{}, fmt.Errorf("schedule is empty")
	}
	if strings.HasPrefix(trimmed, everyPrefix) {
		interval, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(trimmed, everyPrefix)))
		if err != nil {
			return Schedule{}, fmt.Errorf("invalid %q duration: %w", strings.TrimSpace(everyPrefix), err)
		}
		if interval <= 0 {
			return Schedule{}, fmt.Errorf("%q duration must be positive, got %s", strings.TrimSpace(everyPrefix), interval)
		}
		return Schedule{raw: trimmed, every: interval}, nil
	}
	// robfig/cron handles edge cases such as ranges, steps, and day-of-week names,
	// keeping GoSentry compatible with the mental model users know from Unix cron.
	parsed, err := cronParser.Parse(trimmed)
	if err != nil {
		return Schedule{}, fmt.Errorf("invalid cron expression: %w", err)
	}
	return Schedule{raw: trimmed, cron: parsed}, nil
}

// Validate reports whether spec is a usable schedule string. It is a convenience
// wrapper around Parse for callers (such as form validation) that only need the
// yes/no answer and the error message.
func Validate(spec string) error {
	_, err := Parse(spec)
	return err
}

// Next returns the next time the schedule fires strictly after from. For an
// "@every" interval this is from plus the interval; for a cron expression it is
// the cron library's next matching time. A zero (unparsed) Schedule returns the
// zero time.
func (s Schedule) Next(from time.Time) time.Time {
	switch {
	case s.every > 0:
		return from.Add(s.every)
	case s.cron != nil:
		return s.cron.Next(from)
	default:
		return time.Time{}
	}
}

// String returns the original, trimmed schedule specification.
func (s Schedule) String() string {
	return s.raw
}
