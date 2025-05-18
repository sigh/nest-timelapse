# Nest Timelapse

A Go application that captures images from a Nest camera.

## Prerequisites

- Go 1.21 or later
- A Google Nest Enterprise account
- Enterprise ID from your Nest account
- Sufficient disk space for video storage

## Usage

Run the application using the following command:

```bash
go run main.go -enterprise-id "$ENTERPRISE_ID" -output-dir "$OUTPUT_DIR"
```

Where:

- `$ENTERPRISE_ID`: Your Nest Enterprise ID
- `$OUTPUT_DIR`: Directory where individual images will be saved

Then run the following command to generate a timelapse video:

```bash
./make_timelapse.sh _output/*.jpg timelapse.mp4
```
