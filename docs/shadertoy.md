# Using shaders from ShaderToy

You can use fragment shaders from [ShaderToy](https://www.shadertoy.com) in audiovisual by **importing** them (API or paste) or by copying the code and adapting it manually.

## Import (recommended)

Use the **import-shadertoy** subcommand to create a project that runs with variables (gain, palette_mix, pulse) like the bundled template.

### From ShaderToy API (shader must be Public + API)

Get an API key from [ShaderToy → Profile → Config → Apps](https://www.shadertoy.com).

```bash
# By view ID or full URL
audiovisual import-shadertoy -id 7cfGzn -key YOUR_API_KEY -output my-shader.avproj
audiovisual import-shadertoy -id https://www.shadertoy.com/view/7cfGzn -key YOUR_API_KEY -output my-shader.avproj

# Or set the key once
export SHADERTOY_API_KEY=YOUR_API_KEY
audiovisual import-shadertoy -id 7cfGzn -output my-shader.avproj
```

### From pasted code

Copy the fragment code from the ShaderToy tab (Buffer A / Image), then:

```bash
audiovisual import-shadertoy -stdin -output my-shader.avproj < paste.glsl
# or paste in terminal and Ctrl-D
audiovisual import-shadertoy -stdin -output my-shader.avproj
```

### Import options

- **-output** (required): output project path (e.g. `my-shader.avproj`).
- **-compat**: apply WebGL1 compatibility fixes (e.g. `tanh` → `myTanh`). Use if the shader fails to compile in the browser.
- **-name**: set project name (default: from API or "Imported Shader").

The generated project includes **u_resolution**, **u_time**, and params **gain** (waveform), **palette_mix** (audio band 3), **pulse** (audio band 1). Run it with `./audiovisual -project my-shader.avproj -port 8080` and open http://localhost:8080; allow the microphone so the shader reacts to sound.

## Manual paste (without import)

### Steps

1. **Open the ShaderToy link** in a browser and copy the fragment code from the main (Buffer A or Image) tab.

2. **ShaderToy uses different names** than our app. Replace them in the pasted code:

   | ShaderToy   | Audiovisual   |
   |------------|----------------|
   | `iResolution` | `u_resolution` (we pass `vec2(width, height)`; use `.xy` or `.x`/`.y` as needed) |
   | `iTime`       | `u_time`       |
   | `iTimeDelta`  | (omit or use a constant) |
   | `iFrame`      | (omit or use a constant) |
   | `iMouse`      | (omit or add as a custom param later) |

3. **Entry point:** ShaderToy uses `mainImage(out vec4 fragColor, in vec2 fragCoord)`. Wrap it in `main()` and call it:

   ```glsl
   void main() {
       mainImage(gl_FragColor, gl_FragCoord.xy);
   }
   ```

   Keep the rest of the ShaderToy code (including `mainImage`) as-is.

4. **Add a precision** at the top if missing:

   ```glsl
   precision mediump float;
   ```

5. **Put the result** in your project file under `shader.fragment_source` (as a single string; escape newlines as `\n` if needed), or use **import-shadertoy -stdin** to generate a full project (see Import above).

## Optional: ShaderToy API

To fetch shader source programmatically you need a [ShaderToy API key](https://www.shadertoy.com) (Profile → Config → your Apps). The shader must be **Public + API**. Endpoint:

```text
https://www.shadertoy.com/api/v1/shaders/7cfGzn?key=YOUR_KEY
```

The response includes the fragment code in the renderpass; you still need to apply the replacements above for use in audiovisual, or use **import-shadertoy -id &lt;view_id&gt; -key &lt;key&gt; -output out.avproj** to fetch and convert in one step.

## Example project

See **examples/shadertoy-template.avproj** for a project that includes a ShaderToy-style wrapper and a placeholder you can replace with code from a link like `view/7cfGzn`.

**To run the bundled Abstract Shine example:** start the app from the **repository root** so the project path resolves, then open the preview URL:

```bash
cd /path/to/audiovisual
./audiovisual -project examples/shadertoy-template.avproj -port 8080
```

Open http://localhost:8080/ in a browser. If you still see the default gradient, the project file was not found (check the path and cwd) or the shader failed to compile (open the browser DevTools console for GLSL errors).
