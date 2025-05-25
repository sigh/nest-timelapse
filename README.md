# Nest Timelapse

A Go application that captures images from a Nest camera.

## Prerequisites

- Go 1.21 or later
- A Google Nest Enterprise account
- Enterprise ID from your Nest account
- Sufficient disk space for video storage
- Google Cloud credentials (credentials.json) for Nest API access

## Installation

1. Clone the repository:

```bash
git clone https://github.com/yourusername/Nest-Timelapse.git
cd Nest-Timelapse
```

2. Install dependencies:

```bash
go mod download
```

3. Set up credentials:
   - Place your `credentials.json` file in a secure directory (e.g., `~/.nest-timelapse/`)
   - The application will create a `token.json` file in the same directory during the first run

## Usage

Run the application using the following command:

```bash
go run main.go -enterprise-id "$ENTERPRISE_ID" -output-dir "$OUTPUT_DIR" -creds-dir "$CREDS_DIR"
```

Where:

- `$ENTERPRISE_ID`: Your Nest Enterprise ID (required)
- `$OUTPUT_DIR`: Directory where the timelapse videos will be saved (default: current directory)
- `$CREDS_DIR`: Directory containing credentials.json and token.json files (default: current directory)

Then run the following command to generate a timelapse video:

```bash
./make_timelapse.sh _output/*.jpg timelapse.mp4
```

## Credential files

1. `credentials.json`: Google Cloud credentials file (create this in the Google Cloud Console)
2. `token.json`: Generated automatically during the first run, stores the OAuth token

These files should be kept secure and not committed to version control. The application will automatically handle token refresh when needed.
