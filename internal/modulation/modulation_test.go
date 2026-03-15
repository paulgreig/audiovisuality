package modulation_test

import (
	"math"
	"testing"

	"audiovisual/internal/modulation"
)

func TestWaveformSinPeriodAndRange(t *testing.T) {
	w := modulation.Waveform{
		Kind:      modulation.KindSin,
		Frequency: 1,
		Phase:     0,
		Amplitude: 0.5,
		Offset:    0.5,
	}
	// sin(0)=0 -> 0.5 + 0.5*0 = 0.5
	if v := w.ValueAt(0); math.Abs(v-0.5) > 1e-9 {
		t.Fatalf("at t=0 expected 0.5, got %f", v)
	}
	// sin(pi/2)=1 at t = 1/4 period -> 0.5 + 0.5 = 1
	period := 1.0 / w.Frequency
	tQuarter := period / 4
	if v := w.ValueAt(tQuarter); math.Abs(v-1) > 1e-6 {
		t.Fatalf("at t=period/4 expected 1, got %f", v)
	}
	// sin(pi)=0 -> 0.5
	if v := w.ValueAt(period / 2); math.Abs(v-0.5) > 1e-6 {
		t.Fatalf("at t=period/2 expected 0.5, got %f", v)
	}
}

func TestWaveformCos(t *testing.T) {
	w := modulation.Waveform{
		Kind:      modulation.KindCos,
		Frequency: 1,
		Phase:     0,
		Amplitude: 0.5,
		Offset:    0.5,
	}
	// cos(0)=1 -> 0.5+0.5=1
	if v := w.ValueAt(0); math.Abs(v-1) > 1e-9 {
		t.Fatalf("cos at t=0 expected 1, got %f", v)
	}
}

func TestWaveformValueInRange(t *testing.T) {
	w := modulation.Waveform{
		Kind:      modulation.KindSin,
		Frequency: 0.5,
		Amplitude: 0.5,
		Offset:    0.5,
	}
	for _, tSec := range []float64{0, 0.25, 0.5, 1, 2, 10} {
		v := w.ValueAt(tSec)
		if v < 0 || v > 1 {
			t.Fatalf("at t=%f expected value in [0,1], got %f", tSec, v)
		}
	}
}

func TestEngineStaticMode(t *testing.T) {
	e := modulation.NewEngine()
	e.AddParam("x", 0, 1, 0.5, modulation.MappingConfig{
		Mode:        modulation.ModeStatic,
		StaticValue: 0.7,
	})
	got := e.Compute(0, nil)
	if len(got) != 1 || math.Abs(got["x"]-0.7) > 1e-9 {
		t.Fatalf("expected static 0.7, got %v", got)
	}
	got2 := e.Compute(100, nil)
	if math.Abs(got2["x"]-0.7) > 1e-9 {
		t.Fatalf("static should not change over time, got %v", got2)
	}
}

func TestEngineWaveformMode(t *testing.T) {
	e := modulation.NewEngine()
	e.AddParam("gain", 0, 1, 0.5, modulation.MappingConfig{
		Mode: modulation.ModeWaveform,
		Waveform: &modulation.Waveform{
			Kind:      modulation.KindSin,
			Frequency: 1,
			Amplitude: 0.5,
			Offset:    0.5,
		},
	})
	got0 := e.Compute(0, nil)
	got1 := e.Compute(0.25, nil) // period/4
	if math.Abs(got0["gain"]-got1["gain"]) < 1e-6 {
		t.Fatalf("waveform should change over time: %v vs %v", got0, got1)
	}
	// Value should be within param range
	if got1["gain"] < 0 || got1["gain"] > 1 {
		t.Fatalf("gain should be clamped to [0,1], got %f", got1["gain"])
	}
}

func TestEngineBlendRespectsWeights(t *testing.T) {
	e := modulation.NewEngine()
	e.AddParam("p", 0, 1, 0.5, modulation.MappingConfig{
		Mode:        modulation.ModeBlend,
		StaticValue: 0.2,
		Waveform: &modulation.Waveform{
			Kind:      modulation.KindSin,
			Frequency: 1,
			Amplitude: 0.5,
			Offset:    0.5,
		},
		Blend: &modulation.BlendWeights{Static: 1, Waveform: 0, Audio: 0},
	})
	got := e.Compute(0, nil)
	if math.Abs(got["p"]-0.2) > 1e-6 {
		t.Fatalf("blend static-only expected 0.2, got %f", got["p"])
	}
}

func TestEngineClampsToParamRange(t *testing.T) {
	e := modulation.NewEngine()
	e.AddParam("p", 0.1, 0.9, 0.5, modulation.MappingConfig{
		Mode:        modulation.ModeStatic,
		StaticValue: 2,
	})
	got := e.Compute(0, nil)
	if math.Abs(got["p"]-0.9) > 1e-9 {
		t.Fatalf("expected clamp to max 0.9, got %f", got["p"])
	}
}

func TestEngineMissingAudioTreatedAsZero(t *testing.T) {
	e := modulation.NewEngine()
	e.AddParam("p", 0, 1, 0.5, modulation.MappingConfig{
		Mode:  modulation.ModeAudio,
		Blend: &modulation.BlendWeights{Audio: 1},
	})
	got := e.Compute(0, nil)
	if got["p"] != 0 {
		t.Fatalf("no audio expected 0 (min), got %f", got["p"])
	}
	gotWithAudio := e.Compute(0, []float64{0.5})
	if math.Abs(gotWithAudio["p"]-0.5) > 1e-6 {
		t.Fatalf("with audio band 0.5 expected param 0.5, got %f", gotWithAudio["p"])
	}
}
