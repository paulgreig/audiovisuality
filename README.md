# audiovisual

Audiovisual is a **macOS-focused** tool for creating **audio-reactive visuals** using ShaderToy-style fragment shaders.

- **Terminal UI (TUI)** — Charmbracelet/Bubble Tea interface to configure shaders, parameters, and (when built with audio) input devices.
- **WebGL preview** — Local HTTP server serves a full-screen shader in a browser tab with no on-screen controls, suitable for window capture in OBS or similar.
- **Modulation engine** — Parameters can be driven by waveforms (sin, cos, tan, exp) and, with audio capture, by FFT bands.

Audio analysis is done in Go. For **system audio** on macOS you route output through a virtual device (e.g. [BlackHole](https://github.com/ExistentialAudio/BlackHole)) and select that device as the app’s input.

## Quick start

**Requirements:** Go 1.21+ (see `go.mod`).

```bash
# Clone or enter the repo
cd audiovisuality

# Build and run (no audio capture)
go build -o audiovisual ./cmd/audiovisual
./audiovisual -port 8080
```

- **TUI:** Use `1` (Overview), `2` (Audio), `3` (Params), `4` (Shader), `5` (Position), `6` (Overlay), `b` (Shockwave), `?` (Help), `w` (Save), `q` (Quit).
- **Preview:** Open **http://localhost:8080/** in a browser for the full-screen shader.

**With audio capture** (microphone or virtual device):

```bash
brew install pkg-config portaudio   # macOS (pkg-config required for PortAudio build)
go build -tags=portaudio -o audiovisual ./cmd/audiovisual
./audiovisual -port 8080
```

Or with `go run` (must pass the tag so the portaudio code is included):

```bash
brew install pkg-config portaudio
go run -tags=portaudio ./cmd/audiovisual -port 8080
```

Then open the **Audio** view (`2` or `a`) to see input devices. If you use `go run` instead of a built binary, you must pass the tag: `go run -tags=portaudio ./cmd/audiovisual -port 8080`, otherwise the Audio view will show a message to rebuild with `-tags=portaudio`. See [docs/macos-setup.md](docs/macos-setup.md) for full macOS setup, including BlackHole and system audio.

## Project files

Use `-project` to load a project file (JSON); if the file doesn't exist or fails to load, a default in-memory project is used:

```bash
./audiovisual -project myshow.avproj -port 8080
```

The file defines shader source, parameters (id, label, min, max, default), and optional **mapping** per param so values can be driven by waveforms or audio FFT:

- **`mode`**: `"static"` | `"waveform"` | `"audio"` | `"blend"`
- **`waveform`**: `{ "kind": "sin"|"cos"|"tan"|"exp", "frequency", "phase", "amplitude", "offset" }` for waveform mode
- **`audio_band`**: FFT band index (0..7 with default 8 bands) for `"audio"` mode
- **`blend`**: `{ "static", "waveform", "audio" }` weights for `"blend"` mode

See **examples/audio-reactive-demo.avproj** for a project that uses waveform and audio mapping.

**Position (5):** Pan the shader with offset X/Y (`a`/`d`, `w`/`s` or arrow keys). Values are streamed to the preview.

**Shockwave (b):** One keypress triggers a sombrero-style ring that expands across the preview (shader uniform `u_shockwave_trigger`).

**Overlay / background image (6):** Set an image path (e.g. album art) in the Overlay view; the image is loaded as a WebGL texture (`u_background`) so the shader can use it as a background or sample it. Path is relative to the project file or absolute. Save the project (`w`) to persist `overlay_image_path` in the `.avproj` file. Refresh the browser to see the texture.

For shaders from **ShaderToy** (e.g. [view/7cfGzn](https://www.shadertoy.com/view/7cfGzn)), you can **import** them so they run with variables (gain, palette_mix, pulse) like the template:

```bash
./audiovisual import-shadertoy -id 7cfGzn -key YOUR_API_KEY -output my.avproj
./audiovisual import-shadertoy -stdin -output my.avproj   # paste code, then Ctrl-D
```

Or use **examples/shadertoy-template.avproj** and see [docs/shadertoy.md](docs/shadertoy.md) for manual paste and adaptation.

**Audio input device:** When built with `-tags=portaudio`, use `-device=N` to capture from the Nth input device (index shown in the TUI Audio view, e.g. `[0]`, `[1]`). FFT bands are then sent to the server and can drive parameters that use audio mapping:

```bash
./audiovisual -tags=portaudio -device=0 -port 8080
```

## Development

- **Format:** `go fmt ./...` or `make fmt`
- **Tests:** `go test ./...` or `make test`
- **Lint:** `golangci-lint run ./...` or `make lint` (install from [golangci-lint](https://golangci-lint.run/))
- **Build (no audio):** `make build`
- **Build (with audio):** `make build-audio` (requires PortAudio)

## Documentation

- [macOS setup](docs/macos-setup.md) — Go, PortAudio, BlackHole, virtual devices, and troubleshooting.
- [Using shaders from ShaderToy](docs/shadertoy.md) — Paste from a link (e.g. [view/7cfGzn](https://www.shadertoy.com/view/7cfGzn)) and adapt for testing.

## Repository

Source: [github.com/paulgreig/audiovisuality](https://github.com/paulgreig/audiovisuality).

**Clone:**
```bash
git clone https://github.com/paulgreig/audiovisuality.git
cd audiovisuality
```

**Push from an existing local copy** (e.g. this workspace):
```bash
git init
git add .
git commit -m "Initial commit"
git remote add origin https://github.com/paulgreig/audiovisuality.git
git branch -M main
git push -u origin main
```

If the remote already has history (e.g. an existing README), pull first: `git pull origin main --allow-unrelated-histories`, then push.
