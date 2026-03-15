package audio

import (
	"sync"
)

// DeviceInfo describes an audio input device.
type DeviceInfo struct {
	Index             int
	Name              string
	MaxInputChannels  int
	DefaultSampleRate float64
}

// CaptureConfig configures audio capture.
type CaptureConfig struct {
	DeviceIndex int     // index from ListDevices
	SampleRate  float64 // e.g. 44100
	Channels    int     // 1 = mono
	BufferSize  int     // frames per buffer
}

// Capture captures from an input device and feeds an analyzer. Call Stop() to release the device.
type Capture struct {
	doStop   func()
	stopOnce sync.Once
	stopErr  error
	stopMu   sync.Mutex
}

// Stop stops capture and releases the device.
func (c *Capture) Stop() error {
	var err error
	c.stopOnce.Do(func() {
		if c.doStop != nil {
			c.doStop()
			c.doStop = nil
		}
		c.stopMu.Lock()
		err = c.stopErr
		c.stopMu.Unlock()
	})
	return err
}

// ListDevices returns available input devices. Without -tags=portaudio returns nil, nil.
func ListDevices() ([]DeviceInfo, error) {
	return listDevicesPortaudio()
}

// StartCapture starts capturing from the given device and feeding the analyzer.
// Without -tags=portaudio returns an error.
func StartCapture(cfg CaptureConfig, analyzer *Analyzer) (*Capture, error) {
	return startCapturePortaudio(cfg, analyzer)
}
