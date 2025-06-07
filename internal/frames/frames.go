package frames

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// FrameInfo represents information about a single frame in the timelapse
type FrameInfo struct {
	Path     string
	Duration float64
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

// GenerateFrames takes an input pattern and framerate, and sends frame information through a channel.
// It returns an error channel that will receive any errors encountered during processing.
func GenerateFrames(pattern string, framerate int) (<-chan FrameInfo, <-chan error) {
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

		// Sort the matches to ensure consistent ordering
		sort.Strings(matches)

		// Calculate frame duration based on framerate
		frameDuration := 1.0 / float64(framerate)

		// Send all frames except the last one
		for i := 0; i < len(matches)-1; i++ {
			frameChan <- FrameInfo{
				Path:     matches[i],
				Duration: frameDuration,
			}
		}

		// Send the last frame without duration
		if len(matches) > 0 {
			frameChan <- FrameInfo{
				Path:     matches[len(matches)-1],
				Duration: 0, // Last frame doesn't need duration
			}
		}
	}()

	return frameChan, errChan
}