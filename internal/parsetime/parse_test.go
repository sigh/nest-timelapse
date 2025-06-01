package parsetime

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*testing.T, *time.Time)
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
			check: func(t *testing.T, got *time.Time) {
				if got != nil {
					t.Errorf("ParseTime() = %v, want nil", got)
				}
			},
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "time only",
			input:   "14:30",
			wantErr: false,
			check: func(t *testing.T, got *time.Time) {
				if got == nil {
					t.Fatal("ParseTime() = nil, want time")
				}
				now := time.Now()
				want := time.Date(now.Year(), now.Month(), now.Day(), 14, 30, 0, 0, now.Location())
				if !got.Equal(want) {
					t.Errorf("ParseTime() = %v, want %v", got, want)
				}
			},
		},
		{
			name:    "date only",
			input:   "2024-03-20",
			wantErr: false,
			check: func(t *testing.T, got *time.Time) {
				if got == nil {
					t.Fatal("ParseTime() = nil, want time")
				}
				want := time.Date(2024, 3, 20, 0, 0, 0, 0, time.Local)
				if !got.Equal(want) {
					t.Errorf("ParseTime() = %v, want %v", got, want)
				}
			},
		},
		{
			name:    "date and time with space",
			input:   "2024-03-20 14:30",
			wantErr: false,
			check: func(t *testing.T, got *time.Time) {
				if got == nil {
					t.Fatal("ParseTime() = nil, want time")
				}
				want := time.Date(2024, 3, 20, 14, 30, 0, 0, time.Local)
				if !got.Equal(want) {
					t.Errorf("ParseTime() = %v, want %v", got, want)
				}
			},
		},
		{
			name:    "date and time with underscore",
			input:   "2024-03-20_14:30",
			wantErr: false,
			check: func(t *testing.T, got *time.Time) {
				if got == nil {
					t.Fatal("ParseTime() = nil, want time")
				}
				want := time.Date(2024, 3, 20, 14, 30, 0, 0, time.Local)
				if !got.Equal(want) {
					t.Errorf("ParseTime() = %v, want %v", got, want)
				}
			},
		},
		{
			name:    "date and time with dot",
			input:   "2024-03-20.14:30",
			wantErr: false,
			check: func(t *testing.T, got *time.Time) {
				if got == nil {
					t.Fatal("ParseTime() = nil, want time")
				}
				want := time.Date(2024, 3, 20, 14, 30, 0, 0, time.Local)
				if !got.Equal(want) {
					t.Errorf("ParseTime() = %v, want %v", got, want)
				}
			},
		},
		{
			name:    "date and time with multiple separators",
			input:   "2024-03-20...14:30",
			wantErr: false,
			check: func(t *testing.T, got *time.Time) {
				if got == nil {
					t.Fatal("ParseTime() = nil, want time")
				}
				want := time.Date(2024, 3, 20, 14, 30, 0, 0, time.Local)
				if !got.Equal(want) {
					t.Errorf("ParseTime() = %v, want %v", got, want)
				}
			},
		},
		{
			name:    "time with seconds",
			input:   "14:30:45",
			wantErr: true,
		},
		{
			name:    "date and time with seconds",
			input:   "2024-03-20 14:30:45",
			wantErr: true,
		},
		{
			name:    "invalid date format",
			input:   "2024/03/20 14:30",
			wantErr: true,
		},
		{
			name:    "invalid time format",
			input:   "2024-03-20 14-30",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "invalid unit",
			input:   "1x",
			wantErr: true,
		},
		{
			name:    "missing unit",
			input:   "45",
			wantErr: true,
		},
		{
			name:  "weeks only",
			input: "2w",
			want:  2 * 7 * 24 * time.Hour,
		},
		{
			name:  "days only",
			input: "2d",
			want:  48 * time.Hour,
		},
		{
			name:  "hours only",
			input: "6h",
			want:  6 * time.Hour,
		},
		{
			name:  "minutes only",
			input: "30m",
			want:  30 * time.Minute,
		},
		{
			name:  "combined",
			input: "1d6h30m",
			want:  (24*time.Hour + 6*time.Hour + 30*time.Minute),
		},
		{
			name:  "multiple parts",
			input: "1d6h30m",
			want:  (24*time.Hour + 6*time.Hour + 30*time.Minute),
		},
		{
			name:  "decimal not allowed",
			input: "1.5h",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != nil && *got != tt.want {
				t.Errorf("ParseDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakeTimeRange(t *testing.T) {
	// Fixed times for testing
	startTime := time.Date(2024, 3, 20, 10, 0, 0, 0, time.Local)
	endTime := time.Date(2024, 3, 20, 11, 0, 0, 0, time.Local)
	oneHour := time.Hour
	epoch := time.Unix(0, 0)
	epsilon := 30 * time.Second

	// Helper function to check if two times are within epsilon
	timeEqual := func(t *testing.T, got, want time.Time, msg string) {
		diff := got.Sub(want)
		if diff < -epsilon || diff > epsilon {
			t.Errorf("%s = %v, want %v (diff: %v)", msg, got, want, diff)
		}
	}

	tests := []struct {
		name    string
		start   *time.Time
		end     *time.Time
		dur     *time.Duration
		want    *TimeRange
		wantErr bool
	}{
		{
			name:  "both times provided",
			start: &startTime,
			end:   &endTime,
			dur:   nil,
			want: &TimeRange{
				Start: startTime,
				End:   endTime,
			},
			wantErr: false,
		},
		{
			name:    "both times with duration (error)",
			start:   &startTime,
			end:     &endTime,
			dur:     &oneHour,
			wantErr: true,
		},
		{
			name:    "end before start",
			start:   &endTime,
			end:     &startTime,
			dur:     nil,
			wantErr: true,
		},
		{
			name:  "start with duration",
			start: &startTime,
			end:   nil,
			dur:   &oneHour,
			want: &TimeRange{
				Start: startTime,
				End:   startTime.Add(oneHour),
			},
			wantErr: false,
		},
		{
			name:  "end with duration",
			start: nil,
			end:   &endTime,
			dur:   &oneHour,
			want: &TimeRange{
				Start: endTime.Add(-oneHour),
				End:   endTime,
			},
			wantErr: false,
		},
		{
			name:  "duration only",
			start: nil,
			end:   nil,
			dur:   &oneHour,
			want: &TimeRange{
				Start: time.Now().Add(-oneHour),
				End:   time.Now(),
			},
			wantErr: false,
		},
		{
			name:  "start only (no duration)",
			start: &startTime,
			end:   nil,
			dur:   nil,
			want: &TimeRange{
				Start: startTime,
				End:   time.Now(),
			},
			wantErr: false,
		},
		{
			name:  "end only (no duration)",
			start: nil,
			end:   &endTime,
			dur:   nil,
			want: &TimeRange{
				Start: epoch,
				End:   endTime,
			},
			wantErr: false,
		},
		{
			name:  "no times or duration",
			start: nil,
			end:   nil,
			dur:   nil,
			want: &TimeRange{
				Start: epoch,
				End:   time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MakeTimeRange(tt.start, tt.end, tt.dur)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeTimeRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// For all tests, check both start and end times with epsilon
			timeEqual(t, got.Start, tt.want.Start, "MakeTimeRange() start")
			timeEqual(t, got.End, tt.want.End, "MakeTimeRange() end")
		})
	}
}
