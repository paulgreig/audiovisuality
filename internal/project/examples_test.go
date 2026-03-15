package project

import "testing"

func TestExampleProjectsLoadAndValidate(t *testing.T) {
	t.Parallel()

	paths := []string{
		"../../examples/audio-reactive-demo.avproj",
		"../../examples/shadertoy-template.avproj",
	}

	for _, path := range paths {
		p, err := Load(path)
		if err != nil {
			t.Fatalf("Load(%q) failed: %v", path, err)
		}
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate(%q) failed: %v", path, err)
		}
	}
}

