package sdm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sigh/nest-timelapse/internal/auth"
	"google.golang.org/api/option"
	"google.golang.org/api/smartdevicemanagement/v1"
)

// Service wraps the SDM API service and provides high-level operations
type Service struct {
	service *smartdevicemanagement.Service
}

// NewService creates a new SDM service using the provided token source
func NewService(tokenSource *auth.TokenSource) (*Service, error) {
	service, err := smartdevicemanagement.NewService(context.Background(), option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create SDM service: %w", err)
	}

	return &Service{service: service}, nil
}

// FindCamera searches for a camera device in the enterprise and returns
// the first one found
func (s *Service) FindCamera(enterpriseID string) (*smartdevicemanagement.GoogleHomeEnterpriseSdmV1Device, error) {
	if enterpriseID == "" {
		return nil, fmt.Errorf("enterprise ID is required")
	}

	listDeviceResponse, err := s.service.Enterprises.Devices.List("enterprises/" + enterpriseID).Do()
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

// GenerateWebRTCStream sends the WebRTC offer to the camera and returns
// the answer SDP for establishing the connection
func (s *Service) GenerateWebRTCStream(camera *smartdevicemanagement.GoogleHomeEnterpriseSdmV1Device, offerSDP string) (string, error) {
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

	cmdResponse, err := s.service.Enterprises.Devices.ExecuteCommand(camera.Name, command).Do()
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
