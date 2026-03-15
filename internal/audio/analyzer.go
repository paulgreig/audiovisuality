package audio

import (
	"math"
	"sync"

	"gonum.org/v1/gonum/dsp/fourier"
)

// Analyzer performs FFT on PCM blocks and exposes smoothed band magnitudes (0..1).
type Analyzer struct {
	fftSize    int
	sampleRate float64
	numBands   int
	smoothing  float64 // 0..1, higher = more smoothing
	bands      []float64
	prevBands  []float64
	mu         sync.RWMutex
	fft        *fourier.FFT
}

// AnalyzerConfig configures the FFT analyzer.
type AnalyzerConfig struct {
	FFTSize    int     // power of 2 (e.g. 1024, 2048)
	SampleRate float64 // Hz
	NumBands   int     // number of output bands (e.g. 8)
	Smoothing  float64 // 0..1, exponential smoothing factor
}

// NewAnalyzer creates an analyzer. FFTSize must be a power of 2.
func NewAnalyzer(cfg AnalyzerConfig) *Analyzer {
	if cfg.FFTSize <= 0 {
		cfg.FFTSize = 2048
	}
	if cfg.NumBands <= 0 {
		cfg.NumBands = 8
	}
	if cfg.Smoothing < 0 {
		cfg.Smoothing = 0
	}
	if cfg.Smoothing > 1 {
		cfg.Smoothing = 1
	}
	return &Analyzer{
		fftSize:    cfg.FFTSize,
		sampleRate: cfg.SampleRate,
		numBands:   cfg.NumBands,
		smoothing:  cfg.Smoothing,
		bands:      make([]float64, cfg.NumBands),
		prevBands:  make([]float64, cfg.NumBands),
		fft:        fourier.NewFFT(cfg.FFTSize),
	}
}

// ProcessPCM runs FFT on the given block (length must be >= FFTSize), updates band magnitudes.
// Samples are windowed with Hann. If len(samples) > FFTSize, only the first FFTSize are used.
func (a *Analyzer) ProcessPCM(samples []float64) {
	if len(samples) < a.fftSize {
		return
	}
	win := make([]float64, a.fftSize)
	for i := 0; i < a.fftSize; i++ {
		hann := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(a.fftSize)))
		win[i] = samples[i] * hann
	}
	coeff := make([]complex128, a.fftSize/2+1)
	a.fft.Coefficients(coeff, win)

	// Magnitude spectrum (first half; real FFT symmetry)
	mags := make([]float64, a.fftSize/2+1)
	for i := range coeff {
		mags[i] = math.Sqrt(real(coeff[i])*real(coeff[i]) + imag(coeff[i])*imag(coeff[i]))
	}

	// Aggregate into numBands with log-spaced bin grouping
	bands := a.aggregateBands(mags)
	// Normalize to 0..1 (relative to max so far)
	maxMag := 1e-12
	for _, v := range bands {
		if v > maxMag {
			maxMag = v
		}
	}
	for i := range bands {
		bands[i] /= maxMag
		if bands[i] > 1 {
			bands[i] = 1
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range bands {
		a.bands[i] = a.smoothing*a.prevBands[i] + (1-a.smoothing)*bands[i]
		a.prevBands[i] = a.bands[i]
	}
}

// aggregateBands maps FFT bins to numBands using linear grouping (simple and deterministic).
func (a *Analyzer) aggregateBands(mags []float64) []float64 {
	n := len(mags)
	if n == 0 {
		return make([]float64, a.numBands)
	}
	out := make([]float64, a.numBands)
	binsPerBand := float64(n) / float64(a.numBands)
	for i := 0; i < a.numBands; i++ {
		start := int(float64(i) * binsPerBand)
		end := int(float64(i+1) * binsPerBand)
		if end > n {
			end = n
		}
		var sum float64
		for j := start; j < end; j++ {
			sum += mags[j]
		}
		if end > start {
			out[i] = sum / float64(end-start)
		}
	}
	return out
}

// Bands returns a copy of the current band magnitudes (0..1).
func (a *Analyzer) Bands() []float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]float64, len(a.bands))
	copy(out, a.bands)
	return out
}

// NumBands returns the number of bands.
func (a *Analyzer) NumBands() int {
	return a.numBands
}

// FFTSize returns the FFT block size.
func (a *Analyzer) FFTSize() int {
	return a.fftSize
}
