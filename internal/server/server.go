package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"audiovisual/internal/engine"
	"audiovisual/internal/project"
	"github.com/gorilla/websocket"
)

type Server struct {
	addr       string
	project    *project.Project
	projectDir string // directory of project file (for resolving relative overlay path)
	audioBands func() []float64 // optional; when set, passed to modulation engine

	overridesMu sync.RWMutex
	overrides   map[string]float64
}

func New(addr string, p *project.Project) *Server {
	return &Server{
		addr:      addr,
		project:   p,
		overrides: make(map[string]float64),
	}
}

// SetAudioBands sets an optional supplier of current FFT band magnitudes (0..1).
// When set, these are passed to the modulation engine for params using ModeAudio or ModeBlend.
func (s *Server) SetAudioBands(fn func() []float64) {
	s.audioBands = fn
}

// SetProjectDir sets the directory of the project file so overlay image path can be resolved when relative.
func (s *Server) SetProjectDir(dir string) {
	s.projectDir = dir
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/api/shader", s.handleShader)
	mux.HandleFunc("/api/overlay", s.handleOverlay)
	mux.HandleFunc("/ws/params", s.handleParamsWS)
	mux.HandleFunc("/ws/client-params", s.handleClientParamsWS)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(s.webDir()))))

	server := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	log.Printf("HTTP server listening on %s\n", s.addr)
	return server.ListenAndServe()
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.webDir(), "index.html"))
}

type shaderResponse struct {
	FragmentSource string          `json:"fragment_source"`
	Params         []project.Param `json:"params"`
	OverlayURL     string          `json:"overlay_url,omitempty"`
}

func (s *Server) handleShader(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	params := s.project.Params
	// Inject offset_x, offset_y so every shader can receive pan/position from the TUI.
	if !hasParam(params, "offset_x") {
		params = append(params, project.Param{
			ID: "offset_x", Label: "Offset X", Min: -1, Max: 1, Default: 0, Step: 0.01, Type: project.ParamTypeFloat,
		})
	}
	if !hasParam(params, "offset_y") {
		params = append(params, project.Param{
			ID: "offset_y", Label: "Offset Y", Min: -1, Max: 1, Default: 0, Step: 0.01, Type: project.ParamTypeFloat,
		})
	}
	if !hasParam(params, "shockwave_trigger") {
		params = append(params, project.Param{
			ID: "shockwave_trigger", Label: "Shockwave", Min: 0, Max: 1e6, Default: 0, Step: 0, Type: project.ParamTypeFloat,
		})
	}
	resp := shaderResponse{
		FragmentSource: s.project.Shader.FragmentSource,
		Params:         params,
	}
	if s.project.OverlayImagePath != "" {
		resp.OverlayURL = "/api/overlay"
		// Inject background texture uniforms so the client can pass the overlay; shaders that use them will show the image.
		if !strings.Contains(resp.FragmentSource, "u_background") {
			// Insert after precision line so we don't break #version or precision
			const inject = "uniform sampler2D u_background;\nuniform float u_has_background;\n"
			if idx := strings.Index(resp.FragmentSource, "void main()"); idx != -1 {
				resp.FragmentSource = resp.FragmentSource[:idx] + inject + resp.FragmentSource[idx:]
			} else {
				resp.FragmentSource = inject + resp.FragmentSource
			}
		}
	}

	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		http.Error(w, "failed to encode shader response", http.StatusInternalServerError)
	}
}

// handleOverlay serves the overlay image when project has OverlayImagePath set.
func (s *Server) handleOverlay(w http.ResponseWriter, r *http.Request) {
	path := s.project.OverlayImagePath
	if path == "" {
		http.NotFound(w, r)
		return
	}
	if !filepath.IsAbs(path) {
		base := s.projectDir
		if base == "" {
			if wd, err := os.Getwd(); err == nil {
				base = wd
			}
		}
		path = filepath.Join(base, path)
	}
	path = filepath.Clean(path)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		log.Printf("overlay image read error: %v", err)
		http.Error(w, "failed to read overlay image", http.StatusInternalServerError)
		return
	}
	ext := filepath.Ext(path)
	switch ext {
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func hasParam(params []project.Param, id string) bool {
	for _, p := range params {
		if p.ID == id {
			return true
		}
	}
	return false
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type paramsMessage struct {
	Params map[string]float64 `json:"params"`
}

// applyOverrides merges any client-provided overrides into the base param map.
// Overrides win over base values.
func (s *Server) applyOverrides(base map[string]float64) map[string]float64 {
	s.overridesMu.RLock()
	defer s.overridesMu.RUnlock()
	if len(s.overrides) == 0 {
		return base
	}
	out := make(map[string]float64, len(base)+len(s.overrides))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range s.overrides {
		out[k] = v
	}
	return out
}

func (s *Server) handleParamsWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	start := time.Now()
	ticker := time.NewTicker(time.Second / 30)
	defer ticker.Stop()

	engine := engine.BuildEngine(s.project)

	for range ticker.C {
		elapsed := time.Since(start).Seconds()
		var bands []float64
		if s.audioBands != nil {
			bands = s.audioBands()
		}
		values := engine.Compute(elapsed, bands)
		values = s.applyOverrides(values)
		// Ensure offset_x, offset_y exist so the client always has pan uniforms (default 0).
		if _, ok := values["offset_x"]; !ok {
			values["offset_x"] = 0
		}
		if _, ok := values["offset_y"]; !ok {
			values["offset_y"] = 0
		}
		if _, ok := values["shockwave_trigger"]; !ok {
			values["shockwave_trigger"] = 0
		}
		msg := paramsMessage{Params: values}
		if err := conn.WriteJSON(&msg); err != nil {
			log.Printf("websocket write error: %v", err)
			return
		}
	}
}

// handleClientParamsWS accepts browser-provided param overrides (e.g. palette_mix from Web Audio FFT)
// and stores them so they can be merged into the main /ws/params stream.
func (s *Server) handleClientParamsWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("client params websocket upgrade error: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	for {
		var msg paramsMessage
		if err := conn.ReadJSON(&msg); err != nil {
			// Normal close or error: just stop listening on this connection.
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			log.Printf("client params websocket read error: %v", err)
			return
		}
		if len(msg.Params) == 0 {
			continue
		}
		s.overridesMu.Lock()
		for k, v := range msg.Params {
			s.overrides[k] = v
		}
		s.overridesMu.Unlock()
	}
}

func (s *Server) webDir() string {
	if dir := os.Getenv("AUDIOVISUAL_WEB_DIR"); dir != "" {
		return dir
	}
	// Prefer "web" in current working directory (e.g. when run from repo root).
	if info, err := os.Stat("web"); err == nil && info.IsDir() {
		return "web"
	}
	// Fallback: web next to the executable (e.g. when run from another directory).
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "web")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return "web"
}
