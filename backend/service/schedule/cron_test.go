package schedule

import (
	"testing"
	"time"
)

func TestParseCronExpression_BasicAndNext(t *testing.T) {
	s, err := ParseCronExpression("* * * * *")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	base := time.Date(2026, 1, 1, 0, 0, 30, 0, time.UTC)
	next, err := s.Next(base, time.UTC)
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	want := time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("unexpected next.\nwant: %s\ngot:  %s", want, next)
	}
}

func TestParseCronExpression_Macros(t *testing.T) {
	s, err := ParseCronExpression("@hourly")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	base := time.Date(2026, 1, 1, 12, 34, 0, 0, time.UTC)
	next, err := s.Next(base, time.UTC)
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	want := time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("unexpected next.\nwant: %s\ngot:  %s", want, next)
	}
}

func TestParseCronExpression_WeekdayAndMonthNames(t *testing.T) {
	s, err := ParseCronExpression("0 9 * jan mon")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// 2026-01-05 is Monday.
	base := time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC)
	next, err := s.Next(base, time.UTC)
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	want := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("unexpected next.\nwant: %s\ngot:  %s", want, next)
	}
}

func TestParseCronExpression_DayOfMonthOrDayOfWeekSemantics(t *testing.T) {
	s, err := ParseCronExpression("0 0 1 * 0")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Vixie cron semantics: when both day-of-month and day-of-week are restricted,
	// it triggers when either matches (OR).
	// Base: 2026-02-01 00:00:00 (Sunday AND day 1) -> next should be next Sunday.
	base := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	next, err := s.Next(base, time.UTC)
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	want := time.Date(2026, 2, 8, 0, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("unexpected next.\nwant: %s\ngot:  %s", want, next)
	}

	// From 2026-01-02, next should be 2026-01-04 (Sunday) at 00:00, not waiting for day 1.
	base = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	next, err = s.Next(base, time.UTC)
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	want = time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("unexpected next.\nwant: %s\ngot:  %s", want, next)
	}
}

func TestParseCronExpression_Invalid(t *testing.T) {
	if _, err := ParseCronExpression(""); err == nil {
		t.Fatalf("expected error for empty expression")
	}
	if _, err := ParseCronExpression("* * * *"); err == nil {
		t.Fatalf("expected error for missing fields")
	}
	if _, err := ParseCronExpression("61 * * * *"); err == nil {
		t.Fatalf("expected error for invalid minute")
	}
	if _, err := ParseCronExpression("0 0 0 * *"); err == nil {
		t.Fatalf("expected error for invalid day-of-month 0")
	}
}
