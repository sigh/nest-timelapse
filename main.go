// Package main implements a Nest camera video recorder that uses WebRTC to stream
// and record video from a Google Nest camera. It authenticates with the Smart Device
// Management API and handles the WebRTC connection lifecycle.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/h264writer"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/smartdevicemanagement/v1"
)

// Auth and API configuration
const (
	// oauthScope is required for accessing the Smart Device Management API
	oauthScope      = "https://www.googleapis.com/auth/sdm.service"
	tokenFile       = "token.json"
	credentialsFile = "credentials.json"
)

// Timeouts and durations
const (
	// webRtcTimeout is the maximum time to wait for WebRTC operations
	webRtcTimeout = 30 * time.Second
	// recordingDuration is how long to record video from the camera
	recordingDuration = 5 * time.Second
)

// File naming
const (
	imageFilePrefix    = "nest_camera_frame_"
	imageFileExtension = "jpg"
	// timeFormat is used for generating unique filenames
	timeFormat = "20060102_150405"
)

var (
	outputDir    string
	enterpriseID string
)

// Types and interfaces
type credentials struct {
	Installed struct {
		ClientID                string   `json:"client_id"`
		ProjectID               string   `json:"project_id"`
		AuthURI                 string   `json:"auth_uri"`
		TokenURI                string   `json:"token_uri"`
		AuthProviderX509CertURL string   `json:"auth_provider_x509_cert_url"`
		ClientSecret            string   `json:"client_secret"`
		RedirectURIs            []string `json:"redirect_uris"`
	} `json:"installed"`
}

// loadJSON is a generic function that loads and parses a JSON file into the specified type
func loadJSON[T any](filename string) (*T, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from %s: %w", filename, err)
	}
	return &result, nil
}

// saveJSON is a generic function that saves a value as JSON to the specified file
func saveJSON[T any](data *T, filename string) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON for %s: %w", filename, err)
	}
	return os.WriteFile(filename, jsonData, 0600)
}

// getCredentials handles OAuth token management, including loading from cache
// and initiating the OAuth flow if needed
func getCredentials() (*oauth2.Token, error) {
	if token, err := loadJSON[oauth2.Token](tokenFile); err == nil {
		return token, nil
	}

	creds, err := loadJSON[credentials](credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	config := &oauth2.Config{
		ClientID:     creds.Installed.ClientID,
		ClientSecret: creds.Installed.ClientSecret,
		Scopes:       []string{oauthScope},
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8080",
	}

	token, err := handleOAuthFlow(config)
	if err != nil {
		return nil, fmt.Errorf("failed to complete OAuth flow: %w", err)
	}

	if err := saveJSON(token, tokenFile); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return token, nil
}

// handleOAuthFlow implements the OAuth 2.0 authorization code flow, prompting
// the user to authorize the application in their browser. It accepts either
// the authorization code directly or the full redirect URL.
func handleOAuthFlow(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("Go to the following link in your browser:\n%v\n", authURL)
	fmt.Print("Enter the authorization code or redirect URL: ")

	var input string
	if _, err := fmt.Scan(&input); err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	// Try to extract code from redirect URL if it looks like a URL
	var authCode string
	if strings.HasPrefix(input, "http") {
		redirectURL, err := url.Parse(input)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redirect URL: %w", err)
		}
		authCode = redirectURL.Query().Get("code")
		if authCode == "" {
			return nil, fmt.Errorf("no authorization code found in redirect URL")
		}
	} else {
		authCode = input
	}

	token, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	return token, nil
}

// createSDMService initializes the Smart Device Management API client
// with the provided OAuth token
func createSDMService(token *oauth2.Token) (*smartdevicemanagement.Service, error) {
	config := &oauth2.Config{
		Scopes:   []string{oauthScope},
		Endpoint: google.Endpoint,
	}
	tokenSource := config.TokenSource(context.Background(), token)

	service, err := smartdevicemanagement.NewService(context.Background(), option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create SDM service: %w", err)
	}

	return service, nil
}

// findCamera searches for a camera device in the enterprise and returns
// the first one found
func findCamera(service *smartdevicemanagement.Service) (*smartdevicemanagement.GoogleHomeEnterpriseSdmV1Device, error) {
	if enterpriseID == "" {
		return nil, fmt.Errorf("enterprise ID is required")
	}

	listDeviceResponse, err := service.Enterprises.Devices.List("enterprises/" + enterpriseID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	if len(listDeviceResponse.Devices) == 0 {
		return nil, fmt.Errorf("no devices found")
	}

	for _, device := range listDeviceResponse.Devices {
		if device.Type == "sdm.devices.types.CAMERA" {
			return device, nil
		}
	}

	return nil, fmt.Errorf("no camera found in device list")
}

// setupWebRTC initializes the WebRTC peer connection with default codecs
// and a Google STUN server
func setupWebRTC() (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, fmt.Errorf("failed to register default codecs: %w", err)
	}

	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, fmt.Errorf("failed to register default interceptors: %w", err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))

	pcConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	peerConnection, err := api.NewPeerConnection(pcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	return peerConnection, nil
}

// setupTransceivers configures the peer connection to receive audio and video,
// and sets up a data channel for camera control
func setupTransceivers(pc *webrtc.PeerConnection) error {
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly},
	); err != nil {
		return fmt.Errorf("failed to add audio transceiver: %w", err)
	}

	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly},
	); err != nil {
		return fmt.Errorf("failed to add video transceiver: %w", err)
	}

	triggerChannel, err := pc.CreateDataChannel("trigger", nil)
	if err != nil {
		return fmt.Errorf("failed to create data channel: %w", err)
	}

	triggerChannel.OnOpen(func() {
		fmt.Println("Data channel 'trigger' opened")
	})
	triggerChannel.OnClose(func() {
		fmt.Println("Data channel 'trigger' closed")
	})
	triggerChannel.OnError(func(e error) {
		fmt.Printf("Data channel 'trigger': %v\n", e)
	})

	return nil
}

// createOffer generates a WebRTC offer and waits for ICE candidate gathering
func createOffer(pc *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create offer: %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err = pc.SetLocalDescription(offer); err != nil {
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	select {
	case <-gatherComplete:
		fmt.Println("ICE candidate gathering complete")
	case <-time.After(20 * time.Second):
		return nil, fmt.Errorf("failed to gather ICE candidates: timeout")
	}

	return pc.LocalDescription(), nil
}

// writeH264ToBuffer writes H264 RTP packets to a buffer using an H264 writer.
// Returns the buffer with the written data and any error that occurred.
func writeH264ToBuffer(remoteTrack *webrtc.TrackRemote) (*bytes.Buffer, error) {
	buffer := &bytes.Buffer{}
	writer := h264writer.NewWith(buffer)

	// Ensure writer is closed when we're done
	defer func() {
		if err := writer.Close(); err != nil {
			fmt.Println("Failed to close H264 writer:", err)
		}
	}()

	for {
		rtpPacket, _, err := remoteTrack.ReadRTP()
		if err != nil {
			return buffer, fmt.Errorf("track ended: %w", err)
		}
		if err := writer.WriteRTP(rtpPacket); err != nil {
			return buffer, fmt.Errorf("failed to write RTP packet: %w", err)
		}
	}
}

// handleTrack processes incoming media tracks, writing H264 data to a buffer
// and ignoring other track types. Returns the buffered data if video was recorded.
func handleTrack(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) *bytes.Buffer {
	codecName := remoteTrack.Codec().MimeType
	trackType := remoteTrack.Kind().String()
	fmt.Printf("Received track: %s, codec: %s, id: %s, ssrc: %d\n",
		trackType, codecName, remoteTrack.ID(), remoteTrack.SSRC())

	// Skip non-video tracks
	if trackType != "video" {
		fmt.Printf("Skipping non-video track: %s\n", trackType)
		return nil
	}

	// Skip non-H264 tracks
	if codecName != webrtc.MimeTypeH264 {
		fmt.Printf("Skipping non-H264 track: %s\n", codecName)
		return nil
	}

	fmt.Println("Buffering video data...")
	buffer, err := writeH264ToBuffer(remoteTrack)
	if err != nil {
		fmt.Println("Error writing H264 data:", err)
		return buffer // Return buffer even on error as it may contain partial data
	}

	return buffer
}

// generateWebRTCStream sends the WebRTC offer to the camera and returns
// the answer SDP for establishing the connection
func generateWebRTCStream(service *smartdevicemanagement.Service, camera *smartdevicemanagement.GoogleHomeEnterpriseSdmV1Device, offerSDP string) (string, error) {
	cmdParams := map[string]interface{}{
		"offerSdp": offerSDP,
	}
	cmdParamsJSON, err := json.Marshal(cmdParams)
	if err != nil {
		return "", fmt.Errorf("failed to marshal command parameters: %w", err)
	}

	command := &smartdevicemanagement.GoogleHomeEnterpriseSdmV1ExecuteDeviceCommandRequest{
		Command: "sdm.devices.commands.CameraLiveStream.GenerateWebRtcStream",
		Params:  cmdParamsJSON,
	}

	cmdResponse, err := service.Enterprises.Devices.ExecuteCommand(camera.Name, command).Do()
	if err != nil {
		return "", fmt.Errorf("failed to execute GenerateWebRtcStream command: %w", err)
	}

	var response struct {
		AnswerSdp string `json:"answerSdp"`
	}
	if err := json.Unmarshal(cmdResponse.Results, &response); err != nil {
		return "", fmt.Errorf("failed to parse command response: %w", err)
	}

	if response.AnswerSdp == "" {
		return "", fmt.Errorf("failed to get answer SDP: empty response")
	}

	return response.AnswerSdp, nil
}

// waitForConnection monitors the peer connection state until it's connected
// or fails, with a timeout
func waitForConnection(pc *webrtc.PeerConnection, timeout time.Duration) error {
	done := make(chan struct{})
	failed := make(chan struct{})

	monitorCtx, cancelMonitor := context.WithCancel(context.Background())
	defer cancelMonitor()

	go monitorConnectionState(pc, done, failed, monitorCtx)

	select {
	case <-done:
		fmt.Println("WebRTC connection established")
		return nil
	case <-failed:
		return fmt.Errorf("WebRTC connection failed")
	case <-time.After(timeout):
		return fmt.Errorf("failed to establish WebRTC connection: timeout")
	}
}

// waitForConnectionClose gracefully closes the peer connection and waits
// for it to fully close, with a timeout
func waitForConnectionClose(pc *webrtc.PeerConnection, timeout time.Duration) error {
	done := make(chan struct{})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateClosed {
			close(done)
		}
	})

	if err := pc.Close(); err != nil {
		return fmt.Errorf("failed to close peer connection: %w", err)
	}

	select {
	case <-done:
		fmt.Println("WebRTC connection closed")
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("failed to close connection: timeout")
	}
}

// monitorConnectionState is a helper that watches the peer connection state
// and signals when it's connected or failed
func monitorConnectionState(pc *webrtc.PeerConnection, done, failed chan struct{}, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			switch pc.ConnectionState() {
			case webrtc.PeerConnectionStateConnected:
				close(done)
				return
			case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
				close(failed)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// extractFirstFrame uses ffmpeg to extract the first frame from H264 data in memory
func extractFirstFrame(h264Data *bytes.Buffer) error {
	timestamp := time.Now().Format(timeFormat)
	filename := fmt.Sprintf("%s%s.%s", imageFilePrefix, timestamp, imageFileExtension)
	imagePath := filepath.Join(outputDir, filename)

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

// getCameraImage is the main function that orchestrates the entire process:
// authentication, camera discovery, WebRTC setup, streaming, and recording
func getCameraImage() error {
	oauthToken, err := getCredentials()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	service, err := createSDMService(oauthToken)
	if err != nil {
		return err
	}

	camera, err := findCamera(service)
	if err != nil {
		return err
	}

	peerConnection, err := setupWebRTC()
	if err != nil {
		return err
	}
	defer peerConnection.Close()

	if err := setupTransceivers(peerConnection); err != nil {
		return err
	}

	offer, err := createOffer(peerConnection)
	if err != nil {
		return err
	}

	// Create a channel to receive the buffered video data
	videoData := make(chan *bytes.Buffer, 1)
	peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if buffer := handleTrack(remoteTrack, receiver); buffer != nil {
			videoData <- buffer
		}
	})

	answerSdp, err := generateWebRTCStream(service, camera, offer.SDP)
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

	if err := waitForConnection(peerConnection, webRtcTimeout); err != nil {
		return err
	}

	fmt.Printf("Recording for %s...\n", recordingDuration)
	time.Sleep(recordingDuration)

	if err := waitForConnectionClose(peerConnection, webRtcTimeout); err != nil {
		return fmt.Errorf("failed to clean up connection: %w", err)
	}

	fmt.Println("Recording complete")

	// Wait for the video data from the recording
	select {
	case buffer := <-videoData:
		if err := extractFirstFrame(buffer); err != nil {
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
