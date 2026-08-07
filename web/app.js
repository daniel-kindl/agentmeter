"use strict";

const statusNode = document.querySelector("#status");
const dashboardNode = document.querySelector("#dashboard");
const limitsNode = document.querySelector("#limits");
// Numbers are formatted in English because they sit inside English copy: a
// machine-locale "574,7 tis." reads as a defect next to "against your busiest
// window". Times keep the machine locale, where the 12- or 24-hour convention
// is genuinely regional and the value stands on its own.
const number = new Intl.NumberFormat("en");
const compact = new Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: 1 });
const clock = new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit" });
const dayClock = new Intl.DateTimeFormat(undefined, { weekday: "short", hour: "2-digit", minute: "2-digit" });

// Widest a single day's bar may grow. Without a cap, a first-run history of two
// or three days renders as a few enormous slabs that read as a broken chart.
const maxBarWidth = 56;

// How often the browser re-reads the limits endpoint. The server caches provider
// responses well past this, so polling costs a local request and nothing more.
const limitRefreshMs = 60_000;

document.querySelectorAll("[data-range]").forEach((button) => {
  button.addEventListener("click", () => {
    document.querySelectorAll("[data-range]").forEach((item) => item.classList.remove("active"));
    button.classList.add("active");
    loadDashboard(button.dataset.range);
  });
});

function tokens(value) {
  return value.input_tokens + value.output_tokens + value.cache_creation_input_tokens + value.cache_read_input_tokens;
}

function setText(selector, value) {
  document.querySelector(selector).textContent = value;
}

// Costs arrive with full micro-dollar precision. Anything a person would read
// as money is shown as money; sub-cent totals keep their digits rather than
// rounding away to nothing.
function money(value) {
  const amount = Number(value);
  if (!Number.isFinite(amount)) return `$${value}`;
  return amount >= 0.005 ? `$${amount.toFixed(2)}` : `$${value}`;
}

function renderTable(selector, rows) {
  const body = document.querySelector(selector);
  body.replaceChildren(...rows.map((row) => {
    const tr = document.createElement("tr");
    [row.name, number.format(tokens(row)), money(row.cost_usd)].forEach((value) => {
      const cell = document.createElement("td");
      cell.textContent = value;
      tr.append(cell);
    });
    return tr;
  }));
}

function renderChart(days) {
  const chart = document.querySelector("#chart");
  chart.replaceChildren();
  if (days.length === 0) return;
  const values = days.map(tokens);
  const maximum = Math.max(...values, 1);
  const slot = 860 / Math.max(days.length, 1);
  const width = Math.min(Math.max(slot - 3, 2), maxBarWidth);
  const inset = (slot - width) / 2;
  const namespace = "http://www.w3.org/2000/svg";
  days.forEach((day, index) => {
    const height = (tokens(day) / maximum) * 220;
    const rect = document.createElementNS(namespace, "rect");
    rect.setAttribute("x", String(20 + inset + index * slot));
    rect.setAttribute("y", String(245 - height));
    rect.setAttribute("width", String(width));
    rect.setAttribute("height", String(height));
    rect.setAttribute("rx", "3");
    const title = document.createElementNS(namespace, "title");
    title.textContent = `${day.date}: ${number.format(tokens(day))} tokens`;
    rect.append(title);
    chart.append(rect);
  });
}

function render(data) {
  const totals = data.totals;
  setText("#input-total", number.format(totals.input_tokens));
  setText("#output-total", number.format(totals.output_tokens));
  setText("#cache-read-total", number.format(totals.cache_read_input_tokens));
  setText("#cost-total", money(totals.cost_usd));
  // Go reports an unset TZ as the literal "Local", which names nothing. The
  // browser knows the real zone, so use it rather than showing a placeholder.
  const zone = data.timezone === "Local" || !data.timezone
    ? Intl.DateTimeFormat().resolvedOptions().timeZone
    : data.timezone;
  setText("#timezone", zone);
  const warning = document.querySelector("#pricing-warning");
  warning.hidden = data.cost_complete;
  warning.textContent = data.cost_complete ? "" : `Cost excludes unpriced models: ${data.unpriced_models.join(", ")}`;
  renderChart(data.daily);
  renderTable("#source-rows", data.by_source);
  renderTable("#model-rows", data.by_model);
  statusNode.hidden = true;
  dashboardNode.hidden = false;
}

function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function formatReset(value) {
  const at = new Date(value);
  const withinDay = at.getTime() - Date.now() < 24 * 60 * 60 * 1000;
  return (withinDay ? clock : dayClock).format(at);
}

function agentName(source) {
  return { claude: "Claude Code", codex: "Codex" }[source] ?? source;
}

function originBadge(limits) {
  if (limits.origin === "live") return limits.stale ? { text: "stale", className: "stale" } : { text: "live", className: "live" };
  if (limits.origin === "unavailable") return { text: "unavailable", className: "unavailable" };
  return { text: "estimate", className: "" };
}

// Note text alternates prose and measured values. The measured segments are set
// in the mono face, so a reset time or token count lines up with the meters
// above it instead of drifting with the surrounding sentence.
function noteSegments(window, origin) {
  const segments = [];
  if (window.resets_at) segments.push("resets ", { value: formatReset(window.resets_at) });
  if (origin === "estimated" && window.budget_tokens) {
    if (segments.length > 0) segments.push(" · ");
    segments.push(
      { value: `${compact.format(window.used_tokens)} / ${compact.format(window.budget_tokens)}` },
      " tokens, against your busiest window so far",
    );
  }
  return segments;
}

function renderMeter(window, origin) {
  const row = element("div", "meter-row");
  const label = element("div", "meter-label");
  label.append(element("span", null, window.label), element("strong", null, `${Math.round(window.utilization)}%`));

  const track = element("div", "meter");
  const severity = window.utilization >= 90 ? " danger" : window.utilization >= 75 ? " warn" : "";
  const fill = element("div", `meter-fill${severity}`);
  fill.style.width = `${Math.min(Math.max(window.utilization, 0), 100)}%`;
  track.append(fill);
  row.append(label, track);

  const segments = noteSegments(window, origin);
  if (segments.length > 0) {
    const note = element("p", "meter-note");
    segments.forEach((segment) => note.append(segment.value ? element("b", null, segment.value) : segment));
    row.append(note);
  }
  return row;
}

function renderLimitPanel(limits) {
  const panel = element("section", "panel limit-panel");
  const heading = element("div", "panel-heading");
  const badge = originBadge(limits);
  heading.append(element("h2", null, agentName(limits.source)), element("span", `origin-badge ${badge.className}`.trim(), badge.text));
  panel.append(heading);

  const meters = element("div", "meters");
  meters.append(...limits.windows.map((window) => renderMeter(window, limits.origin)));
  panel.append(meters);

  if (limits.message) panel.append(element("p", "limit-message", limits.message));
  return panel;
}

// The switch reads local agent credentials and contacts the vendors, so the
// control says what it does rather than only that it is on.
function renderLiveSwitch(data) {
  const bar = element("div", "limits-bar");
  bar.append(element("h2", null, "Usage limits"));

  const control = element("label", "live-switch");
  const box = document.createElement("input");
  box.type = "checkbox";
  box.checked = data.live;
  box.addEventListener("change", () => setLive(box.checked, box));
  control.append(box, element("span", null, data.live ? "Live" : "Estimated"));
  control.title = data.live
    ? "Reading local agent credentials and contacting the configured endpoints."
    : "Turn on to read local agent credentials and fetch the agents' own numbers.";
  bar.append(control);
  return bar;
}

function renderLimits(data) {
  const children = data.sources.map(renderLimitPanel);
  if (data.configurable) children.unshift(renderLiveSwitch(data));
  limitsNode.replaceChildren(...children);
  limitsNode.hidden = children.length === 0;
}

async function setLive(enabled, control) {
  control.disabled = true;
  try {
    const response = await fetch("/api/v1/limits/live", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    });
    if (!response.ok) throw new Error("request failed");
    // The server replies with the report the switch produced, so the panels
    // show the consequence rather than an optimistic guess.
    renderLimits(await response.json());
  } catch (_) {
    control.checked = !enabled;
    control.disabled = false;
  }
}

// Limits load independently of the token dashboard. A limits failure must never
// blank the usage history, and vice versa.
async function loadLimits() {
  try {
    const response = await fetch("/api/v1/limits");
    if (!response.ok) throw new Error("request failed");
    renderLimits(await response.json());
  } catch (_) {
    limitsNode.replaceChildren();
    limitsNode.hidden = true;
  }
}

async function loadDashboard(range) {
  statusNode.hidden = false;
  statusNode.textContent = "Loading local usage…";
  try {
    const response = await fetch(`/api/v1/dashboard?range=${encodeURIComponent(range)}`);
    if (!response.ok) throw new Error("request failed");
    const data = await response.json();
    if (data.daily.length === 0) {
      dashboardNode.hidden = true;
      statusNode.textContent = "No usage is stored yet. Run agentmeter scan, then refresh this page.";
      return;
    }
    render(data);
  } catch (_) {
    dashboardNode.hidden = true;
    statusNode.textContent = "The dashboard could not read the local usage database.";
  }
}

loadDashboard("30d");
loadLimits();
setInterval(loadLimits, limitRefreshMs);
