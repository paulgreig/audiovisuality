//go:build portaudio

package audio

import (
	"fmt"
	"log"
	"sync"

	"github.com/gordonklaus/portaudio"
)

func listDevicesPortaudio() ([]DeviceInfo, error) {
	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("portaudio init: %w", err)
	}
	defer portaudio.Terminate()

	devs, err := portaudio.Devices()
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}

	var out []DeviceInfo
	for i, d := range devs {
		if d == nil || d.MaxInputChannels < 1 {
			continue
		}
		out = append(out, DeviceInfo{
			Index:             i,
			Name:              d.Name,
			MaxInputChannels:  d.MaxInputChannels,
			DefaultSampleRate: d.DefaultSampleRate,
		})
	}
	return out, nil
}

func startCapturePortaudio(cfg CaptureConfig, analyzer *Analyzer) (*Capture, error) {
	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("portaudio init: %w", err)
	}

	devs, err := portaudio.Devices()
	if err != nil {
		portaudio.Terminate()
		return nil, fmt.Errorf("devices: %w", err)
	}
	if cfg.DeviceIndex < 0 || cfg.DeviceIndex >= len(devs) || devs[cfg.DeviceIndex] == nil {
		portaudio.Terminate()
		return nil, fmt.Errorf("invalid device index %d", cfg.DeviceIndex)
	}

	in := devs[cfg.DeviceIndex]
	if in.MaxInputChannels < cfg.Channels {
		portaudio.Terminate()
		return nil, fmt.Errorf("device has %d input channels, need %d", in.MaxInputChannels, cfg.Channels)
	}

	// If no sample rate was specified, use the device's default.
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = in.DefaultSampleRate
	}

	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1024
	}
	if cfg.Channels <= 0 {
		cfg.Channels = 1
	}

	buf := make([]float32, cfg.BufferSize*cfg.Channels)
	stream, err := portaudio.OpenStream(
		portaudio.StreamParameters{
			Input: portaudio.StreamDeviceParameters{
				Device:   in,
				Channels: cfg.Channels,
				Latency:  in.DefaultLowInputLatency,
			},
			SampleRate:      cfg.SampleRate,
			FramesPerBuffer: cfg.BufferSize,
		},
		buf,
		nil,
	)
	if err != nil {
		portaudio.Terminate()
		return nil, fmt.Errorf("open stream: %w", err)
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		portaudio.Terminate()
		return nil, fmt.Errorf("stream start: %w", err)
	}

	var stopOnce sync.Once
	c := &Capture{
		doStop: func() {
			stopOnce.Do(func() {
				stream.Stop()
				stream.Close()
				portaudio.Terminate()
			})
		},
	}

	fftSize := analyzer.FFTSize()
	samplesFloat64 := make([]float64, 0, fftSize*2)
	go func() {
		for {
			err := stream.Read()
			if err != nil {
				log.Printf("portaudio stream read error on device %q: %v", in.Name, err)
				c.stopMu.Lock()
				c.stopErr = err
				c.stopMu.Unlock()
				c.doStop()
				return
			}
			for i := 0; i < cfg.BufferSize; i++ {
				samplesFloat64 = append(samplesFloat64, float64(buf[i*cfg.Channels]))
			}
			for len(samplesFloat64) >= fftSize {
				analyzer.ProcessPCM(samplesFloat64[:fftSize])
				samplesFloat64 = samplesFloat64[fftSize:]
			}
		}
	}()

	return c, nil
}
