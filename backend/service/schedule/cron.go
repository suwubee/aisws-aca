package schedule

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CronSchedule struct {
	minute [60]bool
	hour   [24]bool
	day    [32]bool // 1-31
	month  [13]bool // 1-12
	week   [7]bool  // 0-6 (Sun=0)

	dayWildcard  bool
	weekWildcard bool
}

var monthNames = map[string]int{
	"jan": 1,
	"feb": 2,
	"mar": 3,
	"apr": 4,
	"may": 5,
	"jun": 6,
	"jul": 7,
	"aug": 8,
	"sep": 9,
	"oct": 10,
	"nov": 11,
	"dec": 12,
}

var weekNames = map[string]int{
	"sun": 0,
	"mon": 1,
	"tue": 2,
	"wed": 3,
	"thu": 4,
	"fri": 5,
	"sat": 6,
}

func expandCronMacros(expr string) string {
	switch strings.ToLower(strings.TrimSpace(expr)) {
	case "@hourly":
		return "0 * * * *"
	case "@daily", "@midnight":
		return "0 0 * * *"
	case "@weekly":
		return "0 0 * * 0"
	case "@monthly":
		return "0 0 1 * *"
	case "@yearly", "@annually":
		return "0 0 1 1 *"
	default:
		return expr
	}
}

func ParseCronExpression(expr string) (*CronSchedule, error) {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return nil, errors.New("cron expression is required")
	}
	raw = expandCronMacros(raw)

	fields := strings.Fields(raw)
	if len(fields) != 5 {
		return nil, fmt.Errorf("invalid cron expression: expected 5 fields, got %d", len(fields))
	}

	minute, _, err := parseField(fields[0], 0, 59, nil)
	if err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	hour, _, err := parseField(fields[1], 0, 23, nil)
	if err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	day, dayWildcard, err := parseField(fields[2], 1, 31, nil)
	if err != nil {
		return nil, fmt.Errorf("day-of-month: %w", err)
	}
	month, _, err := parseField(fields[3], 1, 12, monthNames)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	week, weekWildcard, err := parseField(fields[4], 0, 7, weekNames)
	if err != nil {
		return nil, fmt.Errorf("day-of-week: %w", err)
	}

	var weekNormalized [7]bool
	for i := 0; i < 7; i++ {
		weekNormalized[i] = week[i]
	}
	if week[7] {
		weekNormalized[0] = true
	}

	s := &CronSchedule{
		dayWildcard:  dayWildcard,
		weekWildcard: weekWildcard,
	}
	copy(s.minute[:], minute[:60])
	copy(s.hour[:], hour[:24])
	copy(s.day[:], day[:32])
	copy(s.month[:], month[:13])
	copy(s.week[:], weekNormalized[:])
	return s, nil
}

func (c *CronSchedule) Next(after time.Time, loc *time.Location) (time.Time, error) {
	location := loc
	if location == nil {
		location = time.Local
	}

	base := after.In(location)
	t := base.Truncate(time.Minute).Add(time.Minute)

	// 2 years minute-level search guard.
	const maxMinutes = 2 * 366 * 24 * 60
	for i := 0; i < maxMinutes; i++ {
		if c.matches(t) {
			return t, nil
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, errors.New("no matching time found within search window")
}

func (c *CronSchedule) matches(t time.Time) bool {
	min := t.Minute()
	hr := t.Hour()
	mon := int(t.Month())
	day := t.Day()
	week := int(t.Weekday())

	if min < 0 || min >= len(c.minute) || !c.minute[min] {
		return false
	}
	if hr < 0 || hr >= len(c.hour) || !c.hour[hr] {
		return false
	}
	if mon < 1 || mon >= len(c.month) || !c.month[mon] {
		return false
	}

	dayMatch := day >= 1 && day < len(c.day) && c.day[day]
	weekMatch := week >= 0 && week < len(c.week) && c.week[week]

	switch {
	case c.dayWildcard && c.weekWildcard:
		return true
	case c.dayWildcard:
		return weekMatch
	case c.weekWildcard:
		return dayMatch
	default:
		// Standard (Vixie) cron semantics: day-of-month OR day-of-week.
		return dayMatch || weekMatch
	}
}

func parseField(token string, min int, max int, names map[string]int) ([]bool, bool, error) {
	t := strings.TrimSpace(token)
	if t == "" {
		return nil, false, errors.New("field is empty")
	}
	if t == "*" {
		allowed := make([]bool, max+1)
		for i := min; i <= max; i++ {
			allowed[i] = true
		}
		return allowed, true, nil
	}

	allowed := make([]bool, max+1)
	hasAny := false

	parts := strings.Split(t, ",")
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			return nil, false, errors.New("empty list element")
		}

		base := p
		step := 1
		if strings.Contains(p, "/") {
			chunks := strings.Split(p, "/")
			if len(chunks) != 2 {
				return nil, false, errors.New("invalid step syntax")
			}
			base = strings.TrimSpace(chunks[0])
			stepValue := strings.TrimSpace(chunks[1])
			if stepValue == "" {
				return nil, false, errors.New("missing step value")
			}
			parsed, err := strconv.Atoi(stepValue)
			if err != nil || parsed <= 0 {
				return nil, false, errors.New("invalid step value")
			}
			step = parsed
		}

		if base == "*" {
			for i := min; i <= max; i += step {
				allowed[i] = true
				hasAny = true
			}
			continue
		}

		if strings.Contains(base, "-") {
			rangeParts := strings.Split(base, "-")
			if len(rangeParts) != 2 {
				return nil, false, errors.New("invalid range syntax")
			}
			start, err := parseValue(rangeParts[0], names)
			if err != nil {
				return nil, false, err
			}
			end, err := parseValue(rangeParts[1], names)
			if err != nil {
				return nil, false, err
			}
			if start > end {
				return nil, false, errors.New("range start must be <= end")
			}
			if start < min || end > max {
				return nil, false, fmt.Errorf("range out of bounds (%d-%d)", min, max)
			}
			for i := start; i <= end; i += step {
				allowed[i] = true
				hasAny = true
			}
			continue
		}

		value, err := parseValue(base, names)
		if err != nil {
			return nil, false, err
		}
		if value < min || value > max {
			return nil, false, fmt.Errorf("value out of bounds (%d-%d)", min, max)
		}
		allowed[value] = true
		hasAny = true
	}

	if !hasAny {
		return nil, false, errors.New("field has no values")
	}

	return allowed, false, nil
}

func parseValue(raw string, names map[string]int) (int, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, errors.New("value is empty")
	}
	if names != nil {
		if v, ok := names[strings.ToLower(s)]; ok {
			return v, nil
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", s)
	}
	return n, nil
}
