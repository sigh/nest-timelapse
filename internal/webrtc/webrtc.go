package webrtc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pion/interceptor"
	pionwebrtc "github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/h264writer"
)

// SessionDescription is an alias for pionwebrtc.SessionDescription
type SessionDescription = pionwebrtc.SessionDescription

// SDPTypeAnswer is an alias for pionwebrtc.SDPTypeAnswer
const SDPTypeAnswer = pionwebrtc.SDPTypeAnswer

// TrackRemote is an alias for pionwebrtc.TrackRemote
type TrackRemote = pionwebrtc.TrackRemote

// RTPReceiver is an alias for pionwebrtc.RTPReceiver
type RTPReceiver = pionwebrtc.RTPReceiver

// PeerConnection is an alias for pionwebrtc.PeerConnection
type PeerConnection = pionwebrtc.PeerConnection

// SetupWebRTC initializes the WebRTC peer connection with default codecs
// and a Google STUN server
func SetupWebRTC() (*PeerConnection, error) {
	m := &pionwebrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, fmt.Errorf("failed to register default codecs: %w", err)
	}

	i := &interceptor.Registry{}
	if err := pionwebrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, fmt.Errorf("failed to register default interceptors: %w", err)
	}

	api := pionwebrtc.NewAPI(pionwebrtc.WithMediaEngine(m), pionwebrtc.WithInterceptorRegistry(i))

	pcConfig := pionwebrtc.Configuration{
		ICEServers: []pionwebrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	peerConnection, err := api.NewPeerConnection(pcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	return peerConnection, nil
}

// SetupTransceivers configures the peer connection to receive audio and video,
// and sets up a data channel for camera control
func SetupTransceivers(pc *PeerConnection) error {
	if _, err := pc.AddTransceiverFromKind(pionwebrtc.RTPCodecTypeAudio,
		pionwebrtc.RTPTransceiverInit{Direction: pionwebrtc.RTPTransceiverDirectionRecvonly},
	); err != nil {
		return fmt.Errorf("failed to add audio transceiver: %w", err)
	}

	if _, err := pc.AddTransceiverFromKind(pionwebrtc.RTPCodecTypeVideo,
		pionwebrtc.RTPTransceiverInit{Direction: pionwebrtc.RTPTransceiverDirectionRecvonly},
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

// CreateOffer generates a WebRTC offer and waits for ICE candidate gathering
func CreateOffer(pc *PeerConnection) (*SessionDescription, error) {
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create offer: %w", err)
	}

	gatherComplete := pionwebrtc.GatheringCompletePromise(pc)
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
func writeH264ToBuffer(remoteTrack *TrackRemote) (*bytes.Buffer, error) {
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
		if err == io.EOF {
			return buffer, nil
		}
		if err != nil {
			return buffer, fmt.Errorf("track ended: %w", err)
		}
		if err := writer.WriteRTP(rtpPacket); err != nil {
			return buffer, fmt.Errorf("failed to write RTP packet: %w", err)
		}
	}
}

// HandleTrack processes incoming media tracks, writing H264 data to a buffer
// and ignoring other track types. Returns the buffered data if video was recorded.
func HandleTrack(remoteTrack *TrackRemote, receiver *RTPReceiver) *bytes.Buffer {
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
	if codecName != pionwebrtc.MimeTypeH264 {
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

// WaitForConnection monitors the peer connection state until it's connected
// or fails, with a timeout
func WaitForConnection(pc *PeerConnection, timeout time.Duration) error {
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

// WaitForConnectionClose gracefully closes the peer connection and waits
// for it to fully close, with a timeout
func WaitForConnectionClose(pc *PeerConnection, timeout time.Duration) error {
	done := make(chan struct{})
	pc.OnConnectionStateChange(func(state pionwebrtc.PeerConnectionState) {
		if state == pionwebrtc.PeerConnectionStateClosed {
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
func monitorConnectionState(pc *PeerConnection, done, failed chan struct{}, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			switch pc.ConnectionState() {
			case pionwebrtc.PeerConnectionStateConnected:
				close(done)
				return
			case pionwebrtc.PeerConnectionStateFailed, pionwebrtc.PeerConnectionStateClosed:
				close(failed)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}
