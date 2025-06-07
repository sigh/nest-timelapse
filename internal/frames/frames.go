package frames

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sigh/nest-timelapse/internal/parsetime"
)

// FrameInfo represents information about a single frame in the timelapse
type FrameInfo struct {
	Path     string
	Duration float64
	Time     time.Time // Time when the frame was captured
}

// String returns the frame information formatted for ffmpeg concat demuxer
func (f FrameInfo) String() string {
	// Convert to absolute path
	absPath, err := filepath.Abs(f.Path)
	if err != nil {
		// If we can't get absolute path, use the original path
		absPath = f.Path
	}

	// Escape single quotes in the filename
	escapedFile := strings.ReplaceAll(absPath, "'", "'\\''")
	if f.Duration > 0 {
		return fmt.Sprintf("file 'file://%s'\nduration %f", escapedFile, f.Duration)
	}
	return fmt.Sprintf("file 'file://%s'", escapedFile)
}

// parseFrameTime extracts the timestamp from a frame filename
// Expected format: nest_camera_frame_YYYYMMDD_HHMMSS.jpg
func parseFrameTime(filename string) (time.Time, error) {
	base := filepath.Base(filename)
	parts := strings.Split(base, "_")
	if len(parts) < 4 {
		return time.Time{}, fmt.Errorf("invalid filename format: %s", filename)
	}

	// Get the date and time parts
	dateStr := parts[3]
	timeStr := strings.TrimSuffix(parts[4], filepath.Ext(parts[4]))

	// Parse the timestamp
	t, err := time.Parse("20060102_150405", dateStr+"_"+timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp in filename: %s", filename)
	}

	return t, nil
}

// GenerateFrames takes an input pattern, framerate, and time range, and sends frame information through a channel.
// It returns an error channel that will receive any errors encountered during processing.
func GenerateFrames(pattern string, framerate int, timeRange *parsetime.TimeRange) (<-chan FrameInfo, <-chan error) {
	frameChan := make(chan FrameInfo)
	errChan := make(chan error, 1)

	go func() {
		defer close(frameChan)
		defer close(errChan)

		// Get the list of files matching the pattern
		matches, err := filepath.Glob(pattern)
		if err != nil {
			errChan <- fmt.Errorf("invalid pattern: %v", err)
			return
		}
		if len(matches) == 0 {
			errChan <- fmt.Errorf("no files matching pattern '%s' found", pattern)
			return
		}

		// Parse timestamps and filter by time range
		var frames []FrameInfo
		for _, match := range matches {
			t, err := parseFrameTime(match)
			if err != nil {
				errChan <- fmt.Errorf("error parsing frame time: %v", err)
				return
			}

			// Skip if outside time range
			if timeRange != nil {
				// Check if we have a start time and the frame is before it
				if !timeRange.Start.IsZero() && t.Before(timeRange.Start) {
					continue
				}
				// Check if we have an end time and the frame is after it
				if !timeRange.End.IsZero() && t.After(timeRange.End) {
					continue
				}
			}

			frames = append(frames, FrameInfo{
				Path: match,
				Time: t,
			})
		}

		if len(frames) == 0 {
			errChan <- fmt.Errorf("no frames found within specified time range")
			return
		}

		// Sort frames by timestamp
		sort.Slice(frames, func(i, j int) bool {
			return frames[i].Time.Before(frames[j].Time)
		})

		// Calculate frame duration based on framerate
		frameDuration := 1.0 / float64(framerate)

		// Send all frames except the last one
		for i := 0; i < len(frames)-1; i++ {
			frames[i].Duration = frameDuration
			frameChan <- frames[i]
		}

		// Send the last frame without duration
		if len(frames) > 0 {
			frames[len(frames)-1].Duration = 0 // Last frame doesn't need duration
			frameChan <- frames[len(frames)-1]
		}
	}()

	return frameChan, errChan
}