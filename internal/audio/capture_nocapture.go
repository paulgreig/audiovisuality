//go:build !portaudio

package audio

import (
	"fmt"
)

func listDevicesPortaudio() ([]DeviceInfo, error) {
	return nil, fmt.Errorf("audio device list requires build with -tags=portaudio (e.g. go run -tags=portaudio ./cmd/audiovisual or go build -tags=portaudio)")
}

func startCapturePortaudio(cfg CaptureConfig, analyzer *Analyzer) (*Capture, error) {
	return nil, fmt.Errorf("audio capture requires build with -tags=portaudio and PortAudio C library installed (e.g. brew install portaudio)")
}
