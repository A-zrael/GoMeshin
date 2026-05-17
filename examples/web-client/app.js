const state = {
  apiURL: defaultAPIURL(),
  events: null,
  channels: [],
  nodes: [],
  environment: [],
  messages: [],
  selectedChannel: "Primary",
  map: null,
  markers: new Map(),
  mapFitDone: false,
};

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
  weather: document.querySelector("#weather"),
  weatherSummary: document.querySelector("#weather-summary"),
  refreshWeather: document.querySelector("#refresh-weather"),
  nodes: document.querySelector("#nodes"),
  nodeSearch: document.querySelector("#node-search"),
  traceTarget: document.querySelector("#trace-target"),
  traceForm: document.querySelector("#trace-form"),
  traceOutput: document.querySelector("#trace-output"),
  refreshChannels: document.querySelector("#refresh-channels"),
  refreshNodes: document.querySelector("#refresh-nodes"),
};

els.apiURL.value = state.apiURL;

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
els.nodeSearch.addEventListener("input", renderNodes);
els.fitMap.addEventListener("click", () => fitMapToMarkers(true));

els.traceForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const to = els.traceTarget.value;
  if (!to) return;

  els.traceOutput.textContent = "Waiting for traceroute...";
  try {
    const route = await request("/traceroute", {
      method: "POST",
      body: {
        to,
        channel: state.selectedChannel,
        hopLimit: 3,
        timeoutSeconds: 90,
      },
    });
    els.traceOutput.textContent = formatTrace(route);
  } catch (error) {
    els.traceOutput.textContent = error.message;
  }
});

async function connect() {
  closeEvents();
  setStatus("Connecting...", "");

  try {
    await request("/health");
    await Promise.all([loadChannels(), loadNodes(), loadMessages(), loadWeather()]);
    openEvents();
    setStatus(`Connected to ${state.apiURL}`, "connected");
  } catch (error) {
    setStatus(error.message, "error");
  }
}

async function loadChannels() {
	state.channels = activeChannels(await request("/channels"));
	if (!state.channels.some((channel) => displayChannel(channelName(channel)) === state.selectedChannel)) {
		state.selectedChannel = displayChannel(channelName(state.channels[0]) || "Primary");
	}
	renderChannels();
}

async function loadNodes() {
  state.nodes = await request("/nodes");
  renderNodes();
  renderWeather();
  renderMap();
}

async function loadWeather() {
  state.environment = await request("/telemetry/environment");
  for (const environment of state.environment) {
    updateNodeEnvironment(environment);
  }
  renderWeather();
  renderNodes();
  renderMap();
}

async function loadMessages() {
  state.messages = await request("/messages");
  renderMessages();
}

function openEvents() {
  const events = new EventSource(`${state.apiURL}/events`);
  state.events = events;

  events.addEventListener("message.received", (event) => {
    const envelope = JSON.parse(event.data);
    state.messages.push(envelope.data);
    if (state.messages.length > 500) {
      state.messages = state.messages.slice(-500);
    }
    renderMessages();
  });

  events.addEventListener("position.updated", (event) => {
    const envelope = JSON.parse(event.data);
    updateNodePosition(envelope.data);
    renderNodes();
    renderMap();
  });

  events.addEventListener("environment.updated", (event) => {
    const envelope = JSON.parse(event.data);
    upsertEnvironment(envelope.data);
    updateNodeEnvironment(envelope.data);
    renderWeather();
    renderNodes();
    renderMap();
  });

  events.onerror = () => {
    setStatus("Event stream disconnected", "error");
  };
}

function closeEvents() {
  if (state.events) {
    state.events.close();
    state.events = null;
  }
}

async function request(path, options = {}) {
  const init = {
    method: options.method || "GET",
    headers: {},
  };

  if (options.body) {
    init.headers["content-type"] = "application/json";
    init.body = JSON.stringify(options.body);
  }

  let response;
  try {
    response = await fetch(`${state.apiURL}${path}`, init);
  } catch (error) {
    throw new Error(`Cannot reach ${state.apiURL}. Check that gomeshind is running and CORS allows this page.`);
  }
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
  els.traceTarget.replaceChildren();

	for (const node of nodes) {
		const nodeID = formatNodeID(nodeNum(node));
		const label = formatNode(node);

    const item = document.createElement("li");
    item.textContent = label;
    els.nodes.append(item);

    const option = document.createElement("option");
    option.value = nodeID;
    option.textContent = label;
    els.traceTarget.append(option);
  }
}

function renderMap() {
  const positions = state.nodes
    .map((node) => ({ node, position: nodePosition(node) }))
    .filter(({ position }) => hasLatLon(position));

  els.mapSummary.textContent = `${positions.length} positions`;

  if (!isMapTabActive()) {
    return;
  }

  if (positions.length === 0) {
    clearMapMarkers();
    const empty = document.createElement("div");
    empty.className = "map-empty";
    empty.textContent = "No node positions yet.";
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
  const num = nodeNum(environmentNode(environment));
  if (!num) return;

  const existingIndex = state.environment.findIndex((item) => nodeNum(environmentNode(item)) === num);
  if (existingIndex >= 0) {
    state.environment[existingIndex] = environment;
    return;
  }
  state.environment.unshift(environment);
}

function updateNodeEnvironment(environment) {
  const num = nodeNum(environmentNode(environment));
  if (!num) return;

  const existing = state.nodes.find((node) => nodeNum(node) === num);
  if (existing) {
    existing.Environment = environment;
    existing.environment = environment;
    return;
  }

  const node = environmentNode(environment);
  state.nodes.push({
    Num: num,
    ID: node.ID ?? node.id ?? "",
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
    const existing = state.markers.get(key);
    if (existing) {
      existing.setLatLng([lat, lon]);
      existing.setIcon(nodeIcon(label));
      existing.setPopupContent(popup);
    } else {
      state.markers.set(key, L.marker([lat, lon], { icon: nodeIcon(label) }).bindPopup(popup).addTo(state.map));
    }
  }

  for (const [key, marker] of state.markers) {
    if (!seen.has(key)) {
      marker.remove();
      state.markers.delete(key);
    }
  }

  fitMapToMarkers(false);
  setTimeout(() => state.map.invalidateSize(), 0);
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

function nodeIcon(label) {
  const initial = (label || "?").trim().slice(0, 1).toUpperCase() || "?";
  return L.divIcon({
    className: "",
    html: `<div class="mesh-marker" title="${escapeHTML(label)}"><span>${escapeHTML(initial)}</span></div>`,
    iconSize: [26, 26],
    iconAnchor: [13, 13],
    popupAnchor: [0, -13],
  });
}

function clearMapMarkers() {
  for (const marker of state.markers.values()) {
    marker.remove();
  }
  state.markers.clear();
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
}

function isMapTabActive() {
  return document.querySelector('[data-view="map"]')?.classList.contains("active") ?? false;
}

function renderMessages() {
	const channel = state.selectedChannel;
	const messages = state.messages.filter((message) => displayChannel(channelName(messageChannel(message))) === channel);

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
    const card = document.createElement("article");
    card.className = "weather-card";

    const header = document.createElement("div");
    header.className = "weather-card-header";
    const title = document.createElement("h3");
    title.textContent = formatEnvironmentNode(environment);
    const time = document.createElement("span");
    time.textContent = formatTime(environment.receivedAt ?? environment.ReceivedAt);
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
    if (environment) {
      byNode.set(nodeNum(environmentNode(environment)) || nodeNum(node), environment);
    }
  }
  for (const environment of state.environment) {
    const num = nodeNum(environmentNode(environment));
    if (num) {
      byNode.set(num, environment);
    }
  }
  return [...byNode.values()].sort((left, right) => {
    const leftTime = Date.parse(left.receivedAt ?? left.ReceivedAt ?? "") || 0;
    const rightTime = Date.parse(right.receivedAt ?? right.ReceivedAt ?? "") || 0;
    return rightTime - leftTime;
  });
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
  const lines = [
    `<strong>${escapeHTML(label)}</strong>${escapeHTML(formatNodeID(num))}`,
    escapeHTML(formatLatLon(position)),
  ];
  if (weather) {
    lines.push(escapeHTML(weather));
  }
  return lines.join("<br>");
}

function formatEnvironmentNode(environment) {
  const node = environmentNode(environment);
  const num = nodeNum(node);
  return nodeShortName(node) || nodeLongName(node) || formatNodeID(num);
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
	const lines = [
		`id=${requestID.toString(16).padStart(8, "0")}`,
		`from=${formatNodeID(from)} to=${formatNodeID(to)}`,
		`towards: ${formatHops(towards)}`,
	];
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

function environmentNode(environment = {}) {
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
