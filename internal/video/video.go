package video

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	imageFilePrefix    = "nest_camera_frame_"
	imageFileExtension = "jpg"
	// timeFormat is used for generating unique filenames
	timeFormat = "20060102_150405"
)

// ExtractFirstFrame uses ffmpeg to extract the first frame from H264 data in memory
func ExtractFirstFrame(h264Data *bytes.Buffer, outputDir string) error {
	now := time.Now()

	// Create year/month/day directory structure
	dateDir := filepath.Join(outputDir,
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		fmt.Sprintf("%02d", now.Day()),
	)

	// Create the directory structure if it doesn't exist
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	timestamp := now.Format(timeFormat)
	filename := fmt.Sprintf("%s%s.%s", imageFilePrefix, timestamp, imageFileExtension)
	imagePath := filepath.Join(dateDir, filename)

	// Prepare ffmpeg command to read from stdin
	cmd := exec.CommandContext(context.Background(), "ffmpeg",
		"-f", "h264", // Input format is H264
		"-i", "pipe:0", // Read from stdin
		"-update", "1",
		"-frames:v", "1",
		imagePath,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	if _, err := io.Copy(stdin, h264Data); err != nil {
		return fmt.Errorf("failed to write to ffmpeg: %w", err)
	}
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	output, err := io.ReadAll(io.MultiReader(stdout, stderr))
	if err != nil {
		return fmt.Errorf("failed to read ffmpeg output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to extract frame: %w\nffmpeg output: %s", err, string(output))
	}

	fmt.Printf("Extracted first frame to: %s\n", imagePath)
	return nil
}
