package shadertoy

import (
	"strings"
	"testing"
)

func TestTransform_addsPrecisionAndUniforms(t *testing.T) {
	src := "void mainImage(out vec4 fragColor, in vec2 f) { fragColor = vec4(1.0); }"
	out := Transform(src, []string{"gain", "palette_mix"})
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "precision mediump float") {
		t.Error("expected precision directive")
	}
	if !strings.Contains(out, "uniform vec2 u_resolution") {
		t.Error("expected u_resolution")
	}
	if !strings.Contains(out, "uniform float u_time") {
		t.Error("expected u_time")
	}
	if !strings.Contains(out, "uniform float u_gain") {
		t.Error("expected u_gain")
	}
	if !strings.Contains(out, "uniform float u_palette_mix") {
		t.Error("expected u_palette_mix")
	}
	if !strings.Contains(out, "iResolution") {
		t.Error("expected iResolution define")
	}
	if !strings.Contains(out, "iTime") {
		t.Error("expected iTime define")
	}
	if !strings.Contains(out, "void main()") {
		t.Error("expected main() wrapper")
	}
	if !strings.Contains(out, "mainImage(gl_FragColor") {
		t.Error("expected mainImage call")
	}
}

func TestExtractViewID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"7cfGzn", "7cfGzn"},
		{"https://www.shadertoy.com/view/7cfGzn", "7cfGzn"},
		{"  https://www.shadertoy.com/view/7cfGzn  ", "7cfGzn"},
	}
	for _, tt := range tests {
		got := ExtractViewID(tt.in)
		if got != tt.want {
			t.Errorf("ExtractViewID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
