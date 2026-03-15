package audio_test

import (
	"math"
	"testing"

	"audiovisual/internal/audio"
)

func TestAnalyzerSyntheticSinePeakInBand(t *testing.T) {
	const sampleRate = 44100
	const fftSize = 2048
	const freq = 440.0

	a := audio.NewAnalyzer(audio.AnalyzerConfig{
		FFTSize:    fftSize,
		SampleRate: sampleRate,
		NumBands:   8,
		Smoothing:  0,
	})

	// One block of 440 Hz sine
	samples := make([]float64, fftSize)
	for i := 0; i < fftSize; i++ {
		tSec := float64(i) / sampleRate
		samples[i] = math.Sin(2 * math.Pi * freq * tSec)
	}

	a.ProcessPCM(samples)
	bands := a.Bands()

	// 440 Hz should land in one of the lower bands (band 0 covers 0 to ~430 Hz at 44100/2048 per bin, so 440 is in band 1 or 2)
	// We only assert that some band has non-trivial energy
	var maxBand float64
	for _, v := range bands {
		if v > maxBand {
			maxBand = v
		}
	}
	if maxBand < 0.01 {
		t.Fatalf("expected non-zero band magnitude for 440 Hz sine, got max band %f", maxBand)
	}
}

func TestAnalyzerSilenceYieldsZeroBands(t *testing.T) {
	a := audio.NewAnalyzer(audio.AnalyzerConfig{
		FFTSize:    1024,
		SampleRate: 44100,
		NumBands:   8,
		Smoothing:  0,
	})

	samples := make([]float64, 1024)
	a.ProcessPCM(samples)
	bands := a.Bands()

	for i, v := range bands {
		if v != 0 {
			t.Fatalf("band %d expected 0 for silence, got %f", i, v)
		}
	}
}

func TestAnalyzerSmoothingReducesJitter(t *testing.T) {
	a := audio.NewAnalyzer(audio.AnalyzerConfig{
		FFTSize:    1024,
		SampleRate: 44100,
		NumBands:   8,
		Smoothing:  0.9,
	})

	// First block: silence
	silence := make([]float64, 1024)
	a.ProcessPCM(silence)
	b1 := a.Bands()

	// Second block: tone
	tone := make([]float64, 1024)
	for i := range tone {
		tone[i] = math.Sin(2 * math.Pi * 440 * float64(i) / 44100)
	}
	a.ProcessPCM(tone)
	b2 := a.Bands()

	// With high smoothing, band values should not jump to full scale in one step
	var maxB2 float64
	for _, v := range b2 {
		if v > maxB2 {
			maxB2 = v
		}
	}
	// After one frame, smoothed value is 0.1*new + 0.9*old, so max is at most 0.1*1 + 0.9*0 = 0.1 (approx)
	// So maxB2 should be well below 1
	if maxB2 > 0.5 {
		t.Fatalf("smoothing should dampen single-frame spike; got max band %f (b1=%v)", maxB2, b1)
	}
}

func TestAnalyzerNumBands(t *testing.T) {
	a := audio.NewAnalyzer(audio.AnalyzerConfig{NumBands: 16})
	if a.NumBands() != 16 {
		t.Fatalf("expected 16 bands, got %d", a.NumBands())
	}
}
