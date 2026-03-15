package project_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"audiovisual/internal/project"
)

func TestProjectRoundTripJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.avproj")

	orig := project.Project{
		Name:        "Test Show",
		Description: "A test audiovisual show",
		Shader: project.Shader{
			FragmentSource: "void main() {}",
		},
		Params: []project.Param{
			{
				ID:      "gain",
				Label:   "Gain",
				Min:     0,
				Max:     1,
				Default: 0.5,
				Step:    0.01,
				Type:    project.ParamTypeFloat,
			},
		},
	}

	if err := project.Save(path, &orig); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := project.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Name != orig.Name || got.Description != orig.Description {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, orig)
	}

	if len(got.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(got.Params))
	}

	p := got.Params[0]
	if p.ID != "gain" || p.Min != 0 || p.Max != 1 || p.Default != 0.5 {
		t.Fatalf("param mismatch after round-trip: %+v", p)
	}
}

func TestProjectRoundTripWithMapping(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mapping.avproj")

	orig := project.Project{
		Name: "Audio Show",
		Shader: project.Shader{
			FragmentSource: "void main() { gl_FragColor = vec4(1.0); }",
		},
		Params: []project.Param{
			{
				ID:      "level",
				Label:   "Level",
				Min:     0,
				Max:     1,
				Default: 0.5,
				Step:    0.01,
				Type:    project.ParamTypeFloat,
				Mapping: &project.ParamMapping{
					Mode:      "audio",
					AudioBand: 0,
				},
			},
			{
				ID:      "mix",
				Label:   "Mix",
				Min:     0,
				Max:     1,
				Default: 0.5,
				Type:    project.ParamTypeFloat,
				Mapping: &project.ParamMapping{
					Mode: "blend",
					Blend: &project.BlendWeights{
						Static:   0.2,
						Waveform: 0.3,
						Audio:    0.5,
					},
				},
			},
		},
	}

	if err := project.Save(path, &orig); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := project.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(got.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(got.Params))
	}
	if got.Params[0].Mapping == nil || got.Params[0].Mapping.Mode != "audio" || got.Params[0].Mapping.AudioBand != 0 {
		t.Fatalf("param 0 mapping mismatch: %+v", got.Params[0].Mapping)
	}
	if got.Params[1].Mapping == nil || got.Params[1].Mapping.Blend == nil ||
		got.Params[1].Mapping.Blend.Static != 0.2 || got.Params[1].Mapping.Blend.Audio != 0.5 {
		t.Fatalf("param 1 mapping mismatch: %+v", got.Params[1].Mapping)
	}
}

func TestProjectValidate(t *testing.T) {
	p := project.Project{
		Name: "Invalid",
		Params: []project.Param{
			{
				ID:      "bad",
				Label:   "Bad",
				Min:     1,
				Max:     0,
				Default: 0.5,
				Step:    0.01,
				Type:    project.ParamTypeFloat,
			},
		},
	}

	if err := p.Validate(); err == nil {
		t.Fatalf("expected validation error for min > max")
	}
}

func TestProjectJSONShapeStable(t *testing.T) {
	p := project.Project{
		Name: "ShapeTest",
		Shader: project.Shader{
			FragmentSource: "void main() {}",
		},
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty JSON output")
	}

	if err := os.WriteFile(filepath.Join(t.TempDir(), "shape.avproj"), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
