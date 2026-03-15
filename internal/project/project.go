package project

import (
	"encoding/json"
	"errors"
	"os"
)

type ParamType string

const (
	ParamTypeFloat ParamType = "float"
	ParamTypeInt   ParamType = "int"
	ParamTypeBool  ParamType = "bool"
)

type Project struct {
	Name              string  `json:"name"`
	Description       string  `json:"description,omitempty"`
	Shader            Shader  `json:"shader"`
	Params            []Param `json:"params,omitempty"`
	OverlayImagePath  string  `json:"overlay_image_path,omitempty"`
}

type Shader struct {
	FragmentSource string `json:"fragment_source"`
}

type Param struct {
	ID      string    `json:"id"`
	Label   string    `json:"label"`
	Min     float64   `json:"min"`
	Max     float64   `json:"max"`
	Default float64   `json:"default"`
	Step    float64   `json:"step"`
	Type    ParamType `json:"type"`
	Mapping *ParamMapping `json:"mapping,omitempty"`
}

// ParamMapping configures how a parameter is driven (waveform, audio, blend).
type ParamMapping struct {
	Mode        string          `json:"mode,omitempty"` // "static", "waveform", "audio", "blend"
	StaticValue float64         `json:"static_value,omitempty"`
	Waveform    *WaveformConfig `json:"waveform,omitempty"`
	AudioBand   int             `json:"audio_band,omitempty"`
	Blend       *BlendWeights   `json:"blend,omitempty"`
}

// WaveformConfig defines a looping waveform for modulation.
type WaveformConfig struct {
	Kind      string  `json:"kind"`
	Frequency float64 `json:"frequency"`
	Phase     float64 `json:"phase"`
	Amplitude float64 `json:"amplitude"`
	Offset    float64 `json:"offset"`
}

// BlendWeights defines how to combine static, waveform, and audio (0..1) in blend mode.
type BlendWeights struct {
	Static   float64 `json:"static"`
	Waveform float64 `json:"waveform"`
	Audio    float64 `json:"audio"`
}

func (p *Project) Validate() error {
	for _, param := range p.Params {
		if param.Min > param.Max {
			return errors.New("param min greater than max")
		}
	}
	return nil
}

func Save(path string, p *Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func Load(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
