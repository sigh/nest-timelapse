package frames

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sigh/nest-timelapse/internal/parsetime"
)

const maxFPS = 60

// FrameInfo represents information about a single frame in the timelapse
type FrameInfo struct {
	Path     string        // Location of the image
	Duration time.Duration // Duration of the frame
	Time     time.Time     // Time when the frame was captured
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
		return fmt.Sprintf("file 'file://%s'\nduration %f", escapedFile, f.Duration.Seconds())
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

// GenerateFrames generates frame information for the timelapse, filtering by time range if provided.
// Returns a channel of frames and an error channel.
func GenerateFrames(pattern string, speedup float64, timeRange *parsetime.TimeRange) (<-chan FrameInfo, <-chan error) {
	frameChan := make(chan FrameInfo)
	errChan := make(chan error, 1)

	go func() {
		defer close(frameChan)
		defer close(errChan)

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
		var validFrames []FrameInfo
		for _, match := range matches {
			t, err := parseFrameTime(match)
			if err != nil {
				errChan <- fmt.Errorf("invalid timestamp in filename %s: %v", match, err)
				return
			}
			if timeRange != nil {
				if t.Before(timeRange.Start) || t.After(timeRange.End) {
					continue
				}
			}
			validFrames = append(validFrames, FrameInfo{
				Path: match,
				Time: t,
			})
		}

		if len(validFrames) == 0 {
			errChan <- fmt.Errorf("no frames found within specified time range")
			return
		}

		// Sort frames by timestamp
		sort.Slice(validFrames, func(i, j int) bool {
			return validFrames[i].Time.Before(validFrames[j].Time)
		})

		const minFrameDuration = time.Second / time.Duration(maxFPS) // Minimum frame duration for maxFPS
		currentFrame := &validFrames[0]

		// Process frames
		for i := 1; i < len(validFrames); i++ {
			nextTime := validFrames[i].Time
			duration := nextTime.Sub(currentFrame.Time) / time.Duration(speedup)

			// Skip this frame if it would play faster than maxFPS
			if duration < minFrameDuration {
				continue
			}

			// Now we know the duration, output the current frame
			currentFrame.Duration = duration
			frameChan <- *currentFrame

			// Update currentFrame to the next frame
			currentFrame = &validFrames[i]
		}

		// Output the last frame with duration 0
		currentFrame.Duration = 0
		frameChan <- *currentFrame
	}()

	return frameChan, errChan
}