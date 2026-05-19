const state = {
  apiURL: defaultAPIURL(),
  events: null,
  channels: [],
  nodes: [],
  environment: [],
  telemetry: {
    device: [],
    power: [],
    airquality: [],
    localstats: [],
    health: [],
    history: {
      environment: [],
      device: [],
      localstats: [],
    },
  },
  messages: [],
  selectedChannel: "Primary",
  map: null,
  markers: new Map(),
  overlayLayer: null,
  traceLayer: null,
  overlay: {
    type: "none",
    cellKm: 5,
  },
  tracesVisible: true,
  traceRoutes: [],
  mapNodeQuery: "",
  mapSeenMinutes: 0,
  debugLog: [],
  traceToastSeen: new Map(),
  mapFitDone: false,
  reconnectTimer: null,
  tracing: false,
  traceCooldownUntil: 0,
  topology: {
    panX: 0,
    panY: 0,
    zoom: 1,
    selectedNode: "",
    selectedLink: "",
    dragging: false,
    dragX: 0,
    dragY: 0,
  },
};
const TRACE_STORAGE_KEY = "gomeshin.traceRoutes.v1";
const TRACE_STORAGE_LIMIT = 12;

const els = {
  status: document.querySelector("#status"),
  connectForm: document.querySelector("#connect-form"),
  apiURL: document.querySelector("#api-url"),
  channelSelect: document.querySelector("#channel-select"),
  messages: document.querySelector("#messages"),
  messageCount: document.querySelector("#message-count"),
  sendForm: document.querySelector("#send-form"),
  messageText: document.querySelector("#message-text"),
  channels: document.querySelector("#channels"),
  map: document.querySelector("#map"),
  mapSummary: document.querySelector("#map-summary"),
  fitMap: document.querySelector("#fit-map"),
  overlayType: document.querySelector("#overlay-type"),
  overlayCellKm: document.querySelector("#overlay-cell-km"),
  mapTraces: document.querySelector("#map-traces"),
  mapNodeSearch: document.querySelector("#map-node-search"),
  mapSeenMinutes: document.querySelector("#map-seen-minutes"),
  clearTraces: document.querySelector("#clear-traces"),
  weather: document.querySelector("#weather"),
  weatherSummary: document.querySelector("#weather-summary"),
  refreshWeather: document.querySelector("#refresh-weather"),
  telemetry: document.querySelector("#telemetry"),
  telemetrySummary: document.querySelector("#telemetry-summary"),
  refreshTelemetry: document.querySelector("#refresh-telemetry"),
  settingsSummary: document.querySelector("#settings-summary"),
  refreshSettings: document.querySelector("#refresh-settings"),
  settingsForm: document.querySelector("#settings-form"),
  settingsHopLimit: document.querySelector("#settings-hop-limit"),
  settingsRole: document.querySelector("#settings-role"),
  settingsTxEnabled: document.querySelector("#settings-tx-enabled"),
  locationForm: document.querySelector("#location-form"),
  settingsLatitude: document.querySelector("#settings-latitude"),
  settingsLongitude: document.querySelector("#settings-longitude"),
  settingsAltitude: document.querySelector("#settings-altitude"),
  clearLocation: document.querySelector("#clear-location"),
  topologyGraph: document.querySelector("#topology-graph"),
  topologySummary: document.querySelector("#topology-summary"),
  topologyDetails: document.querySelector("#topology-details"),
  refreshTopology: document.querySelector("#refresh-topology"),
  nodes: document.querySelector("#nodes"),
  nodeSearch: document.querySelector("#node-search"),
  debugLog: document.querySelector("#debug-log"),
  clearDebugLog: document.querySelector("#clear-debug-log"),
  checkPendingTraces: document.querySelector("#check-pending-traces"),
  refreshChannels: document.querySelector("#refresh-channels"),
  refreshNodes: document.querySelector("#refresh-nodes"),
};

els.apiURL.value = state.apiURL;
restoreTraceRoutes();

document.querySelectorAll(".tab").forEach((button) => {
  button.addEventListener("click", () => activateTab(button.dataset.tab));
});

els.connectForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  state.apiURL = els.apiURL.value.replace(/\/+$/, "");
  await connect();
});

els.channelSelect.addEventListener("change", () => {
  state.selectedChannel = els.channelSelect.value;
  renderMessages();
});

els.sendForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const text = els.messageText.value.trim();
  if (!text) return;

  try {
    await request("/messages", {
      method: "POST",
      body: {
        channel: state.selectedChannel,
        text,
      },
    });
    els.messageText.value = "";
    setStatus("Sent", "connected");
  } catch (error) {
    setStatus(error.message, "error");
  }
});

els.refreshChannels.addEventListener("click", loadChannels);
els.refreshNodes.addEventListener("click", loadNodes);
els.refreshWeather.addEventListener("click", loadWeather);
els.refreshTelemetry.addEventListener("click", loadTelemetry);
els.refreshTopology.addEventListener("click", renderTopology);
els.refreshSettings.addEventListener("click", loadSettings);
els.settingsForm.addEventListener("submit", saveSettings);
els.locationForm.addEventListener("submit", saveLocation);
els.clearLocation.addEventListener("click", clearLocation);
els.nodeSearch.addEventListener("input", renderNodes);
els.clearDebugLog.addEventListener("click", () => clearDebugLog());
els.checkPendingTraces.addEventListener("click", async () => {
  try {
    const payload = await request("/traceroutes/pending");
    const pending = Array.isArray(payload) ? payload : [];
    if (!pending.length) {
      addDebugLog("Pending traces: none");
      setStatus("Pending traces: none", "connected");
      showToast("No pending traceroutes", "info");
      return;
    }
    addDebugLog(`Pending traces: ${pending.length}`);
    for (const item of pending) {
      const id = item.requestID ?? item.RequestID ?? 0;
      const to = item.to ?? item.To ?? 0;
      const ch = item.channel ?? item.Channel ?? "";
      const hop = item.hopLimit ?? item.HopLimit ?? 0;
      addDebugLog(`Pending trace id=${Number(id).toString(16).padStart(8, "0")} to=${formatNodeID(to)} channel=${ch || "Primary"} hop=${hop}`);
    }
    setStatus(`Pending traces: ${pending.length}`, "connected");
    showToast(`Pending traceroutes: ${pending.length}`, "ok");
  } catch (error) {
    addDebugLog(`Pending trace check failed: ${error.message}`);
    setStatus(`Pending trace check failed: ${error.message}`, "error");
    showToast("Pending traceroute check failed", "error");
  }
});
els.fitMap.addEventListener("click", () => fitMapToMarkers(true));
els.overlayType.addEventListener("change", () => {
  state.overlay.type = els.overlayType.value;
  renderMap();
});
els.overlayCellKm.addEventListener("change", () => {
  state.overlay.cellKm = Number.parseFloat(els.overlayCellKm.value) || 5;
  renderMap();
});
els.mapTraces.addEventListener("change", () => {
  state.tracesVisible = els.mapTraces.value !== "off";
  renderMap();
});
els.mapNodeSearch.addEventListener("input", () => {
  state.mapNodeQuery = els.mapNodeSearch.value.trim().toLowerCase();
  renderMap();
});
els.mapSeenMinutes.addEventListener("input", () => {
  const value = Number.parseInt(els.mapSeenMinutes.value, 10);
  state.mapSeenMinutes = Number.isFinite(value) && value > 0 ? value : 0;
  renderMap();
});
els.clearTraces.addEventListener("click", () => {
  state.traceRoutes = [];
  persistTraceRoutes();
  renderMap();
  renderTopology();
});
window.addEventListener("resize", () => {
  if (isTopologyTabActive()) {
    renderTopology();
  }
});

async function connect() {
  closeEvents();
  setStatus("Connecting...", "");

  try {
    await request("/health");
    await Promise.all([loadChannels(), loadNodes(), loadMessages(), loadWeather(), loadTelemetry(), loadTraceRoutes(), loadSettings()]);
    openEvents();
    setStatus(`Connected to ${state.apiURL}`, "connected");
  } catch (error) {
    setStatus(error.message, "error");
  }
}

async function loadSettings() {
  try {
    const settings = await request("/settings/radio");
    els.settingsHopLimit.value = `${settings.hopLimit ?? settings.HopLimit ?? ""}`;
    els.settingsRole.value = settings.role ?? settings.Role ?? "CLIENT";
    els.settingsTxEnabled.checked = Boolean(settings.txEnabled ?? settings.TxEnabled);
    const region = settings.region ?? settings.Region ?? "UNKNOWN";
    const preset = settings.modemPreset ?? settings.ModemPreset ?? "UNKNOWN";
    els.settingsSummary.textContent = `${region} • ${preset}`;
  } catch (error) {
    els.settingsSummary.textContent = `Load failed: ${error.message}`;
    throw error
  }
}

async function saveSettings(event) {
  event.preventDefault();
  const hop = Number.parseInt(els.settingsHopLimit.value, 10);
  const role = String(els.settingsRole.value || "CLIENT");
  const txEnabled = Boolean(els.settingsTxEnabled.checked);
  if (!Number.isFinite(hop) || hop < 1 || hop > 7) {
    setStatus("Hop limit must be between 1 and 7", "error");
    return;
  }
  try {
    const settings = await request("/settings/radio", {
      method: "POST",
      body: {
        hopLimit: hop,
        role,
        txEnabled,
      },
    });
    const region = settings.region ?? settings.Region ?? "UNKNOWN";
    const preset = settings.modemPreset ?? settings.ModemPreset ?? "UNKNOWN";
    els.settingsSummary.textContent = `${region} • ${preset}`;
    setStatus("Radio settings updated", "connected");
    showToast("Radio settings applied", "ok");
  } catch (error) {
    setStatus(`Settings apply failed: ${error.message}`, "error");
    showToast("Radio settings apply failed", "error");
  }
}

async function saveLocation(event) {
  event.preventDefault();
  const latitude = Number.parseFloat(els.settingsLatitude.value);
  const longitude = Number.parseFloat(els.settingsLongitude.value);
  const altitudeRaw = els.settingsAltitude.value.trim();
  const altitude = altitudeRaw === "" ? null : Number.parseInt(altitudeRaw, 10);

  if (!Number.isFinite(latitude) || latitude < -90 || latitude > 90) {
    setStatus("Latitude must be between -90 and 90", "error");
    return;
  }
  if (!Number.isFinite(longitude) || longitude < -180 || longitude > 180) {
    setStatus("Longitude must be between -180 and 180", "error");
    return;
  }
  if (altitudeRaw !== "" && !Number.isFinite(altitude)) {
    setStatus("Altitude must be a whole number in meters", "error");
    return;
  }

  try {
    await request("/settings/location", {
      method: "POST",
      body: {
        latitude,
        longitude,
        altitude: altitudeRaw === "" ? undefined : altitude,
      },
    });
    setStatus("Fixed location applied", "connected");
    showToast("Fixed location applied", "ok");
  } catch (error) {
    setStatus(`Set location failed: ${error.message}`, "error");
    showToast("Set location failed", "error");
  }
}

async function clearLocation() {
  try {
    await request("/settings/location", { method: "DELETE" });
    setStatus("Fixed location cleared", "connected");
    showToast("Fixed location cleared", "ok");
  } catch (error) {
    setStatus(`Clear location failed: ${error.message}`, "error");
    showToast("Clear location failed", "error");
  }
}

async function loadTraceRoutes() {
  const traces = await requestSafeArray("/traceroutes?limit=200");
  state.traceRoutes = traces.filter((route) => route && typeof route === "object").slice(0, TRACE_STORAGE_LIMIT);
  persistTraceRoutes();
  renderMap();
  renderTopology();
}

async function loadChannels() {
	state.channels = activeChannels(await request("/channels"));
	if (!state.channels.some((channel) => displayChannel(channelName(channel)) === state.selectedChannel)) {
		state.selectedChannel = displayChannel(channelName(state.channels[0]) || "Primary");
	}
	renderChannels();
}

async function loadNodes() {
  const payload = await request("/nodes");
  state.nodes = Array.isArray(payload) ? payload : [];
  renderNodes();
  renderWeather();
  renderMap();
}

async function loadWeather() {
  const payload = await request("/telemetry/environment");
  state.environment = Array.isArray(payload) ? payload : [];
  for (const environment of state.environment) {
    updateNodeEnvironment(environment);
  }
  renderWeather();
  renderNodes();
  renderMap();
}

async function loadMessages() {
  const payload = await request("/messages");
  state.messages = Array.isArray(payload) ? payload : [];
  renderMessages();
}

async function loadTelemetry() {
  const [device, power, airquality, localstats, health, envHist, devHist, localHist] = await Promise.all([
    requestSafeArray("/telemetry/device"),
    requestSafeArray("/telemetry/power"),
    requestSafeArray("/telemetry/airquality"),
    requestSafeArray("/telemetry/localstats"),
    requestSafeArray("/telemetry/health"),
    requestSafeArray("/telemetry/environment/history?limit=1000"),
    requestSafeArray("/telemetry/device/history?limit=1000"),
    requestSafeArray("/telemetry/localstats/history?limit=1000"),
  ]);
  state.telemetry.device = device;
  state.telemetry.power = power;
  state.telemetry.airquality = airquality;
  state.telemetry.localstats = localstats;
  state.telemetry.health = health;
  state.telemetry.history.environment = envHist;
  state.telemetry.history.device = devHist;
  state.telemetry.history.localstats = localHist;
  renderTelemetry();
}

function openEvents() {
  if (state.reconnectTimer) {
    clearTimeout(state.reconnectTimer);
    state.reconnectTimer = null;
  }
  const events = new EventSource(`${state.apiURL}/events`);
  state.events = events;
  events.onopen = () => {
    setStatus(`Connected to ${state.apiURL}`, "connected");
    addDebugLog("SSE open /events");
  };

  events.addEventListener("message.received", (event) => {
    addDebugLog("SSE message.received");
    const envelope = JSON.parse(event.data);
    const message = envelope.data;
    const duplicate = state.messages.some((item) =>
      (item.id ?? item.ID ?? 0) === (message.id ?? message.ID ?? 0) &&
      nodeNum(messageFrom(item)) === nodeNum(messageFrom(message)) &&
      messageText(item) === messageText(message),
    );
    if (!duplicate) {
      state.messages.push(message);
    }
    if (state.messages.length > 500) {
      state.messages = state.messages.slice(-500);
    }
    renderMessages();
  });

  events.addEventListener("position.updated", (event) => {
    addDebugLog("SSE position.updated");
    const envelope = JSON.parse(event.data);
    updateNodePosition(envelope.data);
    renderNodes();
    renderMap();
  });

  events.addEventListener("environment.updated", (event) => {
    addDebugLog("SSE environment.updated");
    const envelope = JSON.parse(event.data);
    upsertEnvironment(envelope.data);
    updateNodeEnvironment(envelope.data);
    renderWeather();
    renderNodes();
    renderMap();
  });
  events.addEventListener("device.updated", (event) => {
    addDebugLog("SSE device.updated");
    const envelope = JSON.parse(event.data);
    upsertTelemetryRow("device", envelope.data);
    upsertTelemetryHistory("device", envelope.data);
    renderTelemetry();
    renderMap();
  });
  events.addEventListener("power.updated", (event) => {
    addDebugLog("SSE power.updated");
    const envelope = JSON.parse(event.data);
    upsertTelemetryRow("power", envelope.data);
    renderTelemetry();
    renderMap();
  });
  events.addEventListener("airquality.updated", (event) => {
    addDebugLog("SSE airquality.updated");
    const envelope = JSON.parse(event.data);
    upsertTelemetryRow("airquality", envelope.data);
    renderTelemetry();
    renderMap();
  });
  events.addEventListener("localstats.updated", (event) => {
    addDebugLog("SSE localstats.updated");
    const envelope = JSON.parse(event.data);
    upsertTelemetryRow("localstats", envelope.data);
    upsertTelemetryHistory("localstats", envelope.data);
    renderTelemetry();
    renderMap();
  });
  events.addEventListener("health.updated", (event) => {
    addDebugLog("SSE health.updated");
    const envelope = JSON.parse(event.data);
    upsertTelemetryRow("health", envelope.data);
    renderTelemetry();
    renderMap();
  });
  events.addEventListener("trace.updated", (event) => {
    addDebugLog("SSE trace.updated");
    const envelope = JSON.parse(event.data);
    rememberTrace(envelope.data);
    maybeToastTraceReceived(envelope.data);
    renderMap();
    renderTopology();
  });

  events.onerror = () => {
    setStatus("Event stream reconnecting...", "error");
    addDebugLog("SSE error/reconnect");
    closeEvents();
    if (!state.reconnectTimer) {
      state.reconnectTimer = setTimeout(() => {
        state.reconnectTimer = null;
        openEvents();
      }, 1500);
    }
  };
}

function closeEvents() {
  if (state.events) {
    state.events.close();
    state.events = null;
  }
}

function upsertTelemetryRow(kind, sample) {
  const list = state.telemetry[kind];
  if (!Array.isArray(list)) return;
  const key = environmentKey(sample);
  if (!key) return;
  const currentIndex = list.findIndex((item) => environmentKey(item) === key);
  if (currentIndex >= 0) {
    if (telemetryTime(sample) >= telemetryTime(list[currentIndex])) {
      list[currentIndex] = sample;
    }
  } else {
    list.unshift(sample);
  }
}

function upsertTelemetryHistory(kind, sample) {
  const map = {
    environment: "environment",
    device: "device",
    localstats: "localstats",
  };
  const targetKey = map[kind];
  if (!targetKey) return;
  const list = state.telemetry.history[targetKey];
  if (!Array.isArray(list)) return;
  list.unshift(sample);
  if (list.length > 2000) {
    list.length = 2000;
  }
}

async function request(path, options = {}) {
  const method = options.method || "GET";
  const init = {
    method,
    headers: {},
  };

  if (options.body) {
    init.headers["content-type"] = "application/json";
    init.body = JSON.stringify(options.body);
  }

  let response;
  const startedAt = performance.now();
  addDebugLog(`HTTP ${method} ${path}`);
  try {
    response = await fetch(`${state.apiURL}${path}`, init);
  } catch (error) {
    addDebugLog(`HTTP ${method} ${path} FAILED (network)`);
    throw new Error(`Cannot reach ${state.apiURL}. Check that gomeshind is running and CORS allows this page.`);
  }
  const elapsed = Math.round(performance.now() - startedAt);
  addDebugLog(`HTTP ${method} ${path} -> ${response.status} (${elapsed}ms)`);
  if (!response.ok) {
    let message = response.statusText;
    try {
      const payload = await response.json();
      message = payload.error || message;
    } catch {
      // Keep the HTTP status text when the response is not JSON.
    }
    throw new Error(message);
  }
  if (response.status === 204) {
    return null;
  }
  return response.json();
}

function renderChannels() {
  els.channelSelect.replaceChildren();
  els.channels.replaceChildren();

	for (const channel of state.channels) {
		const name = displayChannel(channelName(channel));
		const option = document.createElement("option");
		option.value = name;
		option.textContent = name;
    option.selected = name === state.selectedChannel;
    els.channelSelect.append(option);

		const item = document.createElement("li");
		item.innerHTML = `<span class="primary">[${channelIndex(channel)}]</span> ${escapeHTML(channelRole(channel))} <span class="muted">${escapeHTML(name)}</span>`;
		els.channels.append(item);
	}
}

function renderNodes() {
  const query = els.nodeSearch.value.trim().toLowerCase();
  const nodes = state.nodes.filter((node) => nodeMatches(node, query));

  els.nodes.replaceChildren();
	for (const node of nodes) {
		const label = formatNode(node);

    const item = document.createElement("li");
    item.textContent = label;
    els.nodes.append(item);
  }
}

function renderMap() {
  const allPositions = state.nodes
    .map((node) => ({ node, position: nodePosition(node) }))
    .filter(({ position }) => hasLatLon(position));
  const query = (state.mapNodeQuery || "").trim().toLowerCase();
  const seenMinutes = Number.isFinite(Number(state.mapSeenMinutes)) && Number(state.mapSeenMinutes) > 0
    ? Number(state.mapSeenMinutes)
    : 0;

  let positions = allPositions.filter(({ node }) => {
    if (!nodeMatches(node, query)) return false;
    return nodeMatchesSeenWindow(node, seenMinutes);
  });
  if (positions.length === 0 && !query && seenMinutes <= 0) {
    positions = allPositions;
  }

  const overlayText = state.overlay.type === "none" ? "no overlay" : `${overlayLabel(state.overlay.type)} @ ${state.overlay.cellKm}km radius`;
  const seenText = seenMinutes > 0 ? ` • seen<=${seenMinutes}m` : "";
  els.mapSummary.textContent = `${positions.length}/${allPositions.length} positions${seenText} • ${overlayText}`;

  if (!isMapTabActive()) {
    return;
  }

  if (positions.length === 0) {
    if (state.map) {
      clearMapMarkers();
      if (state.overlayLayer) state.overlayLayer.clearLayers();
      if (state.traceLayer) state.traceLayer.clearLayers();
      if (allPositions.length === 0) {
        els.mapSummary.textContent = `0/${allPositions.length} positions • ${overlayText}`;
      } else {
        els.mapSummary.textContent = `0/${allPositions.length} positions${seenText} • ${overlayText}`;
      }
      setTimeout(() => state.map.invalidateSize(), 0);
      return;
    }
    const empty = document.createElement("div");
    empty.className = "map-empty";
    empty.textContent = allPositions.length === 0 ? "No node positions yet." : "No map nodes match your filters.";
    els.map.replaceChildren();
    els.map.append(empty);
    return;
  }

  ensureMap();
  syncMapMarkers(positions);
}

function updateNodePosition(position) {
  const num = nodeNum(position.node ?? position.Node ?? {});
  if (!num) return;

  const existing = state.nodes.find((node) => nodeNum(node) === num);
  if (existing) {
    existing.Position = position;
    existing.position = position;
    return;
  }

  const node = position.node ?? position.Node ?? {};
  state.nodes.push({
    Num: num,
    ID: node.ID ?? node.id ?? "",
    LongName: node.LongName ?? node.longName ?? "",
    ShortName: node.ShortName ?? node.shortName ?? "",
    Position: position,
  });
}

function upsertEnvironment(environment) {
  const key = environmentKey(environment);
  if (!key) return;

  const eventTime = environmentTime(environment);
  const existingIndex = state.environment.findIndex((item) => {
    return environmentKey(item) === key && environmentTime(item) === eventTime;
  });
  if (existingIndex >= 0) {
    state.environment[existingIndex] = environment;
    return;
  }
  state.environment.unshift(environment);
}

function updateNodeEnvironment(environment) {
  const node = environmentNode(environment);
  const num = nodeNum(node);
  const id = nodeID(node);
  if (!num && !id) return;

  const existing = state.nodes.find((candidate) => {
    const candidateNum = nodeNum(candidate);
    if (num && candidateNum) {
      return candidateNum === num;
    }
    return id && nodeID(candidate) === id;
  });
  if (existing) {
    const currentTime = environmentTime(existing.environment ?? existing.Environment);
    const incomingTime = environmentTime(environment);
    if (incomingTime >= currentTime) {
      existing.Environment = environment;
      existing.environment = environment;
    }
    return;
  }

  const parsedNum = num || parseNodeIDToNum(id);
  state.nodes.push({
    Num: parsedNum,
    ID: id,
    LongName: node.LongName ?? node.longName ?? "",
    ShortName: node.ShortName ?? node.shortName ?? "",
    Environment: environment,
  });
}

function ensureMap() {
  if (state.map) {
    return;
  }
  if (!window.L) {
    els.map.replaceChildren();
    const empty = document.createElement("div");
    empty.className = "map-empty";
    empty.textContent = "Map library failed to load.";
    els.map.append(empty);
    return;
  }

  els.map.replaceChildren();
  state.map = L.map(els.map, {
    zoomControl: true,
    attributionControl: true,
  }).setView([0, 0], 2);

  L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
    maxZoom: 19,
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
  }).addTo(state.map);
  state.overlayLayer = L.layerGroup().addTo(state.map);
  state.traceLayer = L.layerGroup().addTo(state.map);
  state.map.on("popupopen", onMapPopupOpen);
}

function syncMapMarkers(positions) {
  if (!state.map) return;

  const seen = new Set();
  const bounds = [];
  for (const { node, position } of positions) {
    const num = nodeNum(node);
    const key = String(num);
    const lat = Number(positionLatitude(position));
    const lon = Number(positionLongitude(position));
    if (!Number.isFinite(lat) || !Number.isFinite(lon)) {
      continue;
    }

    seen.add(key);
    bounds.push([lat, lon]);

    const label = nodeShortName(node) || nodeLongName(node) || formatNodeID(num);
    const popup = formatMapPopup(node, position);
    const ageClass = markerAgeClass(node);
    const existing = state.markers.get(key);
    if (existing) {
      existing.setLatLng([lat, lon]);
      existing.setIcon(nodeIcon(label, ageClass));
      existing.setPopupContent(popup);
    } else {
      state.markers.set(key, L.marker([lat, lon], { icon: nodeIcon(label, ageClass) }).bindPopup(popup).addTo(state.map));
    }
  }

  for (const [key, marker] of state.markers) {
    if (!seen.has(key)) {
      marker.remove();
      state.markers.delete(key);
    }
  }

  fitMapToMarkers(false);
  const overlayCount = renderMapOverlay(positions);
  const traceCount = renderTraceRoutes();
  setTimeout(() => state.map.invalidateSize(), 0);
  if (overlayCount > 0) {
    els.mapSummary.textContent += ` • ${overlayCount} samples`;
  }
  if (traceCount > 0) {
    els.mapSummary.textContent += ` • ${traceCount} trace paths`;
  }
}

function onMapPopupOpen(event) {
  const root = event.popup?.getElement?.();
  if (!root) return;
  const button = root.querySelector(".map-trace-btn");
  if (!button || button.dataset.bound === "1") return;
  button.dataset.bound = "1";
  button.addEventListener("click", async () => {
    const to = button.dataset.traceNode || "";
    if (!to) return;
    await runTraceroute(to);
  });
}

async function runTraceroute(to) {
  if (!to) return;
  const now = Date.now();
  if (now < state.traceCooldownUntil) {
    const waitSeconds = Math.max(1, Math.ceil((state.traceCooldownUntil - now) / 1000));
    const message = `Traceroute cooldown: wait ${waitSeconds}s`;
    addDebugLog(message);
    showToast(message, "info");
    return;
  }
  if (state.tracing) return;
  state.tracing = true;
  state.traceCooldownUntil = now + 30000;
  const timeoutSeconds = 90;
  addDebugLog(`Traceroute start -> ${to} channel=${state.selectedChannel || "Primary"}`);
  showToast(`Traceroute started: ${to}`, "info");
  try {
    const response = await request("/traceroute", {
      method: "POST",
      body: {
        to,
        channel: state.selectedChannel,
        timeoutSeconds,
      },
    });
    const pendingID = response.requestID ?? response.RequestID ?? 0;
    addDebugLog(`Traceroute queued -> ${to} request=${Number(pendingID).toString(16).padStart(8, "0")}`);
    showToast(`Traceroute queued: ${to}`, "ok");
  } catch (error) {
    addDebugLog(`Traceroute failed -> ${to} (${error.message})`);
    showToast(`Traceroute failed: ${error.message}`, "error");
  } finally {
    state.tracing = false;
  }
}

function fitMapToMarkers(force) {
  if (!state.map || state.markers.size === 0) return;
  const bounds = [...state.markers.values()].map((marker) => marker.getLatLng());
  if (bounds.length === 1) {
    if (force || state.map.getZoom() < 10) {
      state.map.setView(bounds[0], 13);
    }
    return;
  }
  if (force || !state.mapFitDone) {
    state.map.fitBounds(bounds, { padding: [38, 38], maxZoom: 15 });
    state.mapFitDone = true;
  }
}

function nodeIcon(label, ageClass = "mesh-marker--stale") {
  const initial = (label || "?").trim().slice(0, 1).toUpperCase() || "?";
  return L.divIcon({
    className: "",
    html: `<div class="mesh-marker ${escapeHTML(ageClass)}" title="${escapeHTML(label)}"><span>${escapeHTML(initial)}</span></div>`,
    iconSize: [26, 26],
    iconAnchor: [13, 13],
    popupAnchor: [0, -13],
  });
}

function markerAgeClass(node) {
  const heard = nodeLastHeard(node);
  const heardMs = Date.parse(heard || "");
  if (!Number.isFinite(heardMs)) return "mesh-marker--stale";
  const ageMinutes = (Date.now() - heardMs) / 60000;
  if (ageMinutes <= 30) return "mesh-marker--fresh";
  if (ageMinutes <= 60) return "mesh-marker--warn";
  return "mesh-marker--stale";
}

function nodeMatchesSeenWindow(node, minutes) {
  if (!minutes || minutes <= 0) return true;
  const heard = nodeLastHeard(node);
  const heardMs = Date.parse(heard || "");
  if (!Number.isFinite(heardMs)) return false;
  const ageMinutes = (Date.now() - heardMs) / 60000;
  return ageMinutes <= minutes;
}

function clearMapMarkers() {
  for (const marker of state.markers.values()) {
    marker.remove();
  }
  state.markers.clear();
}

function rememberTrace(route) {
  if (!route || typeof route !== "object") return;
  const incomingKey = traceKey(route);
  if (incomingKey) {
    const duplicate = state.traceRoutes.some((item) => traceKey(item) === incomingKey);
    if (duplicate) return;
  }
  state.traceRoutes.unshift(route);
  if (state.traceRoutes.length > TRACE_STORAGE_LIMIT) {
    state.traceRoutes.length = TRACE_STORAGE_LIMIT;
  }
  persistTraceRoutes();
}

function renderTopology() {
  if (!els.topologyGraph) return;
  const { nodes, links } = buildTopologyGraphData();
  els.topologySummary.textContent = `${nodes.length} nodes • ${links.length} links`;
  if (els.topologyDetails && !state.topology.selectedNode && !state.topology.selectedLink) {
    els.topologyDetails.textContent = "Click a node or link for details.";
  }

  if (!nodes.length || !links.length) {
    const empty = document.createElement("div");
    empty.className = "topology-empty";
    empty.textContent = "No traceroute links yet.";
    const details = els.topologyDetails;
    els.topologyGraph.replaceChildren(empty);
    if (details) els.topologyGraph.append(details);
    return;
  }

  const width = Math.max(300, Math.floor(els.topologyGraph.clientWidth || 900));
  const height = Math.max(260, Math.floor(els.topologyGraph.clientHeight || 500));
  runTopologyLayout(nodes, links, width, height);

  const svgNS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(svgNS, "svg");
  svg.setAttribute("class", "topology-svg");
  svg.setAttribute("viewBox", `0 0 ${width} ${height}`);
  svg.style.background = "radial-gradient(1200px 700px at 50% 35%, #121c23 0%, #0b1115 62%, #080d11 100%)";
  bindTopologyViewport(svg, width, height);

  const viewport = document.createElementNS(svgNS, "g");
  viewport.setAttribute("class", "topology-viewport");
  svg.append(viewport);

  for (const link of links) {
    const source = nodes[link.source];
    const target = nodes[link.target];
    if (!source || !target) continue;

    const line = document.createElementNS(svgNS, "line");
    line.setAttribute("x1", `${source.x}`);
    line.setAttribute("y1", `${source.y}`);
    line.setAttribute("x2", `${target.x}`);
    line.setAttribute("y2", `${target.y}`);
    line.setAttribute("class", `topology-link ${isSelectedLink(link) ? "selected" : ""}`);
    line.setAttribute("stroke-width", `${1 + Math.min(3.5, link.count * 0.85)}`);
    line.setAttribute("opacity", `${Math.min(0.85, 0.35 + link.count * 0.08)}`);
    line.addEventListener("click", (event) => {
      event.stopPropagation();
      state.topology.selectedNode = "";
      state.topology.selectedLink = link.key;
      if (els.topologyDetails) {
        els.topologyDetails.textContent = `${source.label} ↔ ${target.label} | traces: ${link.count} | avg link SNR: ${formatSNRValue(link.avgSNR)} dB`;
      }
      renderTopology();
    });
    viewport.append(line);
  }

  const topLabelCount = Math.min(nodes.length, 34);
  const labelThreshold = [...nodes].sort((a, b) => b.degree - a.degree)[Math.max(0, topLabelCount - 1)]?.degree ?? 0;

  for (const node of nodes) {
    const circle = document.createElementNS(svgNS, "circle");
    circle.setAttribute("cx", `${node.x}`);
    circle.setAttribute("cy", `${node.y}`);
    circle.setAttribute("r", `${4.5 + Math.min(8, node.degree * 0.9)}`);
    circle.setAttribute("class", `topology-node ${isSelectedNode(node) ? "selected" : ""}`);
    circle.addEventListener("click", (event) => {
      event.stopPropagation();
      state.topology.selectedLink = "";
      state.topology.selectedNode = node.key;
      if (els.topologyDetails) {
        els.topologyDetails.textContent = `${node.label} | links: ${node.degree} | node: ${formatNodeID(node.num)}`;
      }
      renderTopology();
    });
    viewport.append(circle);

    if (node.degree >= labelThreshold || nodes.length <= 14) {
      const label = document.createElementNS(svgNS, "text");
      label.setAttribute("x", `${node.x + 8}`);
      label.setAttribute("y", `${node.y - 8}`);
      label.setAttribute("class", "topology-label");
      label.textContent = node.label;
      viewport.append(label);
    }
  }

  svg.addEventListener("click", () => {
    state.topology.selectedNode = "";
    state.topology.selectedLink = "";
    if (els.topologyDetails) {
      els.topologyDetails.textContent = "Click a node or link for details.";
    }
    renderTopology();
  });

  applyTopologyViewport(svg, width, height);
  const details = els.topologyDetails;
  els.topologyGraph.replaceChildren(svg);
  if (details) els.topologyGraph.append(details);
}

function runTopologyLayout(nodes, links, width, height) {
  const centerX = width / 2;
  const centerY = height / 2;
  const nodeCount = Math.max(1, nodes.length);
  const area = width * height;
  const base = Math.sqrt(area / nodeCount);
  const repulsion = Math.max(400, base * base * 1.1);
  const spring = 0.015;
  const rest = Math.max(40, Math.min(140, base * 0.88));
  const gravity = 0.018;

  nodes.forEach((node, index) => {
    const angle = (index / nodeCount) * Math.PI * 2;
    const radius = Math.min(width, height) * 0.18;
    node.x = centerX + Math.cos(angle) * radius;
    node.y = centerY + Math.sin(angle) * radius;
    node.vx = 0;
    node.vy = 0;
  });

  for (let step = 0; step < 260; step += 1) {
    for (let i = 0; i < nodes.length; i += 1) {
      const a = nodes[i];
      for (let j = i + 1; j < nodes.length; j += 1) {
        const b = nodes[j];
        let dx = b.x - a.x;
        let dy = b.y - a.y;
        let dist2 = dx * dx + dy * dy;
        if (dist2 < 1) {
          dx = (Math.random() - 0.5) * 0.01;
          dy = (Math.random() - 0.5) * 0.01;
          dist2 = dx * dx + dy * dy;
        }
        const dist = Math.sqrt(dist2);
        const force = repulsion / dist2;
        const fx = (force * dx) / dist;
        const fy = (force * dy) / dist;
        a.vx -= fx;
        a.vy -= fy;
        b.vx += fx;
        b.vy += fy;
      }
    }

    for (const link of links) {
      const a = nodes[link.source];
      const b = nodes[link.target];
      let dx = b.x - a.x;
      let dy = b.y - a.y;
      const dist = Math.max(1, Math.sqrt(dx * dx + dy * dy));
      const desired = Math.max(28, rest - Math.min(30, link.count * 4));
      const force = (dist - desired) * spring;
      const fx = (force * dx) / dist;
      const fy = (force * dy) / dist;
      a.vx += fx;
      a.vy += fy;
      b.vx -= fx;
      b.vy -= fy;
    }

    for (const node of nodes) {
      node.vx += (centerX - node.x) * gravity;
      node.vy += (centerY - node.y) * gravity;
      node.vx *= 0.83;
      node.vy *= 0.83;
      node.x += node.vx;
      node.y += node.vy;
      node.x = Math.max(18, Math.min(width - 18, node.x));
      node.y = Math.max(18, Math.min(height - 18, node.y));
    }
  }
}

function buildTopologyGraphData() {
  const nodeByNum = new Map();

  const links = new Map();
  const addLink = (from, to, snrValue) => {
    if (!from || !to || from === to) return;
    const left = Math.min(from, to);
    const right = Math.max(from, to);
    const key = `${left}:${right}`;
    const entry = links.get(key) || { key, from: left, to: right, count: 0, snrTotal: 0, snrCount: 0 };
    if (Number.isFinite(snrValue)) {
      entry.snrTotal += snrValue;
      entry.snrCount += 1;
    }
    entry.count += 1;
    links.set(key, entry);
  };

  for (const route of state.traceRoutes) {
    const chains = [
      route.towards ?? route.Towards ?? [],
      route.back ?? route.Back ?? [],
    ];
    for (const chain of chains) {
      let previous = 0;
      for (const hop of chain) {
        const node = hop.node ?? hop.Node ?? {};
        const num = nodeNum(node);
        if (!num) continue;
        nodeByNum.set(num, nodeShortName(node) || nodeLongName(node) || nodeByNum.get(num) || formatNodeID(num));
        const snr = hop.snr ?? hop.SNR;
        if (previous) addLink(previous, num, Number(snr));
        previous = num;
      }
    }
  }

  const nodeNums = [...nodeByNum.keys()].sort((a, b) => a - b);
  const nodes = nodeNums.map((num) => ({ num, label: nodeByNum.get(num), degree: 0, x: 0, y: 0 }));
  const indexByNum = new Map(nodes.map((node, index) => [node.num, index]));
  const graphLinks = [];
  for (const link of links.values()) {
    const source = indexByNum.get(link.from);
    const target = indexByNum.get(link.to);
    if (source === undefined || target === undefined) continue;
    nodes[source].degree += 1;
    nodes[target].degree += 1;
    graphLinks.push({ source, target, count: link.count, key: link.key, avgSNR: link.snrCount > 0 ? (link.snrTotal / link.snrCount) : null });
  }
  nodes.forEach((node) => { node.key = `num:${node.num}`; });
  return { nodes, links: graphLinks };
}

function formatSNRValue(value) {
  if (!Number.isFinite(value)) return "n/a";
  return Number(value).toFixed(1);
}

function isSelectedNode(node) {
  return state.topology.selectedNode && state.topology.selectedNode === node.key;
}

function isSelectedLink(link) {
  return state.topology.selectedLink && state.topology.selectedLink === link.key;
}

function bindTopologyViewport(svg, width, height) {
  svg.addEventListener("wheel", (event) => {
    event.preventDefault();
    const delta = event.deltaY < 0 ? 1.12 : 0.9;
    state.topology.zoom = Math.max(0.35, Math.min(4.5, state.topology.zoom * delta));
    applyTopologyViewport(svg, width, height);
  }, { passive: false });

  svg.addEventListener("pointerdown", (event) => {
    state.topology.dragging = true;
    state.topology.dragX = event.clientX;
    state.topology.dragY = event.clientY;
    els.topologyGraph.style.cursor = "grabbing";
  });
  svg.addEventListener("pointermove", (event) => {
    if (!state.topology.dragging) return;
    const dx = event.clientX - state.topology.dragX;
    const dy = event.clientY - state.topology.dragY;
    state.topology.dragX = event.clientX;
    state.topology.dragY = event.clientY;
    state.topology.panX += dx;
    state.topology.panY += dy;
    applyTopologyViewport(svg, width, height);
  });
  const endDrag = () => {
    state.topology.dragging = false;
    if (els.topologyGraph) els.topologyGraph.style.cursor = "grab";
  };
  svg.addEventListener("pointerup", endDrag);
  svg.addEventListener("pointerleave", endDrag);
}

function applyTopologyViewport(svg, width, height) {
  const viewport = svg.querySelector(".topology-viewport");
  if (!viewport) return;
  const cx = width / 2;
  const cy = height / 2;
  const tx = state.topology.panX;
  const ty = state.topology.panY;
  const z = state.topology.zoom;
  viewport.setAttribute("transform", `translate(${tx},${ty}) translate(${cx},${cy}) scale(${z}) translate(${-cx},${-cy})`);
}

function traceKey(route = {}) {
  const requestID = route.requestID ?? route.RequestID ?? 0;
  const from = route.from ?? route.From ?? 0;
  const to = route.to ?? route.To ?? 0;
  const receivedAt = route.receivedAt ?? route.ReceivedAt ?? "";
  if (Number(requestID) > 0) {
    return `id:${Number(requestID)}`;
  }
  if (Number(from) > 0 || Number(to) > 0) {
    if (receivedAt) {
      return `ftt:${Number(from)}:${Number(to)}:${receivedAt}`;
    }
    return "";
  }
  return "";
}

function persistTraceRoutes() {
  try {
    const trimmed = state.traceRoutes.slice(0, TRACE_STORAGE_LIMIT);
    window.localStorage.setItem(TRACE_STORAGE_KEY, JSON.stringify(trimmed));
  } catch {
    // Ignore storage errors (private mode/quota).
  }
}

function restoreTraceRoutes() {
  try {
    const raw = window.localStorage.getItem(TRACE_STORAGE_KEY);
    if (!raw) return;
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return;
    state.traceRoutes = parsed
      .filter((route) => route && typeof route === "object")
      .slice(0, TRACE_STORAGE_LIMIT);
  } catch {
    state.traceRoutes = [];
  }
}

function renderMapOverlay(positions = null) {
  if (!state.map || !state.overlayLayer) return;
  state.overlayLayer.clearLayers();
  if (state.overlay.type === "none") return;

  let points = [];
  try {
    points = overlayPoints(state.overlay.type, state.overlay.cellKm, positions);
  } catch {
    return 0;
  }
  if (!points.length) return 0;

  const values = points.map((point) => point.value).filter((value) => Number.isFinite(value));
  if (!values.length) return 0;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const times = points.map((point) => point.time).filter((value) => Number.isFinite(value));
  const minTime = times.length ? Math.min(...times) : 0;
  const maxTime = times.length ? Math.max(...times) : 0;

  points.sort((left, right) => (left.time || 0) - (right.time || 0));

  for (const point of points) {
    const intensity = max === min ? 0.5 : (point.value - min) / (max - min);
    const recency = maxTime === minTime ? 1 : (((point.time || minTime) - minTime) / (maxTime - minTime));
    const confidence = Number.isFinite(point.confidence) ? point.confidence : 0.5;
    const circle = L.circle([point.lat, point.lon], {
      radius: state.overlay.cellKm * 1000,
      color: overlayColor(intensity),
      weight: 1,
      fillColor: overlayColor(intensity),
      fillOpacity: 0.06 + recency * 0.12 + intensity * 0.16 + confidence * 0.22,
    });
    circle.bindPopup(
      `<strong>${escapeHTML(point.label)}</strong><br>` +
      `${escapeHTML(overlayLabel(state.overlay.type))}: ${escapeHTML(formatOverlayValue(state.overlay.type, point.value))}<br>` +
      `Sample: ${escapeHTML(formatTime(point.timeISO))}<br>` +
      `Confidence: ${escapeHTML((confidence * 100).toFixed(0))}%`,
    );
    circle.addTo(state.overlayLayer);
  }
  return points.length;
}

function renderTraceRoutes() {
  if (!state.map || !state.traceLayer) return 0;
  state.traceLayer.clearLayers();
  if (!state.tracesVisible || state.traceRoutes.length === 0) return 0;

  let drawn = 0;
  state.traceRoutes.forEach((route) => {
    const towards = traceRouteCoordinates(route, "towards");
    const back = traceRouteCoordinates(route, "back");
    if (towards.length >= 2) {
      const line = L.polyline(towards, {
        color: "#ff4fd8",
        weight: 5,
        opacity: 0.98,
        lineCap: "round",
        lineJoin: "round",
      });
      line.bindPopup(tracePopup(route, "towards", towards.length));
      line.addTo(state.traceLayer);
      drawn += 1;
    }
    if (back.length >= 2) {
      const line = L.polyline(back, {
        color: "#ff9af0",
        weight: 4,
        opacity: 0.95,
        dashArray: "12 8",
        lineCap: "round",
        lineJoin: "round",
      });
      line.bindPopup(tracePopup(route, "back", back.length));
      line.addTo(state.traceLayer);
      drawn += 1;
    }
  });
  return drawn;
}

function traceRouteCoordinates(route, direction) {
  const hops = direction === "back"
    ? (route.back ?? route.Back ?? [])
    : (route.towards ?? route.Towards ?? []);
  const points = [];
  for (const hop of hops) {
    const node = hop.node ?? hop.Node ?? {};
    const num = nodeNum(node);
    const position = positionForNodeNum(num);
    if (!position) continue;
    const lat = Number(positionLatitude(position));
    const lon = Number(positionLongitude(position));
    if (!Number.isFinite(lat) || !Number.isFinite(lon)) continue;
    const last = points[points.length - 1];
    if (!last || last[0] !== lat || last[1] !== lon) {
      points.push([lat, lon]);
    }
  }
  return points;
}

function positionForNodeNum(num) {
  if (!num) return null;
  const node = state.nodes.find((item) => nodeNum(item) === num);
  return node ? nodePosition(node) : null;
}

function tracePopup(route, direction, hops) {
  const requestID = route.requestID ?? route.RequestID ?? 0;
  const from = route.from ?? route.From ?? 0;
  const to = route.to ?? route.To ?? 0;
  const rxRSSI = route.rxRssi ?? route.RxRSSI;
  const rxSNR = route.rxSnr ?? route.RxSNR;
  const rx = [];
  if (rxRSSI !== null && rxRSSI !== undefined) rx.push(`rssi ${Number(rxRSSI).toFixed(0)}dBm`);
  if (rxSNR !== null && rxSNR !== undefined) rx.push(`snr ${Number(rxSNR).toFixed(1)}dB`);
  return `<strong>Trace ${escapeHTML(requestID.toString(16).padStart(8, "0"))}</strong><br>` +
    `${escapeHTML(direction)} • hops with geo: ${hops}<br>` +
    `${rx.length ? `${escapeHTML(rx.join(" • "))}<br>` : ""}` +
    `from ${escapeHTML(formatNodeID(from))} to ${escapeHTML(formatNodeID(to))}`;
}

function overlayPoints(type, cellKm, positions = null) {
  const telemetry = type === "density" ? null : telemetryByNode();
  const source = Array.isArray(positions)
    ? positions
    : state.nodes.map((node) => ({ node, position: nodePosition(node) })).filter(({ position }) => hasLatLon(position));
  const samples = source.map(({ node, position }) => {
    const key = nodeNum(node) ? `num:${nodeNum(node)}` : nodeID(node) ? `id:${nodeID(node).toLowerCase()}` : "";
    const bundle = telemetry && key ? telemetry.get(key) : null;
    return {
      lat: Number(positionLatitude(position)),
      lon: Number(positionLongitude(position)),
      node,
      bundle,
    };
  }).filter((sample) => Number.isFinite(sample.lat) && Number.isFinite(sample.lon));

  if (type === "density") {
    const radiusKm = Math.max(0.1, Number(cellKm) || 5);
    return samples.map((sample) => {
      let nearby = 0;
      for (const candidate of samples) {
        if (haversineKm(sample.lat, sample.lon, candidate.lat, candidate.lon) <= radiusKm) {
          nearby += 1;
        }
      }
      return {
        lat: sample.lat,
        lon: sample.lon,
        label: sampleNodeLabel(sample.node),
        value: nearby,
        time: telemetryTime(sample.bundle?.environment ?? sample.bundle?.device ?? sample.bundle?.localstats ?? {}),
        timeISO: telemetryReceivedAt(sample.bundle?.environment ?? sample.bundle?.device ?? sample.bundle?.localstats ?? {}),
        confidence: overlayPointConfidence(type, sample.node, sample.bundle),
      };
    });
  }

  return samples
    .map((sample) => ({
      lat: sample.lat,
      lon: sample.lon,
      label: sampleNodeLabel(sample.node),
      value: overlayMetricValue(type, sample.bundle),
      time: overlayMetricTime(type, sample.bundle),
      timeISO: overlayMetricTimeISO(type, sample.bundle),
      confidence: overlayPointConfidence(type, sample.node, sample.bundle),
    }))
    .filter((sample) => sample.value !== null && Number.isFinite(sample.value));
}

function overlayMetricValue(type, bundle) {
  if (type === "density") return 1;
  if (!bundle) return null;
  if (type === "temperature") return environmentTemperature(bundle.environment);
  if (type === "humidity") return environmentHumidity(bundle.environment);
  if (type === "battery") return telemetryField(bundle.device, "batteryLevel", "BatteryLevel");
  if (type === "channelUtilization") return telemetryField(bundle.localstats, "channelUtilization", "ChannelUtilization");
  return null;
}

function overlayMetricTime(type, bundle) {
  if (!bundle) return 0;
  if (type === "temperature" || type === "humidity") return telemetryTime(bundle.environment);
  if (type === "battery") return telemetryTime(bundle.device);
  if (type === "channelUtilization") return telemetryTime(bundle.localstats);
  return 0;
}

function overlayMetricTimeISO(type, bundle) {
  if (!bundle) return "";
  if (type === "temperature" || type === "humidity") return telemetryReceivedAt(bundle.environment);
  if (type === "battery") return telemetryReceivedAt(bundle.device);
  if (type === "channelUtilization") return telemetryReceivedAt(bundle.localstats);
  return "";
}

function overlayLabel(type) {
  if (type === "density") return "Node density";
  if (type === "temperature") return "Temperature";
  if (type === "humidity") return "Humidity";
  if (type === "battery") return "Battery";
  if (type === "channelUtilization") return "Channel utilization";
  return "Overlay";
}

function formatOverlayValue(type, value) {
  if (type === "density") return formatMaybeInteger(value);
  if (type === "temperature") return formatMaybeNumber(value, "C", 1);
  if (type === "humidity") return formatMaybeNumber(value, "%", 1);
  if (type === "battery") return formatMaybeInteger(value);
  if (type === "channelUtilization") return formatMaybeNumber(value, "%", 1);
  return String(value);
}

function overlayColor(intensity) {
  const t = Math.max(0, Math.min(1, intensity));
  const hue = 210 - t * 170;
  return `hsl(${hue} 80% 55%)`;
}

function overlayPointConfidence(type, node, bundle) {
  let base = 0.35;
  const now = Date.now();
  const time = overlayMetricTime(type, bundle);
  if (time > 0) {
    const ageMinutes = Math.max(0, (now - time) / 60000);
    const ageScore = Math.exp(-ageMinutes / 90);
    base += 0.35 * ageScore;
  }

  const key = nodeNum(node) ? `num:${nodeNum(node)}` : nodeID(node) ? `id:${nodeID(node).toLowerCase()}` : "";
  if (key) {
    let historyCount = 0;
    if (type === "temperature" || type === "humidity") {
      historyCount = historyByNode(state.telemetry.history.environment, key).length;
    } else if (type === "battery") {
      historyCount = historyByNode(state.telemetry.history.device, key).length;
    } else if (type === "channelUtilization") {
      historyCount = historyByNode(state.telemetry.history.localstats, key).length;
    }
    if (historyCount > 0) {
      const depthScore = Math.min(1, Math.log10(historyCount + 1) / 2);
      base += 0.30 * depthScore;
    }
  }
  return Math.max(0.05, Math.min(1, base));
}

function sampleNodeLabel(node) {
  return nodeShortName(node) || nodeLongName(node) || nodeID(node) || formatNodeID(nodeNum(node));
}

function haversineKm(lat1, lon1, lat2, lon2) {
  const toRad = (value) => (value * Math.PI) / 180;
  const dLat = toRad(lat2 - lat1);
  const dLon = toRad(lon2 - lon1);
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLon / 2) ** 2;
  return 6371 * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}

function activateTab(name) {
  document.querySelectorAll(".tab").forEach((button) => {
    button.classList.toggle("active", button.dataset.tab === name);
  });
  document.querySelectorAll(".view").forEach((view) => {
    view.classList.toggle("active", view.dataset.view === name);
  });
  if (name === "map") {
    requestAnimationFrame(() => {
      renderMap();
      if (state.map) {
        state.map.invalidateSize();
        fitMapToMarkers(true);
      }
    });
  }
  if (name === "topology") {
    requestAnimationFrame(() => renderTopology());
  }
}

function isMapTabActive() {
  return document.querySelector('[data-view="map"]')?.classList.contains("active") ?? false;
}

function isTopologyTabActive() {
  return document.querySelector('[data-view="topology"]')?.classList.contains("active") ?? false;
}

function renderMessages() {
	const channel = state.selectedChannel;
	const source = Array.isArray(state.messages) ? state.messages : [];
	const messages = source.filter((message) => displayChannel(channelName(messageChannel(message))) === channel);

  els.messages.replaceChildren();
  els.messageCount.textContent = `${messages.length} shown`;

	for (const message of messages.slice(-200)) {
    const item = document.createElement("li");
    item.className = "message";

		const meta = document.createElement("div");
		meta.className = "message-meta";
		meta.textContent = `[${displayChannel(channelName(messageChannel(message)))}] ${formatFrom(messageFrom(message))}`;

		const text = document.createElement("div");
		text.className = "message-text";
		text.textContent = messageText(message);

    item.append(meta, text);
    els.messages.append(item);
  }

  els.messages.scrollTop = els.messages.scrollHeight;
}

function renderWeather() {
  const environments = latestEnvironment();
  els.weatherSummary.textContent = `${environments.length} stations`;
  els.weather.replaceChildren();

  if (environments.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.textContent = "No weather telemetry yet.";
    els.weather.append(empty);
    return;
  }

  for (const environment of environments) {
    if (!environment || typeof environment !== "object") continue;
    const card = document.createElement("article");
    card.className = "weather-card";

    const header = document.createElement("div");
    header.className = "weather-card-header";
    const title = document.createElement("h3");
    title.textContent = formatEnvironmentNode(environment);
    const time = document.createElement("span");
    time.textContent = formatTime(telemetryReceivedAt(environment));
    header.append(title, time);

    const metrics = document.createElement("dl");
    metrics.className = "weather-metrics";
    appendMetric(metrics, "Temp", formatMaybeNumber(environmentTemperature(environment), "C", 1));
    appendMetric(metrics, "Humidity", formatMaybeNumber(environmentHumidity(environment), "%", 1));
    appendMetric(metrics, "Pressure", formatMaybeNumber(environmentPressure(environment), "hPa", 1));
    appendMetric(metrics, "Wind", formatWind(environment));
    appendMetric(metrics, "Light", formatMaybeNumber(environmentLux(environment), "lx", 1));
    appendMetric(metrics, "Voltage", formatMaybeNumber(environmentVoltage(environment), "V", 2));
    appendMetric(metrics, "IAQ", formatMaybeInteger(environmentIAQ(environment)));

    card.append(header, metrics);
    els.weather.append(card);
  }
}

function renderTelemetry() {
  const byNode = telemetryByNode();
  const cards = [...byNode.values()].sort((left, right) => right.latestTime - left.latestTime);
  els.telemetrySummary.textContent = `${cards.length} nodes with telemetry`;
  els.telemetry.replaceChildren();

  if (cards.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.textContent = "No telemetry yet.";
    els.telemetry.append(empty);
    return;
  }

  const diagnostics = renderTelemetryDiagnosticsCard();
  if (diagnostics) {
    els.telemetry.append(diagnostics);
  }

  for (const cardData of cards) {
    const card = document.createElement("article");
    card.className = "telemetry-card";

    const header = document.createElement("div");
    header.className = "telemetry-card-header";
    const title = document.createElement("h3");
    title.textContent = cardData.label;
    const count = document.createElement("span");
    count.className = "telemetry-count";
    count.textContent = formatTime(cardData.latestAt);
    header.append(title, count);
    card.append(header);

    if (cardData.environment) {
      card.append(telemetrySection("Weather", [
        ["Temp", formatMaybeNumber(environmentTemperature(cardData.environment), "C", 1)],
        ["Humidity", formatMaybeNumber(environmentHumidity(cardData.environment), "%", 1)],
        ["Pressure", formatMaybeNumber(environmentPressure(cardData.environment), "hPa", 1)],
        ["Wind", formatWind(cardData.environment)],
      ]));
    }
    if (cardData.device) {
      card.append(telemetrySection("Device", [
        ["Battery", formatMaybeInteger(telemetryField(cardData.device, "batteryLevel", "BatteryLevel"))],
        ["Voltage", formatMaybeNumber(telemetryField(cardData.device, "voltage", "Voltage"), "V", 2)],
        ["Chan Util", formatMaybeNumber(telemetryField(cardData.device, "channelUtilization", "ChannelUtilization"), "%", 1)],
        ["TX Air", formatMaybeNumber(telemetryField(cardData.device, "airUtilTx", "AirUtilTx"), "%", 1)],
      ]));
    }
    if (cardData.power) {
      card.append(telemetrySection("Power", [
        ["Ch1 V", formatMaybeNumber(telemetryField(cardData.power, "ch1Voltage", "Ch1Voltage"), "V", 2)],
        ["Ch1 A", formatMaybeNumber(telemetryField(cardData.power, "ch1Current", "Ch1Current"), "A", 2)],
        ["Ch2 V", formatMaybeNumber(telemetryField(cardData.power, "ch2Voltage", "Ch2Voltage"), "V", 2)],
        ["Ch3 V", formatMaybeNumber(telemetryField(cardData.power, "ch3Voltage", "Ch3Voltage"), "V", 2)],
      ]));
    }
    if (cardData.airquality) {
      card.append(telemetrySection("Air Quality", [
        ["CO2", formatMaybeInteger(telemetryField(cardData.airquality, "co2", "CO2"))],
        ["PM2.5", formatMaybeInteger(telemetryField(cardData.airquality, "pm25Standard", "Pm25Standard"))],
        ["PM10", formatMaybeInteger(telemetryField(cardData.airquality, "pm100Standard", "Pm100Standard"))],
        ["P2.5 cnt", formatMaybeInteger(telemetryField(cardData.airquality, "particles25um", "Particles25um"))],
      ]));
    }
    if (cardData.localstats) {
      card.append(telemetrySection("Local Stats", [
        ["Online", formatMaybeInteger(telemetryField(cardData.localstats, "numOnlineNodes", "NumOnlineNodes"))],
        ["Total", formatMaybeInteger(telemetryField(cardData.localstats, "numTotalNodes", "NumTotalNodes"))],
        ["TX", formatMaybeInteger(telemetryField(cardData.localstats, "numPacketsTx", "NumPacketsTx"))],
        ["RX", formatMaybeInteger(telemetryField(cardData.localstats, "numPacketsRx", "NumPacketsRx"))],
      ]));
    }
    if (cardData.health) {
      card.append(telemetrySection("Health", [
        ["Heart", formatMaybeInteger(telemetryField(cardData.health, "heartBpm", "HeartBPM"))],
        ["SpO2", formatMaybeInteger(telemetryField(cardData.health, "spO2", "SpO2"))],
        ["Temp", formatMaybeNumber(telemetryField(cardData.health, "temperature", "Temperature"), "C", 1)],
      ]));
    }
    card.append(telemetryTrendSection(cardData));

    els.telemetry.append(card);
  }
}

function renderTelemetryDiagnosticsCard() {
  const locals = latestTelemetrySamples(state.telemetry.localstats);
  const devices = latestTelemetrySamples(state.telemetry.device);
  const avgChannelUtil = averageOf(locals, (sample) => telemetryField(sample, "channelUtilization", "ChannelUtilization"));
  const avgAirUtil = averageOf(locals, (sample) => telemetryField(sample, "airUtilTx", "AirUtilTx"));
  const totalOnline = sumOf(locals, (sample) => telemetryField(sample, "numOnlineNodes", "NumOnlineNodes"));
  const totalNodes = sumOf(locals, (sample) => telemetryField(sample, "numTotalNodes", "NumTotalNodes"));
  const topTalkers = [...locals]
    .sort((left, right) => Number(telemetryField(right, "numPacketsTx", "NumPacketsTx") || 0) - Number(telemetryField(left, "numPacketsTx", "NumPacketsTx") || 0))
    .slice(0, 5);
  const badRatio = averageOf(locals, (sample) => {
    const bad = Number(telemetryField(sample, "numPacketsRxBad", "NumPacketsRxBad") || 0);
    const total = Number(telemetryField(sample, "numPacketsRx", "NumPacketsRx") || 0);
    if (!total) return null;
    return (bad / total) * 100;
  });

  const anomalies = detectTelemetryAnomalies();
  const clusters = detectGeoClusterAlerts();
  if (locals.length === 0 && devices.length === 0 && anomalies.length === 0) {
    return null;
  }

  const card = document.createElement("article");
  card.className = "telemetry-card telemetry-diagnostics";
  const header = document.createElement("div");
  header.className = "telemetry-card-header";
  const title = document.createElement("h3");
  title.textContent = "Mesh Diagnostics";
  const stamp = document.createElement("span");
  stamp.className = "telemetry-count";
  stamp.textContent = `${locals.length} local-stats nodes`;
  header.append(title, stamp);
  card.append(header);

  card.append(telemetrySection("Channel Health", [
    ["Avg Util", formatMaybeNumber(avgChannelUtil, "%", 1)],
    ["Avg TX Air", formatMaybeNumber(avgAirUtil, "%", 1)],
    ["Online Sum", formatMaybeInteger(totalOnline)],
    ["Known Sum", formatMaybeInteger(totalNodes)],
    ["Bad RX Avg", formatMaybeNumber(badRatio, "%", 1)],
  ]));

  const talkersSection = document.createElement("section");
  talkersSection.className = "telemetry-section";
  const talkersTitle = document.createElement("h4");
  talkersTitle.textContent = "Top Talkers";
  talkersSection.append(talkersTitle);
  if (topTalkers.length === 0) {
    const empty = document.createElement("div");
    empty.className = "telemetry-empty";
    empty.textContent = "No local stats packets.";
    talkersSection.append(empty);
  } else {
    const list = document.createElement("ul");
    list.className = "telemetry-list";
    for (const sample of topTalkers) {
      const item = document.createElement("li");
      const tx = telemetryField(sample, "numPacketsTx", "NumPacketsTx");
      item.textContent = `${formatEnvironmentNode(sample)}: ${formatMaybeInteger(tx)} TX`;
      list.append(item);
    }
    talkersSection.append(list);
  }
  card.append(talkersSection);

  const clusterSection = document.createElement("section");
  clusterSection.className = "telemetry-section";
  const clusterTitle = document.createElement("h4");
  clusterTitle.textContent = "Geo Cluster Alerts";
  clusterSection.append(clusterTitle);
  if (clusters.length === 0) {
    const empty = document.createElement("div");
    empty.className = "telemetry-empty";
    empty.textContent = "No geo clusters crossing thresholds.";
    clusterSection.append(empty);
  } else {
    const list = document.createElement("ul");
    list.className = "telemetry-list";
    for (const cluster of clusters.slice(0, 8)) {
      const item = document.createElement("li");
      item.textContent = `${cluster.name}: util ${cluster.avgUtil.toFixed(1)}%, bad RX ${cluster.badRx.toFixed(1)}%, nodes ${cluster.count}`;
      list.append(item);
    }
    clusterSection.append(list);
  }
  card.append(clusterSection);

  const anomalySection = document.createElement("section");
  anomalySection.className = "telemetry-section";
  const anomalyTitle = document.createElement("h4");
  anomalyTitle.textContent = "Anomalies";
  anomalySection.append(anomalyTitle);
  if (anomalies.length === 0) {
    const empty = document.createElement("div");
    empty.className = "telemetry-empty";
    empty.textContent = "No anomalies detected.";
    anomalySection.append(empty);
  } else {
    const list = document.createElement("ul");
    list.className = "telemetry-list";
    for (const anomaly of anomalies.slice(0, 10)) {
      const item = document.createElement("li");
      item.textContent = `${anomaly.node}: ${anomaly.metric} (${anomaly.value}) z=${anomaly.z.toFixed(1)}`;
      list.append(item);
    }
    anomalySection.append(list);
  }
  card.append(anomalySection);

  return card;
}

function detectTelemetryAnomalies() {
  const out = [];
  const envByNode = groupSamplesByNode(state.telemetry.history.environment);
  const devByNode = groupSamplesByNode(state.telemetry.history.device);
  for (const [key, samples] of envByNode) {
    const node = formatEnvironmentNode(samples[0]);
    const temp = anomalyFromSeries(samples, (sample) => environmentTemperature(sample));
    if (temp) out.push({ node, metric: "Temperature", ...temp });
  }
  for (const [key, samples] of devByNode) {
    const node = formatEnvironmentNode(samples[0]);
    const battery = anomalyFromSeries(samples, (sample) => telemetryField(sample, "batteryLevel", "BatteryLevel"));
    if (battery) out.push({ node, metric: "Battery", ...battery });
    const util = anomalyFromSeries(samples, (sample) => telemetryField(sample, "channelUtilization", "ChannelUtilization"));
    if (util) out.push({ node, metric: "Channel Util", ...util });
  }
  return out.sort((left, right) => Math.abs(right.z) - Math.abs(left.z));
}

function detectGeoClusterAlerts() {
  const telemetry = telemetryByNode();
  const points = state.nodes
    .map((node) => ({ node, position: nodePosition(node) }))
    .filter(({ position }) => hasLatLon(position))
    .map(({ node, position }) => {
      const key = nodeNum(node) ? `num:${nodeNum(node)}` : nodeID(node) ? `id:${nodeID(node).toLowerCase()}` : "";
      const bundle = key ? telemetry.get(key) : null;
      const local = bundle?.localstats;
      if (!local) return null;
      const util = Number(telemetryField(local, "channelUtilization", "ChannelUtilization"));
      const rx = Number(telemetryField(local, "numPacketsRx", "NumPacketsRx") || 0);
      const bad = Number(telemetryField(local, "numPacketsRxBad", "NumPacketsRxBad") || 0);
      const badRx = rx > 0 ? (bad / rx) * 100 : 0;
      if (!Number.isFinite(util)) return null;
      return {
        node,
        label: sampleNodeLabel(node),
        lat: Number(positionLatitude(position)),
        lon: Number(positionLongitude(position)),
        util,
        badRx,
      };
    })
    .filter(Boolean);

  const alerts = [];
  const radiusKm = 8;
  for (const point of points) {
    const neighbors = points.filter((candidate) =>
      haversineKm(point.lat, point.lon, candidate.lat, candidate.lon) <= radiusKm);
    if (neighbors.length < 3) continue;
    const avgUtil = neighbors.reduce((sum, sample) => sum + sample.util, 0) / neighbors.length;
    const badRx = neighbors.reduce((sum, sample) => sum + sample.badRx, 0) / neighbors.length;
    if (avgUtil < 25 && badRx < 8) continue;
    alerts.push({
      name: point.label,
      avgUtil,
      badRx,
      count: neighbors.length,
      lat: point.lat,
      lon: point.lon,
    });
  }
  alerts.sort((left, right) => (right.avgUtil + right.badRx) - (left.avgUtil + left.badRx));

  const deduped = [];
  for (const alert of alerts) {
    const overlapping = deduped.some((existing) => haversineKm(alert.lat, alert.lon, existing.lat, existing.lon) < 5);
    if (!overlapping) deduped.push(alert);
  }
  return deduped;
}

function groupSamplesByNode(samples = []) {
  const grouped = new Map();
  for (const sample of samples) {
    const key = environmentKey(sample);
    if (!key) continue;
    const bucket = grouped.get(key) || [];
    bucket.push(sample);
    grouped.set(key, bucket);
  }
  for (const bucket of grouped.values()) {
    bucket.sort((left, right) => telemetryTime(left) - telemetryTime(right));
  }
  return grouped;
}

function anomalyFromSeries(samples, pick) {
  if (!samples || samples.length < 9) return null;
  const values = samples.map((sample) => Number(pick(sample))).filter((value) => Number.isFinite(value));
  if (values.length < 9) return null;
  const latest = values[values.length - 1];
  const baseline = values.slice(0, -1);
  const mean = baseline.reduce((sum, value) => sum + value, 0) / baseline.length;
  const variance = baseline.reduce((sum, value) => sum + (value - mean) ** 2, 0) / baseline.length;
  const std = Math.sqrt(variance);
  if (!(std > 0)) return null;
  const z = (latest - mean) / std;
  if (Math.abs(z) < 3) return null;
  return { value: latest.toFixed(2), z };
}

function averageOf(samples, pick) {
  const values = samples.map((sample) => Number(pick(sample))).filter((value) => Number.isFinite(value));
  if (!values.length) return null;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function sumOf(samples, pick) {
  const values = samples.map((sample) => Number(pick(sample))).filter((value) => Number.isFinite(value));
  if (!values.length) return null;
  return values.reduce((sum, value) => sum + value, 0);
}

function telemetryByNode() {
  const byNode = new Map();
  mergeTelemetryKind(byNode, "environment", latestEnvironment());
  mergeTelemetryKind(byNode, "device", latestTelemetrySamples(state.telemetry.device));
  mergeTelemetryKind(byNode, "power", latestTelemetrySamples(state.telemetry.power));
  mergeTelemetryKind(byNode, "airquality", latestTelemetrySamples(state.telemetry.airquality));
  mergeTelemetryKind(byNode, "localstats", latestTelemetrySamples(state.telemetry.localstats));
  mergeTelemetryKind(byNode, "health", latestTelemetrySamples(state.telemetry.health));
  return byNode;
}

function mergeTelemetryKind(byNode, kind, samples) {
  for (const sample of samples) {
    const key = environmentKey(sample);
    if (!key) continue;
    const row = byNode.get(key) || {
      label: formatEnvironmentNode(sample),
      latestAt: telemetryReceivedAt(sample),
      latestTime: telemetryTime(sample),
    };
    row[kind] = sample;
    const received = telemetryReceivedAt(sample);
    const parsed = telemetryTime(sample);
    if (parsed > row.latestTime) {
      row.latestTime = parsed;
      row.latestAt = received;
    }
    byNode.set(key, row);
  }
}

function telemetrySection(title, rows) {
  const section = document.createElement("section");
  section.className = "telemetry-section";
  const h4 = document.createElement("h4");
  h4.textContent = title;
  section.append(h4);

  const dl = document.createElement("dl");
  dl.className = "telemetry-metrics";
  for (const [label, value] of rows) {
    const item = document.createElement("div");
    const dt = document.createElement("dt");
    const dd = document.createElement("dd");
    dt.textContent = label;
    dd.textContent = value || "-";
    item.append(dt, dd);
    dl.append(item);
  }
  section.append(dl);
  return section;
}

function telemetryTrendSection(cardData) {
  const section = document.createElement("section");
  section.className = "telemetry-section";
  const h4 = document.createElement("h4");
  h4.textContent = "Trends";
  section.append(h4);

  const key = telemetryKeyFromCard(cardData);
  if (!key) {
    const empty = document.createElement("div");
    empty.className = "telemetry-empty";
    empty.textContent = "No trend history.";
    section.append(empty);
    return section;
  }

  const envHist = historyByNode(state.telemetry.history.environment, key, 16);
  const devHist = historyByNode(state.telemetry.history.device, key, 16);
  const localHist = historyByNode(state.telemetry.history.localstats, key, 16);

  const rows = [];
  const tempSpark = sparklineFromSamples(envHist, (sample) => environmentTemperature(sample));
  if (tempSpark) rows.push(["Temp", tempSpark]);
  const battSpark = sparklineFromSamples(devHist, (sample) => telemetryField(sample, "batteryLevel", "BatteryLevel"));
  if (battSpark) rows.push(["Battery", battSpark]);
  const utilSpark = sparklineFromSamples(localHist, (sample) => telemetryField(sample, "channelUtilization", "ChannelUtilization"));
  if (utilSpark) rows.push(["Chan Util", utilSpark]);

  if (!rows.length) {
    const empty = document.createElement("div");
    empty.className = "telemetry-empty";
    empty.textContent = "No trend history.";
    section.append(empty);
    return section;
  }

  const list = document.createElement("ul");
  list.className = "telemetry-list";
  for (const [label, spark] of rows) {
    const item = document.createElement("li");
    item.textContent = `${label}: ${spark}`;
    list.append(item);
  }
  section.append(list);
  return section;
}

function telemetryKeyFromCard(cardData) {
  const sample = cardData.environment || cardData.device || cardData.localstats || cardData.power || cardData.airquality || cardData.health;
  return sample ? environmentKey(sample) : "";
}

function historyByNode(samples, key, limit = 1000) {
  if (!Array.isArray(samples) || !key) return [];
  const out = [];
  for (const sample of samples) {
    if (environmentKey(sample) === key) {
      out.push(sample);
      if (out.length >= limit) break;
    }
  }
  return out.reverse();
}

function sparklineFromSamples(samples, pick) {
  const values = samples.map((sample) => Number(pick(sample))).filter((value) => Number.isFinite(value));
  if (values.length < 3) return "";
  return sparkline(values.slice(-16));
}

function sparkline(values) {
  const chars = ".:-=+*#%@";
  const min = Math.min(...values);
  const max = Math.max(...values);
  if (max === min) return "-".repeat(values.length);
  return values.map((value) => {
    const idx = Math.max(0, Math.min(chars.length - 1, Math.round(((value - min) / (max - min)) * (chars.length - 1))));
    return chars[idx];
  }).join("");
}

function activeChannels(channels) {
	const active = channels.filter((channel) => channelRole(channel) !== "DISABLED");
	return active.length ? active : [{ index: 0, name: "Primary", role: "PRIMARY" }];
}

function displayChannel(name) {
  return name || "Primary";
}

function formatFrom(node) {
	if (!node) return "!00000000";
	return nodeShortName(node) || formatNodeID(nodeNum(node));
}

function formatNode(node) {
	const nodeID = formatNodeID(nodeNum(node));
	const longName = nodeLongName(node);
	const name = nodeShortName(node) || longName || "(unnamed)";
	const base = longName && longName !== name ? `${nodeID}  ${name}  ${longName}` : `${nodeID}  ${name}`;
	const position = nodePosition(node);
	const environment = nodeEnvironment(node);
	const parts = [base];
	if (position) {
		parts.push(formatLatLon(position));
	}
	if (environment) {
		const summary = formatEnvironmentSummary(environment);
		if (summary) {
			parts.push(summary);
		}
	}
	return parts.join("  ");
}

function latestEnvironment() {
  const byNode = new Map();
  for (const node of state.nodes) {
    const environment = nodeEnvironment(node);
    if (environment && typeof environment === "object") {
      const key = environmentKey(environment) || `num:${nodeNum(node)}`;
      if (key) {
        const current = byNode.get(key);
        if (!current || environmentTime(environment) >= environmentTime(current)) {
          byNode.set(key, environment);
        }
      }
    }
  }
  for (const environment of state.environment) {
    if (!environment || typeof environment !== "object") continue;
    const key = environmentKey(environment);
    if (key) {
      const current = byNode.get(key);
      if (!current || environmentTime(environment) >= environmentTime(current)) {
        byNode.set(key, environment);
      }
    }
  }
  return [...byNode.values()].sort((left, right) => {
    const leftTime = environmentTime(left);
    const rightTime = environmentTime(right);
    return rightTime - leftTime;
  });
}

function latestTelemetrySamples(samples = []) {
  const byNode = new Map();
  for (const sample of samples) {
    const key = environmentKey(sample);
    if (!key) continue;
    const current = byNode.get(key);
    if (!current || telemetryTime(sample) >= telemetryTime(current)) {
      byNode.set(key, sample);
    }
  }
  return [...byNode.values()].sort((left, right) => telemetryTime(right) - telemetryTime(left));
}

function telemetryReceivedAt(sample = {}) {
  return sample.receivedAt ?? sample.ReceivedAt ?? sample.timestamp ?? sample.Timestamp ?? "";
}

function telemetryTime(sample = {}) {
  const parsed = Date.parse(telemetryReceivedAt(sample));
  return Number.isFinite(parsed) ? parsed : 0;
}

function telemetryField(sample = {}, camel, pascal) {
  if (Object.prototype.hasOwnProperty.call(sample, camel)) {
    return sample[camel];
  }
  return sample[pascal];
}

function appendMetric(parent, label, value) {
  const row = document.createElement("div");
  const term = document.createElement("dt");
  const detail = document.createElement("dd");
  term.textContent = label;
  detail.textContent = value || "-";
  row.append(term, detail);
  parent.append(row);
}

function formatMapPopup(node, position) {
  const num = nodeNum(node);
  const label = nodeShortName(node) || nodeLongName(node) || formatNodeID(num);
  const weather = formatEnvironmentSummary(nodeEnvironment(node));
  const lastHeard = nodeLastHeard(node);
  const lines = [
    `<strong>${escapeHTML(label)}</strong>${escapeHTML(formatNodeID(num))}`,
    escapeHTML(formatLatLon(position)),
  ];
  if (lastHeard) {
    lines.push(`Last heard: ${escapeHTML(formatTime(lastHeard))}`);
  }
  if (weather) {
    lines.push(escapeHTML(weather));
  }
  if (num) {
    lines.push(`<button type="button" class="map-trace-btn" data-trace-node="${escapeHTML(formatNodeID(num))}">Run traceroute</button>`);
  }
  return lines.join("<br>");
}

function formatEnvironmentNode(environment) {
  const node = environmentNode(environment);
  const num = nodeNum(node);
  return nodeShortName(node) || nodeLongName(node) || nodeID(node) || formatNodeID(num);
}

async function requestSafeArray(path) {
  try {
    const payload = await request(path);
    return Array.isArray(payload) ? payload : [];
  } catch {
    return [];
  }
}

function formatEnvironmentSummary(environment) {
  if (!environment) return "";
  const parts = [];
  const temp = environmentTemperature(environment);
  const humidity = environmentHumidity(environment);
  const wind = environmentWindSpeed(environment);
  if (temp !== null) parts.push(`${Number(temp).toFixed(1)}C`);
  if (humidity !== null) parts.push(`${Number(humidity).toFixed(0)}% RH`);
  if (wind !== null) parts.push(`${Number(wind).toFixed(1)}m/s wind`);
  return parts.join("  ");
}

function formatWind(environment) {
  const speed = environmentWindSpeed(environment);
  if (speed === null) return "-";
  const direction = environmentWindDirection(environment);
  if (direction === null) {
    return `${Number(speed).toFixed(1)} m/s`;
  }
  return `${Number(speed).toFixed(1)} m/s @ ${Number(direction).toFixed(0)}deg`;
}

function formatMaybeNumber(value, suffix, digits) {
  if (value === null || value === undefined || !Number.isFinite(Number(value))) {
    return "-";
  }
  return `${Number(value).toFixed(digits)} ${suffix}`;
}

function formatMaybeInteger(value) {
  if (value === null || value === undefined || !Number.isFinite(Number(value))) {
    return "-";
  }
  return Number(value).toFixed(0);
}

function formatTime(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString();
}

function addDebugLog(message) {
  const line = `${new Date().toLocaleTimeString()}  ${message}`;
  state.debugLog.push(line);
  if (state.debugLog.length > 500) {
    state.debugLog = state.debugLog.slice(-500);
  }
  renderDebugLog();
}

function clearDebugLog() {
  state.debugLog = [];
  renderDebugLog();
}

function renderDebugLog() {
  if (!els.debugLog) return;
  if (!state.debugLog.length) {
    els.debugLog.textContent = "No API activity yet.";
    return;
  }
  els.debugLog.textContent = state.debugLog.join("\n");
  els.debugLog.scrollTop = els.debugLog.scrollHeight;
}

function maybeToastTraceReceived(route) {
  const id = traceRequestID(route);
  if (!id) {
    showToast("Traceroute received", "ok");
    return;
  }
  const now = Date.now();
  const seenAt = state.traceToastSeen.get(id) || 0;
  if (now-seenAt < 5000) return;
  state.traceToastSeen.set(id, now);
  if (state.traceToastSeen.size > 200) {
    const oldest = [...state.traceToastSeen.entries()].sort((a, b) => a[1] - b[1]).slice(0, 100);
    for (const [key] of oldest) state.traceToastSeen.delete(key);
  }
  showToast(`Traceroute received: ${id}`, "ok");
}

function traceRequestID(route = {}) {
  const id = route.requestID ?? route.RequestID;
  if (!Number.isFinite(Number(id)) || Number(id) <= 0) return "";
  return Number(id).toString(16).padStart(8, "0");
}

function showToast(message, kind = "info") {
  let stack = document.querySelector(".toast-stack");
  if (!stack) {
    stack = document.createElement("div");
    stack.className = "toast-stack";
    document.body.append(stack);
  }
  const toast = document.createElement("div");
  toast.className = `toast toast-${kind}`;
  toast.textContent = message;
  stack.append(toast);
  setTimeout(() => toast.classList.add("show"), 0);
  setTimeout(() => {
    toast.classList.remove("show");
    setTimeout(() => toast.remove(), 180);
  }, 2600);
}

function nodeMatches(node, query) {
  if (!query) return true;
  return formatNode(node).toLowerCase().includes(query);
}

function formatTrace(route) {
	const requestID = route.requestID ?? route.RequestID ?? 0;
	const from = route.from ?? route.From ?? 0;
	const to = route.to ?? route.To ?? 0;
	const towards = route.towards ?? route.Towards ?? [];
	const back = route.back ?? route.Back ?? [];
	const rxRSSI = route.rxRssi ?? route.RxRSSI;
	const rxSNR = route.rxSnr ?? route.RxSNR;
	const lines = [
		`id=${requestID.toString(16).padStart(8, "0")}`,
		`from=${formatNodeID(from)} to=${formatNodeID(to)}`,
		`towards: ${formatHops(towards)}`,
	];
	if (rxRSSI !== null || rxSNR !== null) {
		let line = "rx:";
		if (rxRSSI !== null && rxRSSI !== undefined) line += ` rssi=${Number(rxRSSI).toFixed(0)}dBm`;
		if (rxSNR !== null && rxSNR !== undefined) line += ` snr=${Number(rxSNR).toFixed(1)}dB`;
		lines.push(line);
	}
	if (back.length) {
		lines.push(`back:    ${formatHops(back)}`);
	}
	return lines.join("\n");
}

function formatHops(hops = []) {
	if (!hops.length) return "(empty)";
	return hops.map((hop) => {
		const node = hop.node ?? hop.Node ?? {};
		const snr = hop.snr ?? hop.SNR;
		const name = nodeShortName(node) || formatNodeID(nodeNum(node));
		return snr === null || snr === undefined ? name : `${name} ${snr.toFixed(1)}dB`;
	}).join(" -> ");
}

function channelIndex(channel = {}) {
	return channel.index ?? channel.Index ?? 0;
}

function channelName(channel = {}) {
	return channel.name ?? channel.Name ?? "";
}

function channelRole(channel = {}) {
	return channel.role ?? channel.Role ?? "";
}

function messageChannel(message = {}) {
	return message.channel ?? message.Channel ?? {};
}

function messageFrom(message = {}) {
	return message.from ?? message.From ?? {};
}

function messageText(message = {}) {
	return message.text ?? message.Text ?? "";
}

function nodeNum(node = {}) {
	return node.num ?? node.Num ?? 0;
}

function nodeShortName(node = {}) {
	return node.shortName ?? node.ShortName ?? "";
}

function nodeLongName(node = {}) {
	return node.longName ?? node.LongName ?? "";
}

function nodePosition(node = {}) {
	return node.position ?? node.Position ?? null;
}

function nodeEnvironment(node = {}) {
	return node.environment ?? node.Environment ?? null;
}

function nodeLastHeard(node = {}) {
  let best = parseTimeValue(node.lastSeen ?? node.LastSeen);
  best = maxTimeValue(best, parseTimeValue(nodePosition(node)?.receivedAt ?? nodePosition(node)?.ReceivedAt));
  best = maxTimeValue(best, parseTimeValue(nodeEnvironment(node)?.receivedAt ?? nodeEnvironment(node)?.ReceivedAt));
  return best || "";
}

function parseTimeValue(value) {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

function maxTimeValue(left, right) {
  if (!left) return right || "";
  if (!right) return left || "";
  return Date.parse(right) > Date.parse(left) ? right : left;
}

function environmentNode(environment = {}) {
  if (!environment || typeof environment !== "object") return {};
	return environment.node ?? environment.Node ?? {};
}

function environmentTemperature(environment = {}) {
	return environment.temperature ?? environment.Temperature ?? null;
}

function environmentHumidity(environment = {}) {
	return environment.relativeHumidity ?? environment.RelativeHumidity ?? null;
}

function environmentPressure(environment = {}) {
	return environment.barometricPressure ?? environment.BarometricPressure ?? null;
}

function environmentWindSpeed(environment = {}) {
	return environment.windSpeed ?? environment.WindSpeed ?? null;
}

function environmentWindDirection(environment = {}) {
	return environment.windDirection ?? environment.WindDirection ?? null;
}

function environmentLux(environment = {}) {
	return environment.lux ?? environment.Lux ?? null;
}

function environmentVoltage(environment = {}) {
	return environment.voltage ?? environment.Voltage ?? null;
}

function environmentIAQ(environment = {}) {
	return environment.iaq ?? environment.IAQ ?? null;
}

function positionLatitude(position) {
	if (!position) {
		return null;
	}
	return position.latitude ?? position.Latitude ?? null;
}

function positionLongitude(position) {
	if (!position) {
		return null;
	}
	return position.longitude ?? position.Longitude ?? null;
}

function hasLatLon(position) {
  return positionLatitude(position) !== null && positionLongitude(position) !== null;
}

function formatLatLon(position) {
	const lat = positionLatitude(position);
	const lon = positionLongitude(position);
	if (lat === null || lon === null) {
		return "";
	}
	return `${Number(lat).toFixed(6)}, ${Number(lon).toFixed(6)}`;
}

function formatNodeID(num) {
	return `!${Number(num || 0).toString(16).padStart(8, "0")}`;
}

function nodeID(node = {}) {
	return node.id ?? node.ID ?? "";
}

function parseNodeIDToNum(id) {
	if (typeof id !== "string") return 0;
	const normalized = id.trim().toLowerCase();
	const hex = normalized.startsWith("!") ? normalized.slice(1) : normalized;
	if (!/^[0-9a-f]{1,8}$/.test(hex)) return 0;
	const parsed = Number.parseInt(hex, 16);
	return Number.isFinite(parsed) ? parsed : 0;
}

function environmentKey(environment = {}) {
	const node = environmentNode(environment);
	const num = nodeNum(node);
	if (num) return `num:${num}`;
	const id = nodeID(node);
	if (id) return `id:${id.toLowerCase()}`;
	return "";
}

function environmentTime(environment = {}) {
  if (!environment || typeof environment !== "object") return 0;
	const received = Date.parse(environment.receivedAt ?? environment.ReceivedAt ?? "");
	if (Number.isFinite(received)) {
		return received;
	}
	const sample = Date.parse(environment.timestamp ?? environment.Timestamp ?? "");
	if (Number.isFinite(sample)) {
		return sample;
	}
	return 0;
}

function setStatus(text, className) {
  els.status.textContent = text;
  els.status.className = className;
}

function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[char]));
}

function defaultAPIURL() {
  if (window.location.protocol === "http:" && window.location.port === "8080") {
    return window.location.origin;
  }
  return "http://127.0.0.1:8080";
}

connect();
