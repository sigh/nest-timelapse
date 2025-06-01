package parsetime

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseTime parses a time string in format "HH:MM", "YYYY-MM-DD", or "YYYY-MM-DD HH:MM" (separator can be any non-alphanumeric character except colon)
func ParseTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}

	// First check if it's a time-only format (HH:MM)
	if strings.Contains(value, ":") && !strings.Contains(value, "-") {
		if t, err := time.ParseInLocation("15:04", value, time.Local); err == nil {
			// Use today's date, set seconds to 0
			now := time.Now()
			t = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
			return &t, nil
		}
		return nil, fmt.Errorf("invalid time format: %s (must be HH:MM)", value)
	}

	// For date or date+time, split on any non-alphanumeric character (except colon and hyphen)
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == ':' || r == '-')
	})

	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid time value: empty after splitting")
	}

	// Try parsing as date only (YYYY-MM-DD)
	if len(parts) == 1 {
		if t, err := time.ParseInLocation("2006-01-02", parts[0], time.Local); err == nil {
			// Set time to start of day (seconds already 0)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
			return &t, nil
		}
		return nil, fmt.Errorf("invalid date format: %s (must be YYYY-MM-DD)", parts[0])
	}

	// Try parsing as date and time (YYYY-MM-DD HH:MM)
	if len(parts) == 2 {
		date, timeStr := parts[0], parts[1]
		t, err := time.ParseInLocation("2006-01-02", date, time.Local)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %s (must be YYYY-MM-DD)", date)
		}
		if timeVal, err := time.ParseInLocation("15:04", timeStr, time.Local); err == nil {
			// Set time components, seconds to 0
			t = time.Date(t.Year(), t.Month(), t.Day(), timeVal.Hour(), timeVal.Minute(), 0, 0, time.Local)
			return &t, nil
		}
		return nil, fmt.Errorf("invalid time format: %s (must be HH:MM)", timeStr)
	}

	return nil, fmt.Errorf("invalid time value: must be in format 'HH:MM', 'YYYY-MM-DD', or 'YYYY-MM-DD HH:MM' (separator can be any non-alphanumeric character except colon and hyphen)")
}

// ParseDuration parses a duration string supporting weeks, days, hours, and minutes (e.g. "2w3d6h30m")
func ParseDuration(value string) (*time.Duration, error) {
	if value == "" {
		return nil, nil
	}

	// Split the string into parts by finding transitions from digits to units
	var parts []string
	var current strings.Builder
	var inNumber bool

	for _, r := range value {
		isDigit := r >= '0' && r <= '9'
		isUnit := r == 'w' || r == 'd' || r == 'h' || r == 'm'

		if !inNumber && isDigit {
			// Start of a new number
			inNumber = true
			current.WriteRune(r)
		} else if inNumber && isDigit {
			// Continue number
			current.WriteRune(r)
		} else if inNumber && isUnit {
			// End of number, found unit
			current.WriteRune(r)
			parts = append(parts, current.String())
			current.Reset()
			inNumber = false
		} else if !isDigit && !isUnit {
			// Invalid character
			return nil, fmt.Errorf("invalid character in duration: %c (must be digits or units w,d,h,m)", r)
		}
	}

	// Check if we ended in the middle of a number
	if inNumber {
		return nil, fmt.Errorf("incomplete duration: missing unit after %s", current.String())
	}

	var total time.Duration
	for _, part := range parts {
		// Extract the number and unit
		num := strings.TrimRightFunc(part, func(r rune) bool { return r < '0' || r > '9' })
		unit := strings.TrimLeftFunc(part, func(r rune) bool { return r >= '0' && r <= '9' })

		n, err := strconv.Atoi(num)
		if err != nil {
			return nil, fmt.Errorf("invalid number in duration: %v", err)
		}

		switch unit {
		case "w":
			total += time.Duration(n) * 7 * 24 * time.Hour
		case "d":
			total += time.Duration(n) * 24 * time.Hour
		case "h":
			total += time.Duration(n) * time.Hour
		case "m":
			total += time.Duration(n) * time.Minute
		default:
			return nil, fmt.Errorf("invalid unit in duration: %s (must be w, d, h, or m)", unit)
		}
	}

	return &total, nil
}

// TimeRange represents a start and end time pair
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// MakeTimeRange combines start time, end time, and duration into a complete time range.
func MakeTimeRange(start, end *time.Time, duration *time.Duration) (*TimeRange, error) {
	// If both start and end are provided, use them directly (but error if duration is also provided)
	if start != nil && end != nil {
		if duration != nil {
			return nil, fmt.Errorf("cannot provide duration when both start and end times are specified")
		}
		if end.Before(*start) {
			return nil, fmt.Errorf("end time %v is before start time %v", end, start)
		}
		return &TimeRange{Start: *start, End: *end}, nil
	}

	now := time.Now()
	epoch := time.Unix(0, 0)  // January 1, 1970 UTC

	// Handle all cases where duration exists
	if duration != nil {
		if start != nil {
			// Duration + start: calculate end
			return &TimeRange{
				Start: *start,
				End:   start.Add(*duration),
			}, nil
		}
		if end != nil {
			// Duration + end: calculate start
			return &TimeRange{
				Start: end.Add(-*duration),
				End:   *end,
			}, nil
		}
		// Duration only: use current time as end
		return &TimeRange{
			Start: now.Add(-*duration),
			End:   now,
		}, nil
	}

	// When no duration is provided, use defaults for missing times
	startTime := start
	if startTime == nil {
		startTime = &epoch
	}
	endTime := end
	if endTime == nil {
		endTime = &now
	}
	return &TimeRange{
		Start: *startTime,
		End:   *endTime,
	}, nil
}