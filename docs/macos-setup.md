# macOS setup guide

This guide walks through setting up the audiovisual app on macOS: installing dependencies, optional audio capture for system sound, and running the app.

## Requirements

- **macOS** (tested on recent versions; Intel or Apple Silicon)
- **Go 1.21+** (check `go.mod` for the exact version)
- For **audio capture**: PortAudio and a virtual audio device (e.g. BlackHole) if you want to drive visuals from system audio

## Install Go

Install Go using one of the following:

- **Homebrew:** `brew install go`
- **Official installer:** [go.dev/dl](https://go.dev/dl/) — pick the macOS package for your architecture (ARM64 for Apple Silicon, AMD64 for Intel)

Verify:

```bash
go version
```

## Build and run (no audio capture)

You can build and run without any extra system libraries:

```bash
cd /path/to/audiovisual
go build -o audiovisual ./cmd/audiovisual
./audiovisual -port 8080
```

Or run directly:

```bash
go run ./cmd/audiovisual -port 8080
```

- The **TUI** runs in the terminal. Use `1`–`4`, `?`, and `q` to switch views and quit.
- Open **http://localhost:8080/** in a browser to see the full-screen WebGL shader (no on-screen controls).

## Optional: audio capture (microphone or system audio)

To use an audio input device (e.g. microphone or a virtual device that carries system audio), you need:

1. **PortAudio** (C library) so the Go app can open an input stream.
2. **pkg-config** so the Go build can find PortAudio (the bindings use it for compiler/linker flags).
3. **Build with the `portaudio` tag** so the app includes the capture code.

### Install PortAudio and pkg-config

Using Homebrew:

```bash
brew install pkg-config portaudio
```

Build the app with capture support:

```bash
go build -tags=portaudio -o audiovisual ./cmd/audiovisual
./audiovisual -port 8080
```

Or run without building a binary (the `-tags=portaudio` must come before the package path):

```bash
go run -tags=portaudio ./cmd/audiovisual -port 8080
```

In the TUI, press `2` or `a` to open the **Audio** view and see the list of input devices. Device selection (which device to capture from) will be configurable in a future update.

### Using system audio (e.g. for streaming music into the visuals)

macOS does not allow apps to capture “system output” directly. To drive the visuals from whatever is playing (e.g. browser, Spotify), you need a **virtual audio device** that receives system output and exposes it as an input:

1. **Install BlackHole** (free, open source):  
   [https://github.com/ExistentialAudio/BlackHole](https://github.com/ExistentialAudio/BlackHole)  
   Install the 2ch version unless you need multichannel.

2. **Create a Multi-Output Device** (so sound still goes to your speakers/headphones and to BlackHole):
   - Open **Audio MIDI Setup** (Applications → Utilities, or Spotlight).
   - Menu **Window → Show Audio Devices**.
   - Click **+** at bottom left → **Create Multi-Output Device**.
   - Enable **BlackHole 2ch** and your normal output (e.g. **MacBook Pro Speakers** or your DAC). Optionally set the Multi-Output Device as the system output so all apps use it.

3. **Route system output through the Multi-Output Device**  
   In **System Settings → Sound → Output**, select the Multi-Output Device. System audio will go to both your speakers and BlackHole.

4. **Select BlackHole as the input in audiovisual**  
   When device selection is implemented in the TUI, choose the BlackHole input. The app will then run FFT on that stream and you can map bands to shader parameters.

Until then, you can still use the built-in microphone or any other input device that appears in the Audio view (with a `-tags=portaudio` build).

## Project files

- Use the **`-project`** flag to load a project file:  
  `./audiovisual -project myshow.avproj -port 8080`  
  If the file does not exist or is invalid, a default in-memory project is used.
- Project files are **JSON** and contain shader source, parameters, and (in the future) audio and mapping settings.
- **`-device=N`** (when built with `-tags=portaudio`): capture from the Nth input device (index from the Audio view). FFT bands are sent to the server for parameters that use audio mapping.

## Troubleshooting

- **“pkg-config: executable file not found”**  
  You’re building with `-tags=portaudio` but `pkg-config` is not installed. Install it and PortAudio with `brew install pkg-config portaudio`.

- **“No input devices found”**  
  With a `-tags=portaudio` build, the Audio view lists devices from PortAudio. If the list is empty, check that your microphone or virtual device is connected and allowed in **System Settings → Privacy & Security → Microphone** (and that the app is not sandboxed in a way that blocks device access).

- **Preview page is blank or shows errors**  
  Open the browser console (e.g. Developer Tools → Console). WebGL or shader compile errors will appear there. Ensure the project’s fragment shader is valid GLSL.

- **Port 8080 already in use**  
  Use another port: `./audiovisual -port 9090` and open **http://localhost:9090/**.
