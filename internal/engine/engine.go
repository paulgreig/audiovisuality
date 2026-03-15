package engine

import (
	"audiovisual/internal/modulation"
	"audiovisual/internal/project"
)

// BuildEngine converts a project's params and mappings into a modulation engine.
// Params without a mapping use a default sine waveform.
func BuildEngine(p *project.Project) *modulation.Engine {
	params := make([]modulation.ParamWithMapping, 0, len(p.Params))
	for _, param := range p.Params {
		m := projectMappingToModulation(param.Mapping)
		params = append(params, modulation.ParamWithMapping{
			ID:      param.ID,
			Min:     param.Min,
			Max:     param.Max,
			Default: param.Default,
			Mapping: m,
		})
	}
	return modulation.FromProjectParamsWithMappings(params)
}

func projectMappingToModulation(m *project.ParamMapping) modulation.MappingConfig {
	defaultSine := modulation.MappingConfig{
		Mode: modulation.ModeWaveform,
		Waveform: &modulation.Waveform{
			Kind:      modulation.KindSin,
			Frequency: 1,
			Amplitude: 0.5,
			Offset:    0.5,
		},
	}
	if m == nil {
		return defaultSine
	}
	out := modulation.MappingConfig{
		Mode:        modulation.MappingMode(m.Mode),
		StaticValue:  m.StaticValue,
		AudioBand:    m.AudioBand,
	}
	if m.Waveform != nil {
		out.Waveform = &modulation.Waveform{
			Kind:      modulation.WaveformKind(m.Waveform.Kind),
			Frequency: m.Waveform.Frequency,
			Phase:     m.Waveform.Phase,
			Amplitude: m.Waveform.Amplitude,
			Offset:    m.Waveform.Offset,
		}
	} else if out.Mode == modulation.ModeWaveform {
		out.Waveform = defaultSine.Waveform
	}
	if m.Blend != nil {
		out.Blend = &modulation.BlendWeights{
			Static:   m.Blend.Static,
			Waveform: m.Blend.Waveform,
			Audio:    m.Blend.Audio,
		}
	}
	return out
}
