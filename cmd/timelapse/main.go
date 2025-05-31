package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Config struct {
	Framerate   int
	OutputFile  string
	Overwrite   bool
	InputPattern string
}

func parseArgs() (*Config, error) {
	config := &Config{
		Framerate:   5,
		OutputFile:  "timelapse.mp4",
		InputPattern: "nest_camera_frame_*.jpg",
	}

	flag.IntVar(&config.Framerate, "framerate", config.Framerate, "Set the output framerate")
	flag.IntVar(&config.Framerate, "f", config.Framerate, "Set the output framerate (shorthand)")
	flag.StringVar(&config.OutputFile, "output", config.OutputFile, "Set the output file")
	flag.StringVar(&config.OutputFile, "o", config.OutputFile, "Set the output file (shorthand)")
	flag.BoolVar(&config.Overwrite, "y", false, "Overwrite output file if it exists")
	flag.BoolVar(&config.Overwrite, "overwrite", false, "Overwrite output file if it exists (long form)")

	// Custom help message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [input_pattern]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s -f 10 -o timelapse.mp4 -y \"_output/*.jpg\"\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nIf input_pattern is not provided, defaults to \"nest_camera_frame_*.jpg\"\n")
	}

	flag.Parse()

	// Handle input pattern as positional argument
	if flag.NArg() > 0 {
		config.InputPattern = flag.Arg(0)
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
	args = append(args,
		"-c:v", "libx264",
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