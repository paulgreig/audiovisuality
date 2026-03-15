package engine

import (
	"testing"

	"audiovisual/internal/project"
)

func TestBuildEngineEmptyProject(t *testing.T) {
	p := &project.Project{
		Shader: project.Shader{FragmentSource: "void main() {}"},
		Params: nil,
	}
	e := BuildEngine(p)
	if e == nil {
		t.Fatal("BuildEngine returned nil")
	}
	got := e.Compute(0, nil)
	if len(got) != 0 {
		t.Fatalf("expected 0 params, got %d", len(got))
	}
}

func TestBuildEngineWithMappingUsesProjectMapping(t *testing.T) {
	p := &project.Project{
		Params: []project.Param{
			{
				ID:      "x",
				Min:     0,
				Max:     1,
				Default: 0.5,
				Mapping: &project.ParamMapping{
					Mode:        "static",
					StaticValue: 0.8,
				},
			},
		},
	}
	e := BuildEngine(p)
	got := e.Compute(0, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 param, got %d", len(got))
	}
	if got["x"] != 0.8 {
		t.Fatalf("expected static 0.8, got %f", got["x"])
	}
}

func TestBuildEngineNilMappingUsesDefaultWaveform(t *testing.T) {
	p := &project.Project{
		Params: []project.Param{
			{ID: "g", Min: 0, Max: 1, Default: 0.5},
		},
	}
	e := BuildEngine(p)
	got0 := e.Compute(0, nil)
	got1 := e.Compute(0.25, nil)
	if got0["g"] == got1["g"] {
		t.Fatalf("default waveform should vary over time: %f vs %f", got0["g"], got1["g"])
	}
}

func TestBuildEngineAudioModeUsesBands(t *testing.T) {
	p := &project.Project{
		Params: []project.Param{
			{
				ID:    "level",
				Min:   0,
				Max:   1,
				Default: 0,
				Mapping: &project.ParamMapping{
					Mode:      "audio",
					AudioBand: 0,
				},
			},
		},
	}
	e := BuildEngine(p)
	got := e.Compute(0, []float64{0.6})
	if got["level"] < 0.5 || got["level"] > 0.7 {
		t.Fatalf("expected level ~0.6 from audio band, got %f", got["level"])
	}
	gotNil := e.Compute(0, nil)
	if gotNil["level"] != 0 {
		t.Fatalf("no audio expected 0 (min), got %f", gotNil["level"])
	}
}

func TestBuildEngineProjectMappingToModulation(t *testing.T) {
	// Ensure waveform from project is passed through
	p := &project.Project{
		Params: []project.Param{
			{
				ID: "w", Min: 0, Max: 1, Default: 0.5,
				Mapping: &project.ParamMapping{
					Mode: "waveform",
					Waveform: &project.WaveformConfig{
						Kind: "cos", Frequency: 2, Amplitude: 0.5, Offset: 0.5,
					},
				},
			},
		},
	}
	e := BuildEngine(p)
	got := e.Compute(0, nil)
	// cos(0)=1 -> 0.5+0.5=1
	if got["w"] < 0.99 || got["w"] > 1.01 {
		t.Fatalf("cos at t=0 expected ~1, got %f", got["w"])
	}
}
