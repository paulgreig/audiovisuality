package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"audiovisual/internal/project"
	"github.com/gorilla/websocket"
)

func TestHandleShaderReturnsProjectShader(t *testing.T) {
	p := &project.Project{
		Shader: project.Shader{
			FragmentSource: "void main() {}",
		},
		Params: []project.Param{
			{ID: "gain"},
		},
	}

	s := New(":0", p)

	req := httptest.NewRequest(http.MethodGet, "/api/shader", nil)
	rr := httptest.NewRecorder()

	s.handleShader(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp shaderResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.FragmentSource != p.Shader.FragmentSource {
		t.Fatalf("expected fragment_source %q, got %q", p.Shader.FragmentSource, resp.FragmentSource)
	}

	if len(resp.Params) < 1 || resp.Params[0].ID != "gain" {
		t.Fatalf("expected at least gain as first param, got: %+v", resp.Params)
	}
	// Server injects offset_x, offset_y for position view and shockwave_trigger for TUI shockwave
	var hasOffsetX, hasOffsetY, hasShockwave bool
	for _, p := range resp.Params {
		if p.ID == "offset_x" {
			hasOffsetX = true
		}
		if p.ID == "offset_y" {
			hasOffsetY = true
		}
		if p.ID == "shockwave_trigger" {
			hasShockwave = true
		}
	}
	if !hasOffsetX || !hasOffsetY {
		t.Fatalf("expected injected offset_x and offset_y params, got: %+v", resp.Params)
	}
	if !hasShockwave {
		t.Fatalf("expected injected shockwave_trigger param, got: %+v", resp.Params)
	}
	if resp.OverlayURL != "" {
		t.Fatalf("expected no overlay_url when not set, got %q", resp.OverlayURL)
	}
}

func TestHandleShaderIncludesOverlayURLWhenSet(t *testing.T) {
	p := &project.Project{
		Shader:            project.Shader{FragmentSource: "void main() {}"},
		OverlayImagePath:  "cover.png",
	}
	s := New(":0", p)

	req := httptest.NewRequest(http.MethodGet, "/api/shader", nil)
	rr := httptest.NewRecorder()
	s.handleShader(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var resp shaderResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.OverlayURL != "/api/overlay" {
		t.Fatalf("expected overlay_url /api/overlay when set, got %q", resp.OverlayURL)
	}
}

func TestHandleOverlayNotFoundWhenEmpty(t *testing.T) {
	p := &project.Project{}
	s := New(":0", p)

	req := httptest.NewRequest(http.MethodGet, "/api/overlay", nil)
	rr := httptest.NewRecorder()
	s.handleOverlay(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when overlay path empty, got %d", rr.Code)
	}
}

func TestHandleOverlayServesImage(t *testing.T) {
	tmpDir := t.TempDir()
	// Minimal 1x1 PNG
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f,
		0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc, 0xcc, 0x59,
		0xe7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	imgPath := filepath.Join(tmpDir, "cover.png")
	if err := os.WriteFile(imgPath, pngBytes, 0o644); err != nil {
		t.Fatalf("write test image: %v", err)
	}

	p := &project.Project{OverlayImagePath: imgPath}
	s := New(":0", p)

	req := httptest.NewRequest(http.MethodGet, "/api/overlay", nil)
	rr := httptest.NewRecorder()
	s.handleOverlay(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected Content-Type image/png, got %q", ct)
	}
	if rr.Body.Len() != len(pngBytes) {
		t.Fatalf("expected body len %d, got %d", len(pngBytes), rr.Body.Len())
	}
}

func TestHandleRootServesIndexHTML(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<!doctype html><title>test</title>"), 0o644); err != nil {
		t.Fatalf("failed to write temp index.html: %v", err)
	}

	t.Setenv("AUDIOVISUAL_WEB_DIR", tmpDir)

	p := &project.Project{}
	s := New(":0", p)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	s.handleRoot(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if body == "" {
		t.Fatal("expected non-empty body from index.html")
	}
	if body[0] != '<' {
		t.Fatalf("expected HTML content, got: %q", body)
	}
}

func TestHandleParamsWSStreamsValues(t *testing.T) {
	p := &project.Project{
		Params: []project.Param{
			{
				ID:  "gain",
				Min: 0,
				Max: 1,
			},
		},
	}

	s := New(":0", p)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/params", s.handleParamsWS)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	u := "ws" + ts.URL[len("http"):] + "/ws/params"

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	var msg paramsMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("failed to read websocket message: %v", err)
	}

	if len(msg.Params) == 0 {
		t.Fatalf("expected at least one param value, got: %#v", msg.Params)
	}
}

func TestClientParamsOverridesAreMerged(t *testing.T) {
	p := &project.Project{
		Params: []project.Param{
			{
				ID:  "palette_mix",
				Min: 0,
				Max: 1,
			},
		},
	}

	s := New(":0", p)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/params", s.handleParamsWS)
	mux.HandleFunc("/ws/client-params", s.handleClientParamsWS)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	baseURL := "ws" + ts.URL[len("http"):]

	// First, connect client-params WS and send an override.
	clientURL := baseURL + "/ws/client-params"
	dialer := websocket.Dialer{}
	clientConn, _, err := dialer.Dial(clientURL, nil)
	if err != nil {
		t.Fatalf("failed to dial client-params websocket: %v", err)
	}
	defer func() { _ = clientConn.Close() }()

	override := paramsMessage{Params: map[string]float64{"palette_mix": 0.7}}
	if err := clientConn.WriteJSON(&override); err != nil {
		t.Fatalf("failed to write override message: %v", err)
	}

	// Now connect to /ws/params and verify that palette_mix reflects the override.
	paramsURL := baseURL + "/ws/params"
	paramsConn, _, err := dialer.Dial(paramsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial params websocket: %v", err)
	}
	defer func() { _ = paramsConn.Close() }()

	_ = paramsConn.SetReadDeadline(time.Now().Add(2 * time.Second))

	var msg paramsMessage
	if err := paramsConn.ReadJSON(&msg); err != nil {
		t.Fatalf("failed to read websocket message: %v", err)
	}

	v, ok := msg.Params["palette_mix"]
	if !ok {
		t.Fatalf("expected palette_mix in params, got: %#v", msg.Params)
	}
	if v != 0.7 {
		t.Fatalf("expected palette_mix override 0.7, got %v", v)
	}
}
