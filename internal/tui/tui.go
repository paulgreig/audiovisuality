package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"audiovisual/internal/audio"
	"audiovisual/internal/project"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

type viewKind string

const (
	viewOverview viewKind = "overview"
	viewAudio    viewKind = "audio"
	viewParams   viewKind = "params"
	viewShader   viewKind = "shader"
	viewPosition viewKind = "position"
	viewOverlay  viewKind = "overlay"
	viewHelp     viewKind = "help"
)

// paramsUpdateMsg carries param values received from /ws/params (engine + browser overrides).
type paramsUpdateMsg struct {
	Params map[string]float64
}

type Model struct {
	project     *project.Project
	port        int
	projectPath string
	paramValues map[string]float64
	start       time.Time

	// paramsCh receives merged param values from the server (/ws/params stream).
	paramsCh <-chan paramsUpdateMsg

	// navigation
	currentView viewKind

	// audio view
	audioDevices []audio.DeviceInfo
	audioErr     error
	audioLoaded  bool

	// save result (shown in footer after save attempt)
	saveResult string

	// position view: shader pan (sent to server as overrides)
	offsetX, offsetY float64
	sendOverridesCh  chan<- map[string]float64

	// overlay view: image path (e.g. album cover) shown over shader
	overlayPath string
}

func NewModel(p *project.Project, port int, projectPath string, paramsCh <-chan paramsUpdateMsg, sendOverridesCh chan<- map[string]float64) Model {
	m := Model{
		project:         p,
		port:            port,
		projectPath:     projectPath,
		paramValues:     make(map[string]float64),
		paramsCh:        paramsCh,
		sendOverridesCh: sendOverridesCh,
		start:           time.Now(),
		currentView:     viewOverview,
	}
	m.overlayPath = p.OverlayImagePath
	return m
}

// waitForParamsCmd returns a Cmd that blocks until the next params update from the server.
func waitForParamsCmd(ch <-chan paramsUpdateMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

type audioDevicesMsg struct {
	devices []audio.DeviceInfo
	err     error
}

func loadAudioDevices() tea.Msg {
	devices, err := audio.ListDevices()
	return audioDevicesMsg{devices: devices, err: err}
}

type saveResultMsg struct {
	err error
}

func saveProject(path string, p *project.Project) tea.Msg {
	err := project.Save(path, p)
	return saveResultMsg{err: err}
}

func (m Model) Init() tea.Cmd {
	return waitForParamsCmd(m.paramsCh)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Overlay view: line edit for image path
		if m.currentView == viewOverlay {
			switch msg.Type {
			case tea.KeyEnter:
				m.project.OverlayImagePath = m.overlayPath
				return m, nil
			case tea.KeyBackspace:
				if len(m.overlayPath) > 0 {
					r := []rune(m.overlayPath)
					m.overlayPath = string(r[:len(r)-1])
				}
				return m, nil
			case tea.KeyRunes:
				m.overlayPath += string(msg.Runes)
				return m, nil
			}
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1", "o":
			m.currentView = viewOverview
			return m, nil
		case "2", "a":
			if m.currentView == viewPosition && msg.String() == "a" && m.sendOverridesCh != nil {
				m.offsetX -= 0.05
				if m.offsetX < -1 {
					m.offsetX = -1
				}
				select {
				case m.sendOverridesCh <- map[string]float64{"offset_x": m.offsetX, "offset_y": m.offsetY}:
				default:
				}
				return m, nil
			}
			m.currentView = viewAudio
			if !m.audioLoaded {
				m.audioLoaded = true
				return m, func() tea.Msg { return loadAudioDevices() }
			}
			return m, nil
		case "d":
			if m.currentView == viewPosition && m.sendOverridesCh != nil {
				m.offsetX += 0.05
				if m.offsetX > 1 {
					m.offsetX = 1
				}
				select {
				case m.sendOverridesCh <- map[string]float64{"offset_x": m.offsetX, "offset_y": m.offsetY}:
				default:
				}
				return m, nil
			}
		case "3", "p":
			m.currentView = viewParams
			return m, nil
		case "4", "s":
			if m.currentView == viewPosition && m.sendOverridesCh != nil && msg.String() == "s" {
				m.offsetY -= 0.05
				if m.offsetY < -1 {
					m.offsetY = -1
				}
				select {
				case m.sendOverridesCh <- map[string]float64{"offset_x": m.offsetX, "offset_y": m.offsetY}:
				default:
				}
				return m, nil
			}
			m.currentView = viewShader
			return m, nil
		case "5", "x":
			m.currentView = viewPosition
			return m, nil
		case "6", "v":
			m.currentView = viewOverlay
			return m, nil
		case "?", "h":
			m.currentView = viewHelp
			return m, nil
		case "b":
			// Single key: trigger shockwave (sombrero ring) on the shader.
			if m.sendOverridesCh != nil {
				trigger := time.Since(m.start).Seconds()
				overrides := map[string]float64{
					"offset_x":          m.offsetX,
					"offset_y":          m.offsetY,
					"shockwave_trigger": trigger,
				}
				select {
				case m.sendOverridesCh <- overrides:
				default:
				}
			}
			return m, nil
		case "w":
			if m.currentView == viewPosition && m.sendOverridesCh != nil {
				m.offsetY += 0.05
				if m.offsetY > 1 {
					m.offsetY = 1
				}
				select {
				case m.sendOverridesCh <- map[string]float64{"offset_x": m.offsetX, "offset_y": m.offsetY}:
				default:
				}
				return m, nil
			}
			if m.projectPath != "" {
				m.project.OverlayImagePath = m.overlayPath
				return m, func() tea.Msg { return saveProject(m.projectPath, m.project) }
			}
			m.saveResult = "no project path (start with -project to save)"
			return m, nil
		case "left", "right", "up", "down":
			if m.currentView == viewPosition && m.sendOverridesCh != nil {
				step := 0.05
				switch msg.String() {
				case "left":
					m.offsetX -= step
				case "right":
					m.offsetX += step
				case "down":
					m.offsetY -= step
				case "up":
					m.offsetY += step
				}
				if m.offsetX < -1 {
					m.offsetX = -1
				}
				if m.offsetX > 1 {
					m.offsetX = 1
				}
				if m.offsetY < -1 {
					m.offsetY = -1
				}
				if m.offsetY > 1 {
					m.offsetY = 1
				}
				select {
				case m.sendOverridesCh <- map[string]float64{"offset_x": m.offsetX, "offset_y": m.offsetY}:
				default:
				}
				return m, nil
			}
		}
	case saveResultMsg:
		if msg.err != nil {
			m.saveResult = "save failed: " + msg.err.Error()
		} else {
			m.saveResult = "saved to " + m.projectPath
		}
		return m, nil
	case audioDevicesMsg:
		m.audioDevices = msg.devices
		m.audioErr = msg.err
		return m, nil
	case paramsUpdateMsg:
		if msg.Params != nil {
			m.paramValues = make(map[string]float64, len(msg.Params))
			for k, v := range msg.Params {
				m.paramValues[k] = v
			}
			// Keep position view in sync with server
			if v, ok := msg.Params["offset_x"]; ok {
				m.offsetX = v
			}
			if v, ok := msg.Params["offset_y"]; ok {
				m.offsetY = v
			}
		}
		return m, waitForParamsCmd(m.paramsCh)
	}
	return m, nil
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	urlStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	header := titleStyle.Render("audiovisual")
	previewURL := fmt.Sprintf("http://localhost:%d/", m.port)
	urlLine := urlStyle.Render(previewURL)
	footer := keyStyle.Render("1 Overview  2 Audio  3 Params  4 Shader  5 Position  6 Overlay  b Shockwave  ? Help  w Save  q Quit")
	if m.saveResult != "" {
		footer += "\n" + m.saveResult
	}

	var body string
	switch m.currentView {
	case viewOverview:
		body = m.viewOverview()
	case viewAudio:
		body = m.viewAudio()
	case viewParams:
		body = m.viewParams()
	case viewShader:
		body = m.viewShader()
	case viewPosition:
		body = m.viewPosition()
	case viewOverlay:
		body = m.viewOverlay()
	case viewHelp:
		body = m.viewHelp()
	default:
		body = m.viewOverview()
	}

	return strings.Join([]string{
		header,
		"",
		"Preview: " + urlLine,
		"",
		body,
		"",
		footer,
	}, "\n") + "\n"
}

func (m Model) viewOverview() string {
	var b strings.Builder
	projectName := m.project.Name
	if projectName == "" {
		projectName = "(unnamed project)"
	}
	b.WriteString(fmt.Sprintf("Project: %s\n", projectName))
	projectFile := m.projectPath
	if projectFile == "" {
		projectFile = "(default in-memory)"
	}
	b.WriteString(fmt.Sprintf("Project file: %s\n", projectFile))
	if m.project.Description != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n", m.project.Description))
	}
	b.WriteString("\nStreamed values:\n")
	if len(m.project.Params) == 0 {
		b.WriteString("  No parameters defined.\n")
	} else {
		for _, p := range m.project.Params {
			label := p.Label
			if label == "" {
				label = p.ID
			}
			v := m.paramValues[p.ID]
			b.WriteString(fmt.Sprintf("  %s: %.3f\n", label, v))
		}
	}
	return b.String()
}

func (m Model) viewAudio() string {
	var b strings.Builder
	b.WriteString("Audio input devices\n")
	b.WriteString("(Build with -tags=portaudio for capture.)\n\n")
	if !m.audioLoaded {
		b.WriteString("Loading...\n")
		return b.String()
	}
	if m.audioErr != nil {
		b.WriteString(fmt.Sprintf("Error: %v\n", m.audioErr))
		return b.String()
	}
	if len(m.audioDevices) == 0 {
		b.WriteString("No input devices found.\n")
		return b.String()
	}
	for i, d := range m.audioDevices {
		b.WriteString(fmt.Sprintf("  [%d] %s (channels: %d, rate: %.0f Hz)\n", i, d.Name, d.MaxInputChannels, d.DefaultSampleRate))
	}
	if len(m.audioDevices) > 0 {
		restart := "-device=N"
		if m.projectPath != "" {
			restart = fmt.Sprintf("-project %s -device=N", m.projectPath)
		}
		b.WriteString(fmt.Sprintf("\nTo capture: restart with %s (replace N with device index above).\n", restart))
	}
	return b.String()
}

func (m Model) viewPosition() string {
	return fmt.Sprintf(`Shader position (pan)

  Offset X: %.3f   (a / d or left / right)
  Offset Y: %.3f   (w / s or up / down)

  Values are in -1..1 and pan the shader; the preview updates live.
`, m.offsetX, m.offsetY)
}

func (m Model) viewOverlay() string {
	const maxPathLen = 60
	path := m.overlayPath
	if path == "" {
		path = "(none — type path and Enter to set)"
	} else if len(path) > maxPathLen {
		path = "..." + path[len(path)-maxPathLen+3:]
	}
	return fmt.Sprintf(`Overlay image (e.g. album cover)

  Path: %s

  Type the image path (relative to project file or absolute), then Enter to apply.
  Backspace to delete. Refresh the preview in the browser to see changes.
  Supported: PNG, JPEG, GIF, WebP.
`, path)
}

func (m Model) viewParams() string {
	var b strings.Builder
	b.WriteString("Parameters\n\n")
	if len(m.project.Params) == 0 {
		b.WriteString("No parameters. Add params in the project file.\n")
		return b.String()
	}
	for _, p := range m.project.Params {
		label := p.Label
		if label == "" {
			label = p.ID
		}
		v := m.paramValues[p.ID]
		b.WriteString(fmt.Sprintf("  %s\n    id: %s  min: %.2f  max: %.2f  default: %.2f\n    current: %.3f\n\n", label, p.ID, p.Min, p.Max, p.Default, v))
	}
	return b.String()
}

func (m Model) viewShader() string {
	const maxLines = 24
	lines := strings.Split(m.project.Shader.FragmentSource, "\n")
	if len(lines) == 0 {
		return "(empty shader)"
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		return strings.Join(lines, "\n") + "\n\n... (truncated)"
	}
	return strings.Join(lines, "\n")
}

func (m Model) viewHelp() string {
	return `Keybindings
  1, o    Overview (project info + streamed values)
  2, a    Audio (input device list)
  3, p    Params (parameter definitions and current values)
  4, s    Shader (fragment source)
  5, x    Position (offset X/Y to pan the shader)
  6, v    Overlay — image (e.g. album cover) displayed over the shader
  b       Shockwave — trigger a sombrero-style ring that travels across the preview
  ?, h    This help
  w       Save project to file (uses -project path)
  q       Quit

Overlay view (6)
  Type path to image file (relative to project dir or absolute), Enter to apply.
  Backspace to edit. Refresh browser preview to see the overlay.

Position view (5)
  a, d or left, right   Adjust offset X
  w, s or up, down     Adjust offset Y
  Values -1..1 pan the preview; updates stream to the browser.

Preview
  Open the URL shown at the top in a browser to see the full-screen
  WebGL shader. Use that window for streaming (e.g. OBS) with no UI.`
}

// Run starts the TUI. It subscribes to the server's /ws/params stream so displayed
// values reflect both the engine and any browser overrides (e.g. palette_mix from Web Audio).
func Run(p *project.Project, port int, projectPath string) error {
	ch := make(chan paramsUpdateMsg, 16)
	sendCh := make(chan map[string]float64, 8)
	go runParamsStream(port, ch)
	go runClientParamsSender(port, sendCh)
	model := NewModel(p, port, projectPath, ch, sendCh)
	pgm := tea.NewProgram(model, tea.WithAltScreen())
	_, err := pgm.Run()
	return err
}

// runClientParamsSender sends param overrides (e.g. offset_x, offset_y from Position view) to the server.
func runClientParamsSender(port int, ch <-chan map[string]float64) {
	url := "ws://localhost:" + strconv.Itoa(port) + "/ws/client-params"
	for {
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		for msg := range ch {
			if len(msg) == 0 {
				continue
			}
			if err := conn.WriteJSON(struct {
				Params map[string]float64 `json:"params"`
			}{Params: msg}); err != nil {
				_ = conn.Close()
				break
			}
		}
		_ = conn.Close()
		return
	}
}

// runParamsStream dials /ws/params and sends merged param updates to ch. Reconnects on error.
func runParamsStream(port int, ch chan<- paramsUpdateMsg) {
	url := "ws://localhost:" + strconv.Itoa(port) + "/ws/params"
	for {
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		for {
			var msg struct {
				Params map[string]float64 `json:"params"`
			}
			if err := conn.ReadJSON(&msg); err != nil {
				_ = conn.Close()
				break
			}
			select {
			case ch <- paramsUpdateMsg{Params: msg.Params}:
			default:
				// channel full, drop this frame
			}
		}
	}
}
