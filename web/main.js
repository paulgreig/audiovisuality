(() => {
  const canvas = document.getElementById("glcanvas");
  const gl =
    canvas.getContext("webgl2") ||
    canvas.getContext("webgl") ||
    canvas.getContext("experimental-webgl");

  if (!gl) {
    console.error("WebGL not supported");
    return;
  }

  function resizeCanvas() {
    const displayWidth = canvas.clientWidth;
    const displayHeight = canvas.clientHeight;
    if (canvas.width !== displayWidth || canvas.height !== displayHeight) {
      canvas.width = displayWidth;
      canvas.height = displayHeight;
      gl.viewport(0, 0, canvas.width, canvas.height);
    }
  }

  window.addEventListener("resize", resizeCanvas);
  resizeCanvas();

  const vertexSource = `
    attribute vec2 a_position;
    void main() {
      gl_Position = vec4(a_position, 0.0, 1.0);
    }
  `;

  const defaultFragmentSource = `
    precision mediump float;
    uniform vec2 u_resolution;
    uniform float u_time;
    uniform float u_gain;
    void main() {
      vec2 st = gl_FragCoord.xy / u_resolution.xy;
      float intensity = 0.5 + 0.5 * u_gain;
      vec3 color = vec3(st.x * intensity, st.y * intensity, 0.5 + 0.5 * sin(u_time));
      gl_FragColor = vec4(color, 1.0);
    }
  `;

  // Browser-side audio: drive any param with mapping.mode === "audio" from FFT
  let paramMeta = {}; // id -> { min, max, default, mapping }
  let browserAudioSmoothed = {}; // id -> smoothed 0..1 value
  let audioAnalyser = null;
  let audioDataArray = null;
  const NUM_FFT_BANDS = 8;

  function createShader(type, source) {
    const shader = gl.createShader(type);
    gl.shaderSource(shader, source);
    gl.compileShader(shader);
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
      console.error(gl.getShaderInfoLog(shader));
      gl.deleteShader(shader);
      return null;
    }
    return shader;
  }

  let program = null;
  let positionLocation = -1;
  let resolutionLocation = null;
  let timeLocation = null;
  let paramIds = [];
  let uniformLocations = {};
  let paramValues = {};

  let backgroundTexture = null;
  let hasBackgroundImage = false;
  let backgroundTextureUnit = 0;
  let dummyBackgroundTexture = null;

  const quadBuffer = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, quadBuffer);
  const vertices = new Float32Array([
    -1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1,
  ]);
  gl.bufferData(gl.ARRAY_BUFFER, vertices, gl.STATIC_DRAW);

  function initProgram(fragmentSource) {
    const vertexShader = createShader(gl.VERTEX_SHADER, vertexSource);
    const fragmentShader = createShader(gl.FRAGMENT_SHADER, fragmentSource);
    if (!vertexShader || !fragmentShader) return false;

    program = gl.createProgram();
    gl.attachShader(program, vertexShader);
    gl.attachShader(program, fragmentShader);
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      console.error(gl.getProgramInfoLog(program));
      return false;
    }

    positionLocation = gl.getAttribLocation(program, "a_position");
    resolutionLocation = gl.getUniformLocation(program, "u_resolution");
    timeLocation = gl.getUniformLocation(program, "u_time");
    uniformLocations = {};
    for (const id of paramIds) {
      const loc = gl.getUniformLocation(program, "u_" + id);
      if (loc) uniformLocations[id] = loc;
    }
    const locBg = gl.getUniformLocation(program, "u_background");
    const locHasBg = gl.getUniformLocation(program, "u_has_background");
    if (locBg) uniformLocations["__u_background"] = locBg;
    if (locHasBg) uniformLocations["__u_has_background"] = locHasBg;
    return true;
  }

  function createDummyBackgroundTexture() {
    if (dummyBackgroundTexture) return dummyBackgroundTexture;
    const tex = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([255, 255, 255, 255]));
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.bindTexture(gl.TEXTURE_2D, null);
    dummyBackgroundTexture = tex;
    return tex;
  }

  function loadImageTexture(url, callback) {
    const img = new Image();
    img.crossOrigin = "anonymous";
    img.onload = function () {
      const tex = gl.createTexture();
      gl.bindTexture(gl.TEXTURE_2D, tex);
      const canvas = document.createElement("canvas");
      canvas.width = img.naturalWidth;
      canvas.height = img.naturalHeight;
      const ctx = canvas.getContext("2d");
      ctx.translate(0, canvas.height);
      ctx.scale(1, -1);
      ctx.drawImage(img, 0, 0);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, canvas);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
      gl.bindTexture(gl.TEXTURE_2D, null);
      callback(tex);
    };
    img.onerror = function () {
      console.warn("Failed to load background image texture:", url);
      callback(null);
    };
    img.src = url;
  }

  function render(timeMs) {
    resizeCanvas();
    if (!program) {
      requestAnimationFrame(render);
      return;
    }

    gl.clearColor(0, 0, 0, 1);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.useProgram(program);

    gl.enableVertexAttribArray(positionLocation);
    gl.bindBuffer(gl.ARRAY_BUFFER, quadBuffer);
    gl.vertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 0, 0);

    gl.uniform2f(resolutionLocation, canvas.width, canvas.height);
    gl.uniform1f(timeLocation, timeMs * 0.001);

    // Update any params with mapping.mode === "audio" from browser mic FFT
    if (audioAnalyser && audioDataArray) {
      audioAnalyser.getByteFrequencyData(audioDataArray);
      const bands = NUM_FFT_BANDS;
      const binsPerBand = Math.floor(audioDataArray.length / bands) || 1;

      const overrides = {};
      for (const id of paramIds) {
        const meta = paramMeta[id];
        if (!meta || !meta.mapping || meta.mapping.mode !== "audio") continue;

        const bandIndex = Math.max(0, Math.min((meta.mapping.audio_band ?? 0), bands - 1));
        let start = bandIndex * binsPerBand;
        let end = Math.min(start + binsPerBand, audioDataArray.length);
        let sum = 0;
        for (let i = start; i < end; i++) sum += audioDataArray[i];
        let avg = end > start ? sum / (end - start) : 0;
        const level = Math.min(1, avg / 128);

        let smoothed = browserAudioSmoothed[id] ?? level;
        smoothed = 0.8 * smoothed + 0.2 * level;
        if (!Number.isFinite(smoothed)) smoothed = 0;
        smoothed = Math.max(0, Math.min(1, smoothed));
        browserAudioSmoothed[id] = smoothed;

        const min = typeof meta.min === "number" ? meta.min : 0;
        const max = typeof meta.max === "number" ? meta.max : 1;
        const value = min + smoothed * (max - min);
        paramValues[id] = value;
        overrides[id] = value;
      }

      // Expose overrides for the client-params sender (see setInterval below)
      window.__browserAudioOverrides = overrides;
    }

    for (const id of paramIds) {
      const loc = uniformLocations[id];
      if (loc != null && typeof paramValues[id] === "number") {
        gl.uniform1f(loc, paramValues[id]);
      }
    }

    const locBg = uniformLocations["__u_background"];
    const locHasBg = uniformLocations["__u_has_background"];
    if (locBg != null && locHasBg != null) {
      gl.activeTexture(gl.TEXTURE0 + backgroundTextureUnit);
      gl.bindTexture(gl.TEXTURE_2D, hasBackgroundImage && backgroundTexture ? backgroundTexture : createDummyBackgroundTexture());
      gl.uniform1i(locBg, backgroundTextureUnit);
      gl.uniform1f(locHasBg, hasBackgroundImage ? 1.0 : 0.0);
    }

    gl.drawArrays(gl.TRIANGLES, 0, 6);
    requestAnimationFrame(render);
  }

  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const wsUrl = `${protocol}://${window.location.host}/ws/params`;
  const ws = new WebSocket(wsUrl);
  ws.addEventListener("message", (event) => {
    try {
      const data = JSON.parse(event.data);
      if (data && data.params && typeof data.params === "object") {
        for (const [id, value] of Object.entries(data.params)) {
          if (typeof value === "number") paramValues[id] = value;
        }
      }
    } catch (err) {
      console.error("failed to parse params message", err);
    }
  });

  fetch("/api/shader")
    .then((r) => r.json())
    .then((data) => {
      paramIds = (data.params || []).map((p) => p.id).filter(Boolean);
      (data.params || []).forEach((p) => {
        if (p.id) {
          paramMeta[p.id] = {
            min: p.min,
            max: p.max,
            default: p.default,
            mapping: p.mapping || null,
          };
          if (typeof paramValues[p.id] !== "number") {
            paramValues[p.id] = typeof p.default === "number" ? p.default : 0;
          }
        }
      });
      const overlayUrl = data.overlay_url && data.overlay_url.trim();
      hasBackgroundImage = false;
      backgroundTexture = null;
      if (overlayUrl) {
        loadImageTexture(overlayUrl, function (tex) {
          backgroundTexture = tex;
          hasBackgroundImage = tex != null;
        });
      }
      const fragmentSource = data.fragment_source && data.fragment_source.trim()
        ? data.fragment_source
        : defaultFragmentSource;
      const useDefault = !fragmentSource.trim() ||
        (!fragmentSource.includes("gl_FragColor") && !fragmentSource.includes("fragColor"));
      const finalSource = useDefault ? defaultFragmentSource : fragmentSource;
      if (initProgram(finalSource)) {
        console.log("Shader loaded, params:", paramIds);
      } else {
        console.error("Shader compile/link failed, using default. Check browser console for GLSL errors.");
        paramIds = ["gain"];
        paramValues.gain = 0;
        initProgram(defaultFragmentSource);
      }
    })
    .catch((err) => {
      console.warn("Fetch /api/shader failed, using default shader", err);
      paramIds = ["gain"];
      paramValues.gain = 0;
      initProgram(defaultFragmentSource);
    });

  // Start browser mic capture; any param with mapping.mode === "audio" is driven by FFT.
  if (navigator.mediaDevices && navigator.mediaDevices.getUserMedia) {
    console.log("Requesting browser mic access for audio-mapped params...");
    navigator.mediaDevices
      .getUserMedia({ audio: true })
      .then((stream) => {
        const AudioContextCtor =
          window.AudioContext || window.webkitAudioContext;
        const audioCtx = new AudioContextCtor();
        const source = audioCtx.createMediaStreamSource(stream);
        const analyser = audioCtx.createAnalyser();
        analyser.fftSize = 1024;
        const bufferLength = analyser.frequencyBinCount;
        const dataArray = new Uint8Array(bufferLength);
        source.connect(analyser);
        audioAnalyser = analyser;
        audioDataArray = dataArray;
        console.log("Browser audio capture started for param mapping (mode=audio)");

        // Open a WebSocket to send client-side param overrides back to the server.
        const clientWsProto =
          window.location.protocol === "https:" ? "wss" : "ws";
        const clientWsUrl = `${clientWsProto}://${window.location.host}/ws/client-params`;
        const clientWs = new WebSocket(clientWsUrl);
        clientWs.addEventListener("open", () => {
          console.log("client-params websocket connected");
        });
        clientWs.addEventListener("error", (err) => {
          console.warn("client-params websocket error", err);
        });

        // Send all browser-audio-driven param overrides to the server.
        setInterval(() => {
          if (clientWs.readyState !== WebSocket.OPEN) return;
          const overrides = window.__browserAudioOverrides;
          if (overrides && typeof overrides === "object" && Object.keys(overrides).length > 0) {
            try {
              clientWs.send(JSON.stringify({ params: overrides }));
            } catch (err) {
              console.warn("failed to send client params", err);
            }
          }
        }, 100);
      })
      .catch((err) => {
        console.warn("Browser audio capture failed", err);
      });
  } else {
    console.warn("getUserMedia not supported; browser audio capture disabled");
  }

  requestAnimationFrame(render);
})();
