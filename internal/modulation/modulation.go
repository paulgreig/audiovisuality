package modulation

import "math"

// EngineFromProject builds an engine from a project's params. Params without
// explicit mapping use a default sine waveform. Uses project.Project via interface
// to avoid import cycle; call FromProjectParams from server/tui with project params.
func FromProjectParams(params []ParamRef) *Engine {
	e := NewEngine()
	for _, p := range params {
		m := MappingConfig{
			Mode: ModeWaveform,
			Waveform: &Waveform{
				Kind:      KindSin,
				Frequency: 1,
				Amplitude: 0.5,
				Offset:    0.5,
			},
		}
		e.AddParam(p.ID, p.Min, p.Max, p.Default, m)
	}
	return e
}

// ParamWithMapping is a param plus its mapping config (for use with FromProjectParamsWithMappings).
type ParamWithMapping struct {
	ID      string
	Min     float64
	Max     float64
	Default float64
	Mapping MappingConfig
}

// FromProjectParamsWithMappings builds an engine from params and their mapping configs.
// Use this when the project has per-param mapping (e.g. audio band, blend).
func FromProjectParamsWithMappings(params []ParamWithMapping) *Engine {
	e := NewEngine()
	for _, p := range params {
		e.AddParam(p.ID, p.Min, p.Max, p.Default, p.Mapping)
	}
	return e
}

// ParamRef is a minimal param definition for building an engine (avoids project import).
type ParamRef struct {
	ID      string
	Min     float64
	Max     float64
	Default float64
}

// WaveformKind is the type of procedural waveform.
type WaveformKind string

const (
	KindSin WaveformKind = "sin"
	KindCos WaveformKind = "cos"
	KindTan WaveformKind = "tan"
	KindExp WaveformKind = "exp"
)

// Waveform defines a looping waveform (sin, cos, tan, exp) with frequency, phase, amplitude, and offset.
// ValueAt returns a value in [0, 1] when Amplitude and Offset are in the usual range (e.g. Amplitude 0.5, Offset 0.5).
type Waveform struct {
	Kind      WaveformKind
	Frequency float64 // Hz
	Phase     float64 // radians
	Amplitude float64
	Offset    float64
}

// ValueAt returns the waveform value at time t (seconds). Result is typically in [0, 1].
func (w Waveform) ValueAt(t float64) float64 {
	angle := 2*math.Pi*w.Frequency*t + w.Phase
	var v float64
	switch w.Kind {
	case KindSin:
		v = math.Sin(angle)
	case KindCos:
		v = math.Cos(angle)
	case KindTan:
		v = math.Tan(angle)
		// Clamp tan to avoid infinity
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
	case KindExp:
		v = math.Exp(-math.Abs(math.Sin(angle)))
		// Normalize exp to roughly [0,1]
		v = (v - 1/math.E) / (1 - 1/math.E)
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		return v
	default:
		v = math.Sin(angle)
	}
	// Map [-1,1] to [0,1] via amplitude and offset
	out := w.Offset + w.Amplitude*v
	if out < 0 {
		out = 0
	}
	if out > 1 {
		out = 1
	}
	return out
}

// MappingMode is how a parameter value is derived.
type MappingMode string

const (
	ModeStatic   MappingMode = "static"
	ModeWaveform MappingMode = "waveform"
	ModeAudio    MappingMode = "audio"
	ModeBlend    MappingMode = "blend"
)

// BlendWeights defines how to combine static, waveform, and audio (each 0..1) into one value.
// Weights are normalized if needed; result is in [0, 1].
type BlendWeights struct {
	Static   float64
	Waveform float64
	Audio    float64
}

// MappingConfig configures how a single parameter is driven.
type MappingConfig struct {
	Mode        MappingMode
	StaticValue float64
	Waveform    *Waveform
	AudioBand   int
	Blend       *BlendWeights
}

// ParamSpec is a single parameter definition for the engine.
type ParamSpec struct {
	ID      string
	Min     float64
	Max     float64
	Default float64
	Mapping MappingConfig
}

// Engine computes parameter values from time and optional audio bands.
type Engine struct {
	params []ParamSpec
}

// NewEngine returns a new modulation engine.
func NewEngine() *Engine {
	return &Engine{}
}

// AddParam registers a parameter with the engine.
func (e *Engine) AddParam(id string, min, max, defaultVal float64, m MappingConfig) {
	e.params = append(e.params, ParamSpec{
		ID:      id,
		Min:     min,
		Max:     max,
		Default: defaultVal,
		Mapping: m,
	})
}

// Compute returns the current value for each parameter at time t (seconds).
// audio can be nil; if provided, audio[MappingConfig.AudioBand] is used for ModeAudio/ModeBlend (0..1).
func (e *Engine) Compute(t float64, audio []float64) map[string]float64 {
	out := make(map[string]float64, len(e.params))
	for _, p := range e.params {
		out[p.ID] = e.computeOne(t, audio, p)
	}
	return out
}

func (e *Engine) computeOne(t float64, audio []float64, p ParamSpec) float64 {
	var staticV, waveV, audioV float64

	staticV = p.Mapping.StaticValue
	if staticV < 0 {
		staticV = 0
	} else if staticV > 1 {
		staticV = 1
	}

	if p.Mapping.Waveform != nil {
		waveV = p.Mapping.Waveform.ValueAt(t)
	} else {
		waveV = 0.5
	}

	if audio != nil && p.Mapping.AudioBand >= 0 && p.Mapping.AudioBand < len(audio) {
		audioV = audio[p.Mapping.AudioBand]
		if audioV < 0 {
			audioV = 0
		} else if audioV > 1 {
			audioV = 1
		}
	}

	var norm float64
	switch p.Mapping.Mode {
	case ModeStatic:
		norm = staticV
	case ModeWaveform:
		norm = waveV
	case ModeAudio:
		norm = audioV
	case ModeBlend:
		b := p.Mapping.Blend
		if b == nil {
			norm = p.Default
		} else {
			sum := b.Static + b.Waveform + b.Audio
			if sum <= 0 {
				norm = p.Default
			} else {
				norm = (b.Static*staticV + b.Waveform*waveV + b.Audio*audioV) / sum
			}
		}
	default:
		norm = waveV
	}

	if norm < 0 {
		norm = 0
	} else if norm > 1 {
		norm = 1
	}

	val := p.Min + norm*(p.Max-p.Min)
	if val < p.Min {
		val = p.Min
	} else if val > p.Max {
		val = p.Max
	}
	return val
}
