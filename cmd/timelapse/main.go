package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sigh/nest-timelapse/internal/frames"
	"github.com/sigh/nest-timelapse/internal/parsetime"
)

type CropRange struct {
	Start float64
	End   float64
}

type Config struct {
	Speedup     float64
	OutputFile  string
	Overwrite   bool
	InputDir    string
	CropX       *CropRange
	CropY       *CropRange
	TimeRange   *parsetime.TimeRange
}

// FrameInfo represents information about a single frame in the timelapse
type FrameInfo struct {
	Path     string
	Duration float64
}

// String returns the frame information formatted for ffmpeg concat demuxer
func (f FrameInfo) String() string {
	// Escape single quotes in the filename
	escapedFile := strings.ReplaceAll(f.Path, "'", "'\\''")
	if f.Duration > 0 {
		return fmt.Sprintf("file '%s'\nduration %f", escapedFile, f.Duration)
	}
	return fmt.Sprintf("file '%s'", escapedFile)
}

func parseCropRange(value string, paramName string) (*CropRange, error) {
	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("%s must be in format 'start-end' (e.g. '0.4-0.6')", paramName)
	}

	start, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid start value in %s: %v", paramName, err)
	}

	end, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid end value in %s: %v", paramName, err)
	}

	if start < 0 || end > 1 || start >= end {
		return nil, fmt.Errorf("%s values must be between 0 and 1, and start must be less than end", paramName)
	}

	return &CropRange{start, end}, nil
}

func parseTimeRange(startTimeStr, endTimeStr, durationStr string) (*parsetime.TimeRange, error) {
	var startTime, endTime *time.Time
	var duration *time.Duration

	if startTimeStr != "" {
		t, err := parsetime.ParseTime(startTimeStr)
		if err != nil {
			return nil, err
		}
		startTime = t
	}

	if endTimeStr != "" {
		t, err := parsetime.ParseTime(endTimeStr)
		if err != nil {
			return nil, err
		}
		endTime = t
	}

	if durationStr != "" {
		d, err := parsetime.ParseDuration(durationStr)
		if err != nil {
			return nil, err
		}
		duration = d
	}

	return parsetime.MakeTimeRange(startTime, endTime, duration)
}

func parseArgs() (*Config, error) {
	config := &Config{
		Speedup:    3600, // Default to 3600x speedup (1 hour = 1 second)
		OutputFile: "timelapse.mp4",
		InputDir:   ".", // Default to current directory
	}

	var cropXStr, cropYStr string
	var startTimeStr, endTimeStr, durationStr string
	var speedupStr string

	flag.StringVar(&speedupStr, "speedup", "1h/1s", "Speedup ratio (e.g. '1h/1m' for 1 hour = 1 minute, '1d/30s' for 1 day = 30 seconds)")
	flag.StringVar(&speedupStr, "s", "1h/1s", "Speedup ratio (shorthand)")
	flag.StringVar(&config.OutputFile, "output", config.OutputFile, "Set the output file")
	flag.StringVar(&config.OutputFile, "o", config.OutputFile, "Set the output file (shorthand)")
	flag.BoolVar(&config.Overwrite, "y", false, "Overwrite output file if it exists")
	flag.BoolVar(&config.Overwrite, "overwrite", false, "Overwrite output file if it exists (long form)")
	flag.StringVar(&cropXStr, "crop-x", "", "Crop horizontally using width ratios (e.g. '0.4-0.6')")
	flag.StringVar(&cropYStr, "crop-y", "", "Crop vertically using height ratios (e.g. '0.4-0.6')")
	flag.StringVar(&startTimeStr, "start-time", "", "Start time (HH:MM:SS or YYYY-MM-DD HH:MM:SS)")
	flag.StringVar(&endTimeStr, "end-time", "", "End time (HH:MM:SS or YYYY-MM-DD HH:MM:SS)")
	flag.StringVar(&durationStr, "duration", "", "Duration (e.g. '1d6h30m', '2d', '6h30m')")

	// Add minimal usage message for the positional argument
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [input_directory]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "If input_directory is not provided, defaults to current directory\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Handle input directory as positional argument
	if flag.NArg() > 0 {
		config.InputDir = flag.Arg(0)
	}

	// Convert input directory to absolute path
	absInputDir, err := filepath.Abs(config.InputDir)
	if err != nil {
		return nil, fmt.Errorf("invalid input directory: %w", err)
	}
	config.InputDir = absInputDir

	// Parse speedup ratio
	speedup, err := parsetime.ParseSpeedup(speedupStr)
	if err != nil {
		return nil, fmt.Errorf("invalid speedup ratio: %v", err)
	}
	config.Speedup = speedup

	// Parse crop parameters
	if cropXStr != "" {
		cropX, err := parseCropRange(cropXStr, "crop-x")
		if err != nil {
			return nil, err
		}
		config.CropX = cropX
	}

	if cropYStr != "" {
		cropY, err := parseCropRange(cropYStr, "crop-y")
		if err != nil {
			return nil, err
		}
		config.CropY = cropY
	}

	// Parse time range
	timeRange, err := parseTimeRange(startTimeStr, endTimeStr, durationStr)
	if err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}
	config.TimeRange = timeRange

	return config, nil
}

func checkFFmpeg() error {
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg is not installed: %v", err)
	}
	return nil
}

func checkInputDir(inputDir string) error {
	info, err := os.Stat(inputDir)
	if err != nil {
		return fmt.Errorf("failed to access input directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("input path is not a directory: %s", inputDir)
	}
	return nil
}

func checkOutputFile(outputFile string, overwrite bool) error {
	if _, err := os.Stat(outputFile); err == nil && !overwrite {
		return fmt.Errorf("output file '%s' already exists. Use -y to overwrite", outputFile)
	}
	return nil
}

func generateTimelapse(config *Config) error {
	// Start constructing ffmpeg command
	args := []string{}
	if config.Overwrite {
		args = append(args, "-y")
	}

	// Add input options
	args = append(args,
		"-f", "concat",
		"-protocol_whitelist", "file,pipe",
		"-safe", "0",
		"-i", "pipe:0", // Read from stdin
	)

	// Add encoding options
	args = append(args, "-c:v", "libx264")

	// Build crop filter if either crop-x or crop-y is specified
	if config.CropX != nil || config.CropY != nil {
		var cropFilter string
		if config.CropX != nil && config.CropY != nil {
			// Both X and Y cropping
			cropFilter = fmt.Sprintf("crop=iw*%f:ih*%f:iw*%f:ih*%f",
				config.CropX.End-config.CropX.Start,
				config.CropY.End-config.CropY.Start,
				config.CropX.Start,
				config.CropY.Start)
		} else if config.CropX != nil {
			// Only X cropping
			cropFilter = fmt.Sprintf("crop=iw*%f:ih:iw*%f:0",
				config.CropX.End-config.CropX.Start,
				config.CropX.Start)
		} else {
			// Only Y cropping
			cropFilter = fmt.Sprintf("crop=iw:ih*%f:0:ih*%f",
				config.CropY.End-config.CropY.Start,
				config.CropY.Start)
		}
		args = append(args, "-vf", cropFilter)
	}

	// Add remaining encoding options
	args = append(args,
		"-preset", "slow",
		"-crf", "18",
		"-tune", "stillimage",
		"-pix_fmt", "yuv420p",
		config.OutputFile,
	)

	// Create a pipe for passing frame information to ffmpeg
	reader, writer := io.Pipe()
	defer reader.Close()

	// Start ffmpeg
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdin = reader
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	// Get frames through the channel
	frameChan, errChan := frames.GenerateFrames(config.InputDir, config.Speedup, config.TimeRange)

	// Write frames to the pipe in a goroutine
	go func() {
		defer writer.Close()
		for frame := range frameChan {
			if _, err := fmt.Fprintln(writer, frame.String()); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to pipe: %v\n", err)
				return
			}
		}
		// Check for any errors from the frame generation
		if err := <-errChan; err != nil {
			fmt.Fprintf(os.Stderr, "Error generating frames: %v\n", err)
			return
		}
	}()

	// Wait for ffmpeg to complete
	if err := cmd.Wait(); err != nil {
		// Get the last few lines of stderr for more context
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "FFmpeg stderr output:\n%s\n", exitErr.Stderr)
		}
		return fmt.Errorf("failed to generate timelapse: %v", err)
	}

	fmt.Printf("Timelapse generated: %s\n", config.OutputFile)
	return nil
}

func main() {
	config, err := parseArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing arguments: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Validate environment and inputs
	if err := checkFFmpeg(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := checkInputDir(config.InputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := checkOutputFile(config.OutputFile, config.Overwrite); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Generate the timelapse
	if err := generateTimelapse(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}