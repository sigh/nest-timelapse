// Package main implements a Nest camera video recorder that uses WebRTC to stream
// and record video from a Google Nest camera. It authenticates with the Smart Device
// Management API and handles the WebRTC connection lifecycle.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sigh/nest-timelapse/internal/auth"
	"github.com/sigh/nest-timelapse/internal/sdm"
	"github.com/sigh/nest-timelapse/internal/video"
	"github.com/sigh/nest-timelapse/internal/webrtc"
)

// Auth files
const (
	tokenFile       = "token.json"
	credentialsFile = "credentials.json"
)

// Timeouts and durations
const (
	// recordingDuration is how long to record video from the camera
	recordingDuration = 5 * time.Second
	// webRtcTimeout is the maximum time to wait for WebRTC operations
	webRtcTimeout = 30 * time.Second
)

var (
	outputDir    string
	enterpriseID string
)

// getCameraImage is the main function that orchestrates the entire process:
// authentication, camera discovery, WebRTC setup, streaming, and recording
func getCameraImage() error {
	tokenSource, err := auth.GetCredentials(tokenFile, credentialsFile)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	sdmService, err := sdm.NewService(tokenSource)
	if err != nil {
		return err
	}

	cameraDevice, err := sdmService.FindCamera(enterpriseID)
	if err != nil {
		return err
	}

	peerConnection, err := webrtc.SetupWebRTC()
	if err != nil {
		return err
	}
	defer peerConnection.Close()

	if err := webrtc.SetupTransceivers(peerConnection); err != nil {
		return err
	}

	offer, err := webrtc.CreateOffer(peerConnection)
	if err != nil {
		return err
	}

	// Create a channel to receive the buffered video data
	videoData := make(chan *bytes.Buffer, 1)
	peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if buffer := webrtc.HandleTrack(remoteTrack, receiver); buffer != nil {
			videoData <- buffer
		}
	})

	answerSdp, err := sdmService.GenerateWebRTCStream(cameraDevice, offer.SDP)
	if err != nil {
		return err
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answerSdp,
	}
	if err := peerConnection.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	if err := webrtc.WaitForConnection(peerConnection, webRtcTimeout); err != nil {
		return err
	}

	fmt.Printf("Recording for %s...\n", recordingDuration)
	time.Sleep(recordingDuration)

	if err := webrtc.WaitForConnectionClose(peerConnection, webRtcTimeout); err != nil {
		return fmt.Errorf("failed to clean up connection: %w", err)
	}

	fmt.Println("Recording complete")

	// Wait for the video data from the recording
	select {
	case buffer := <-videoData:
		if err := video.ExtractFirstFrame(buffer, outputDir); err != nil {
			return fmt.Errorf("failed to extract frame: %w", err)
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for video data")
	}

	return nil
}

func main() {
	// Parse command line flags
	flag.StringVar(&outputDir, "output-dir", ".", "Directory to save captured frames")
	flag.StringVar(&enterpriseID, "enterprise-id", "", "Google Workspace enterprise ID where the camera is registered")
	flag.Parse()

	if enterpriseID == "" {
		log.Fatal("enterprise-id flag is required")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Convert to absolute path for consistent handling
	absPath, err := filepath.Abs(outputDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path for output directory: %v", err)
	}
	outputDir = absPath

	fmt.Printf("Using enterprise ID: %s\n", enterpriseID)
	fmt.Printf("Saving frames to: %s\n", outputDir)

	if err := getCameraImage(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
