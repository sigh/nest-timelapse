package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type CropRange struct {
	Start float64
	End   float64
}

type Config struct {
	Framerate    int
	OutputFile   string
	Overwrite    bool
	InputPattern string
	CropX        *CropRange
	CropY        *CropRange
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

func parseArgs() (*Config, error) {
	config := &Config{
		Framerate:    5,
		OutputFile:   "timelapse.mp4",
		InputPattern: "nest_camera_frame_*.jpg",
	}

	var cropXStr, cropYStr string
	flag.IntVar(&config.Framerate, "framerate", config.Framerate, "Set the output framerate")
	flag.IntVar(&config.Framerate, "f", config.Framerate, "Set the output framerate (shorthand)")
	flag.StringVar(&config.OutputFile, "output", config.OutputFile, "Set the output file")
	flag.StringVar(&config.OutputFile, "o", config.OutputFile, "Set the output file (shorthand)")
	flag.BoolVar(&config.Overwrite, "y", false, "Overwrite output file if it exists")
	flag.BoolVar(&config.Overwrite, "overwrite", false, "Overwrite output file if it exists (long form)")
	flag.StringVar(&cropXStr, "crop-x", "", "Crop horizontally using width ratios (e.g. '0.4-0.6')")
	flag.StringVar(&cropYStr, "crop-y", "", "Crop vertically using height ratios (e.g. '0.4-0.6')")

	// Add minimal usage message for the positional argument
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [input_pattern]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "If input_pattern is not provided, defaults to \"nest_camera_frame_*.jpg\"\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Handle input pattern as positional argument
	if flag.NArg() > 0 {
		config.InputPattern = flag.Arg(0)
	}

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

	return config, nil
}

func checkFFmpeg() error {
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg is not installed: %v", err)
	}
	return nil
}

func checkInputFiles(pattern string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %v", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no files matching pattern '%s' found", pattern)
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
	args := []string{}
	if config.Overwrite {
		args = append(args, "-y")
	}

	// Add input options
	args = append(args,
		"-framerate", fmt.Sprintf("%d", config.Framerate),
		"-pattern_type", "glob",
		"-i", config.InputPattern,
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

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Generating timelapse from %s -> %s\n", config.InputPattern, config.OutputFile)
	if config.CropX != nil {
		fmt.Printf("Cropping horizontally from %.2f to %.2f of width\n", config.CropX.Start, config.CropX.End)
	}
	if config.CropY != nil {
		fmt.Printf("Cropping vertically from %.2f to %.2f of height\n", config.CropY.Start, config.CropY.End)
	}
	if err := cmd.Run(); err != nil {
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

	if err := checkInputFiles(config.InputPattern); err != nil {
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