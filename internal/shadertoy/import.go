package shadertoy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// Transform converts ShaderToy fragment code to audiovisual format:
// - Ensures precision and our uniforms (u_resolution, u_time, u_<paramId>).
// - Adds #define iResolution / iTime so ShaderToy names work.
// - Wraps mainImage in main() if missing.
// paramIDs are optional param names to declare as uniforms (e.g. gain, palette_mix, pulse).
func Transform(fragmentSource string, paramIDs []string) string {
	s := strings.TrimSpace(fragmentSource)
	var out strings.Builder

	// Precision (required in GLSL ES)
	if !strings.HasPrefix(s, "precision") {
		out.WriteString("precision mediump float;\n")
	}

	// Our uniforms
	out.WriteString("uniform vec2 u_resolution;\nuniform float u_time;\n")
	for _, id := range paramIDs {
		out.WriteString("uniform float u_" + id + ";\n")
	}
	out.WriteString("#define iResolution vec3(u_resolution, 1.0)\n#define iTime u_time\n\n")

	// Strip existing precision if present so we don't duplicate
	if strings.HasPrefix(s, "precision") {
		if i := strings.Index(s, ";"); i != -1 {
			s = strings.TrimSpace(s[i+1:])
		}
	}

	// Remove ShaderToy's own iResolution/iTime if present (we defined them)
	// So that pasted code using iResolution.xy etc. still works
	s = regexp.MustCompile(`\buniform\s+vec3\s+iResolution\s*;`).ReplaceAllString(s, "// iResolution from audiovisual")
	s = regexp.MustCompile(`\buniform\s+float\s+iTime\s*;`).ReplaceAllString(s, "// iTime from audiovisual")

	out.WriteString(s)

	// If there's mainImage but no main(), add our main() (with position offset for TUI pan and optional shockwave)
	if strings.Contains(s, "mainImage") && !regexp.MustCompile(`\bvoid\s+main\s*\s*\(`).MatchString(s) {
		mainBody := "  mainImage(gl_FragColor, gl_FragCoord.xy + vec2(u_offset_x, u_offset_y) * u_resolution.xy);\n"
		if sliceContains(paramIDs, "shockwave_trigger") {
			mainBody += `  float t = u_time - u_shockwave_trigger;
  if (t > 0.0) {
    vec2 st = (gl_FragCoord.xy + vec2(u_offset_x, u_offset_y) * u_resolution.xy) / u_resolution.xy;
    float r = length(st - 0.5);
    float ringR = t * 1.2;
    float d = abs(r - ringR);
    float hat = max(0.0, 1.0 - d * 3.0) * exp(-d * d * 4.0);
    gl_FragColor.rgb += hat * vec3(1.0, 0.95, 0.9);
  }
`
		}
		out.WriteString("\n\nvoid main() {\n" + mainBody + "}\n")
	}

	return out.String()
}

func sliceContains(s []string, id string) bool {
	for _, x := range s {
		if x == id {
			return true
		}
	}
	return false
}

// WebGL1Compat applies minimal compatibility fixes for GLSL ES 1.0 (e.g. no tanh).
// Optional; many ShaderToy shaders need manual fixes.
func WebGL1Compat(src string) string {
	// Replace tanh(x) with a manual approximation
	s := regexp.MustCompile(`\btanh\s*\(\s*([^)]+)\s*\)`).ReplaceAllString(src, "myTanh($1)")
	if s != src {
		// Prepend myTanh helper if we replaced tanh
		helper := `
vec4 myTanh(vec4 x) { x = clamp(x, -10.0, 10.0); return (exp(x)-exp(-x))/(exp(x)+exp(-x)); }
vec3 myTanh(vec3 x) { return vec3(myTanh(vec4(x,0.0)).xyz); }
float myTanh(float x) { return myTanh(vec4(x,0.0,0.0,0.0)).x; }
`
		s = helper + "\n" + s
	}
	return s
}

// ShaderToy API response (subset we need). API returns e.g. "Shader" or "shader" with "renderpass" array.
type apiResponse struct {
	Shader *struct {
		Name       string `json:"name"`
		Renderpass []struct {
			Code string `json:"code"`
		} `json:"renderpass"`
	} `json:"Shader"`
	// Some clients use lowercase
	ShaderLower *struct {
		Name       string `json:"name"`
		Renderpass []struct {
			Code string `json:"code"`
		} `json:"renderpass"`
	} `json:"shader"`
}

// FetchShader fetches a Public+API shader by view ID (e.g. "7cfGzn"). API key is required.
func FetchShader(viewID string, apiKey string) (name string, fragmentCode string, err error) {
	url := fmt.Sprintf("https://www.shadertoy.com/api/v1/shaders/%s?key=%s", viewID, apiKey)
	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("fetch shadertoy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("shadertoy API returned %d: %s", resp.StatusCode, string(body))
	}
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode shadertoy response: %w", err)
	}
	if out.Shader != nil && len(out.Shader.Renderpass) > 0 {
		name := out.Shader.Name
		code := out.Shader.Renderpass[0].Code
		if code == "" {
			return "", "", fmt.Errorf("shadertoy: empty fragment code")
		}
		return name, code, nil
	}
	if out.ShaderLower != nil && len(out.ShaderLower.Renderpass) > 0 {
		name := out.ShaderLower.Name
		code := out.ShaderLower.Renderpass[0].Code
		if code == "" {
			return "", "", fmt.Errorf("shadertoy: empty fragment code")
		}
		return name, code, nil
	}
	return "", "", fmt.Errorf("shadertoy: no shader or empty fragment code in response")
}

// ExtractViewID returns the view ID from a ShaderToy URL or the string itself if it looks like an ID.
// e.g. "https://www.shadertoy.com/view/7cfGzn" -> "7cfGzn", "7cfGzn" -> "7cfGzn".
func ExtractViewID(s string) string {
	s = strings.TrimSpace(s)
	// URL pattern
	if idx := strings.LastIndex(s, "/view/"); idx != -1 {
		s = s[idx+6:]
	} else if idx := strings.LastIndex(s, "/"); idx != -1 && idx < len(s)-1 {
		s = s[idx+1:]
	}
	// Drop query string
	if i := strings.Index(s, "?"); i != -1 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
