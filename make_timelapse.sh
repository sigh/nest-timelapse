#!/bin/bash

# Usage: ./make_timelapse.sh [-f|--framerate FPS] [-o|--output FILE] [-y] input_pattern
# Example: ./make_timelapse.sh -f 10 -o timelapse.mp4 -y "_output/*.jpg"

# Default values
FRAMERATE=5
OUTPUT_FILE="timelapse.mp4"
INPUT_PATTERN="nest_camera_frame_*.jpg"
OVERWRITE=""

# Parse named arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--framerate)
            FRAMERATE="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        -y)
            OVERWRITE="-y"
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [-f|--framerate FPS] [-o|--output FILE] [-y] input_pattern"
            echo "Example: $0 -f 10 -o timelapse.mp4 -y \"_output/*.jpg\""
            echo "Options:"
            echo "  -f, --framerate FPS  Set the output framerate (default: 5)"
            echo "  -o, --output FILE    Set the output file (default: timelapse.mp4)"
            echo "  -y                   Overwrite output file if it exists"
            echo "  -h, --help           Show this help message"
            exit 0
            ;;
        -*)
            echo "Error: Unknown option '$1'"
            echo "Usage: $0 [-f|--framerate FPS] [-o|--output FILE] [-y] input_pattern"
            exit 1
            ;;
        *)
            # First positional argument is the input pattern
            if [ -z "$INPUT_PATTERN" ] || [ "$INPUT_PATTERN" = "nest_camera_frame_*.jpg" ]; then
                INPUT_PATTERN="$1"
                shift
            else
                echo "Error: Only one input pattern allowed"
                echo "Usage: $0 [-f|--framerate FPS] [-o|--output FILE] [-y] input_pattern"
                exit 1
            fi
            ;;
    esac
done

# Check if ffmpeg is installed
if ! command -v ffmpeg &> /dev/null; then
    echo "Error: ffmpeg is not installed"
    exit 1
fi

# Check if input files exist
if ! ls $INPUT_PATTERN &> /dev/null; then
    echo "Error: No files matching pattern '$INPUT_PATTERN' found"
    exit 1
fi

# Check if output file exists and -y not specified
if [ -f "$OUTPUT_FILE" ] && [ -z "$OVERWRITE" ]; then
    echo "Error: Output file '$OUTPUT_FILE' already exists. Use -y to overwrite."
    exit 1
fi

echo "Generating timelapse from $INPUT_PATTERN -> $OUTPUT_FILE"

# Video codec (libx264 is widely supported and efficient)
CODEC="libx264"
# Encoding preset (slower = better compression, options: ultrafast,superfast,veryfast,
# faster,fast,medium,slow,slower,veryslow)
PRESET="slow"
# Constant Rate Factor (lower = better quality, range 0-51, 18 is visually lossless)
CRF=18
# Encoding tune (stillimage optimizes for static content)
TUNE="stillimage"
# Pixel format (yuv420p is widely compatible with most players)
PIX_FMT="yuv420p"

# Generate the timelapse
ffmpeg $OVERWRITE \
    -framerate $FRAMERATE \
    -pattern_type glob -i "$INPUT_PATTERN" \
    -c:v $CODEC \
    -preset $PRESET \
    -crf $CRF \
    -tune $TUNE \
    -pix_fmt $PIX_FMT \
    "$OUTPUT_FILE"

if [ $? -eq 0 ]; then
    echo "Timelapse generated: $OUTPUT_FILE"
    exit 0
else
    echo "Error: Failed to generate timelapse (ffmpeg exit code: $FFMPEG_EXIT)"
    exit 1
fi
