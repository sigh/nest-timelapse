#!/bin/bash

# Usage: ./make_timelapse.sh [input_pattern] [output_file]
# Example: ./make_timelapse.sh "_output/*.jpg" timelapse.mp4

# Output video framerate (higher = smoother but larger file)
FRAMERATE=30
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

# Default input/output
INPUT_PATTERN=${1:-"nest_camera_frame_*.jpg"}
OUTPUT_FILE=${2:-"timelapse.mp4"}

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

echo "Generating timelapse from $INPUT_PATTERN -> $OUTPUT_FILE"

# Generate the timelapse
ffmpeg \
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
else
    echo "Error: Failed to generate timelapse"
    exit 1
fi