const state = {
  apiURL: defaultAPIURL(),
  events: null,
  channels: [],
  nodes: [],
  messages: [],
  selectedChannel: "Primary",
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
  nodes: document.querySelector("#nodes"),
  nodeSearch: document.querySelector("#node-search"),
  traceTarget: document.querySelector("#trace-target"),
  traceForm: document.querySelector("#trace-form"),
  traceOutput: document.querySelector("#trace-output"),
  refreshChannels: document.querySelector("#refresh-channels"),
  refreshNodes: document.querySelector("#refresh-nodes"),
};

els.apiURL.value = state.apiURL;

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
els.nodeSearch.addEventListener("input", renderNodes);

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
    await Promise.all([loadChannels(), loadNodes(), loadMessages()]);
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
	return longName && longName !== name ? `${nodeID}  ${name}  ${longName}` : `${nodeID}  ${name}`;
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
