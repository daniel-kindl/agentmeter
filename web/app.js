"use strict";

const statusNode = document.querySelector("#status");
const dashboardNode = document.querySelector("#dashboard");
const limitsNode = document.querySelector("#limits");
const liveMount = document.querySelector("#live-mount");

// Numbers are formatted in English because they sit inside English copy: a
// machine-locale "574,7 tis." reads as a defect next to "against your busiest
// window". Weekday names are copy too, and follow the page. The 24-hour clock
// stays because en-GB keeps it.
const number = new Intl.NumberFormat("en");
const compact = new Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: 1 });
const clock = new Intl.DateTimeFormat("en-GB", { hour: "2-digit", minute: "2-digit" });
const dayClock = new Intl.DateTimeFormat("en-GB", { weekday: "short", hour: "2-digit", minute: "2-digit" });
const axisDate = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", timeZone: "UTC" });

// This page is left open on a second screen, so everything on it refreshes
// itself. Limits move fastest and lead; the history behind them changes slowly
// enough to reread less often. The server caches provider responses well past
// either interval, so polling costs local requests and nothing more.
const limitRefreshMs = 60_000;
const historyRefreshMs = 180_000;

// Where a live meter changes colour. Below the first it is green, between them
// amber, at or above the second red.
const warnPercent = 75;
const criticalPercent = 90;

// How many unpriced models the warning names before it counts the rest.
const namedModelLimit = 3;

let activeRange = "30d";
let lamp = null;

document.querySelectorAll("[data-range]").forEach((button) => {
  button.addEventListener("click", () => {
    document.querySelectorAll("[data-range]").forEach((item) => item.classList.remove("active"));
    button.classList.add("active");
    activeRange = button.dataset.range;
    loadDashboard();
  });
});

function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

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

function formatReset(value) {
  const at = new Date(value);
  const withinDay = at.getTime() - Date.now() < 24 * 60 * 60 * 1000;
  return (withinDay ? clock : dayClock).format(at);
}

function agentName(source) {
  return { claude: "Claude Code", codex: "Codex" }[source] ?? source;
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

// The API returns only days that recorded usage. A strip drawn straight from
// that list puts two distant days side by side and silently misreads as
// continuous time, so idle days are filled back in at zero.
function continuousDays(days) {
  const byDate = new Map(days.map((day) => [day.date, day]));
  const filled = [];
  const cursor = new Date(`${days[0].date}T00:00:00Z`);
  const last = new Date(`${days.at(-1).date}T00:00:00Z`);
  while (cursor <= last) {
    const date = cursor.toISOString().slice(0, 10);
    filled.push(byDate.get(date) ?? { date, idle: true });
    cursor.setUTCDate(cursor.getUTCDate() + 1);
  }
  return filled;
}

// One bar per day, height by tokens. The density strip this replaces encoded
// magnitude as lightness, which is the channel people read least accurately:
// two days could not be compared and a run of them showed no trend. Height at
// the same size does both.
function renderChart(days) {
  const chart = document.querySelector("#chart");
  chart.replaceChildren();
  if (days.length === 0) return;
  const maximum = Math.max(...days.map(tokens), 1);
  const filled = continuousDays(days);

  const bars = element("div", "chart-bars");
  bars.append(...filled.map((day) => {
    const used = day.idle ? 0 : tokens(day);
    const bar = element("div", "chart-bar");
    // An idle day keeps a visible stub instead of vanishing, so a gap reads as
    // a day with no usage rather than as a break in the axis.
    bar.style.height = `${Math.max((used / maximum) * 100, 2)}%`;
    if (day.idle) bar.classList.add("is-idle");
    bar.title = `${day.date}: ${number.format(used)} tokens`;
    return bar;
  }));

  const axis = element("div", "chart-axis");
  axis.append(element("span", null, axisLabel(filled[0].date)), element("span", null, axisLabel(filled.at(-1).date)));
  chart.append(bars, axis);
}

// The date is a local calendar day the server already bucketed, so it is read
// back as UTC rather than reinterpreted — formatting it in the browser's zone
// would slide the label a day west of Greenwich.
function axisLabel(date) {
  return axisDate.format(new Date(`${date}T00:00:00Z`));
}

function originChip(limits) {
  if (limits.origin === "live") return limits.stale ? { text: "stale", className: "stale" } : { text: "live", className: "live" };
  if (limits.origin === "unavailable") return { text: "no reading", className: "unavailable" };
  return { text: "uncalibrated", className: "" };
}

// Note text alternates prose and measured values. The measured segments are set
// in the mono face so a token count lines up with the wedges above it instead
// of drifting with the surrounding sentence.
function noteSegments(window, origin) {
  if (origin !== "estimated" || !window.budget_tokens) return [];
  return [
    { value: `${compact.format(window.used_tokens)} / ${compact.format(window.budget_tokens)}` },
    " tokens, against your busiest window so far",
  ];
}

// Only a live reading earns a status colour. An estimate sits against the
// busiest window in local history, so it reaches 100% the moment the current
// window is the busiest one — an artefact of a thin baseline, not a limit being
// reached. Lighting that up would be the page telling a lie in colour.
function meterState(used, origin) {
  if (origin !== "live") return "is-derived";
  if (used >= criticalPercent) return "is-critical";
  if (used >= warnPercent) return "is-warn";
  return "";
}

// A filled track, read by length. The numeral beside it already carries the
// value for a screen reader, so the bar is a visual duplicate and says nothing
// of its own.
function renderMeter(used) {
  const meter = element("div", "meter");
  meter.setAttribute("aria-hidden", "true");
  const fill = element("div", "meter-fill");
  fill.style.width = `${used}%`;
  meter.append(fill);
  return meter;
}

function renderWindow(window, origin) {
  const group = element("div", "window");
  const used = Math.min(Math.max(window.utilization, 0), 100);

  const head = element("div", "window-head");
  head.append(element("span", "window-name", window.label));
  // The reset slot always renders. Dropping the line loses half of what the row
  // promises to say, but the two ways a window can lack a reset are not the
  // same thing: a five-hour block has none because nothing is open, while a
  // derived weekly window has none because its real reset instant is not
  // knowable offline. Saying "no block open" for both would misdescribe one.
  head.append(window.resets_at
    ? element("span", "window-reset", `resets ${formatReset(window.resets_at)}`)
    : element("span", "window-reset is-idle", window.kind === "5h" ? "no block open" : "no fixed reset"));
  group.append(head);

  const row = element("div", `window-row ${meterState(used, origin)}`.trim());
  const value = element("div", "window-value");
  const percent = element("span", "window-percent", String(Math.round(used)));
  percent.append(element("span", null, "%"));
  value.append(percent, element("span", "window-unit", "used"));
  row.append(value, renderMeter(used));
  group.append(row);

  const segments = noteSegments(window, origin);
  if (segments.length > 0) {
    const note = element("p", "window-note");
    segments.forEach((segment) => note.append(segment.value ? element("b", null, segment.value) : segment));
    group.append(note);
  }
  return group;
}

function renderLimitCard(limits) {
  const card = element("section", "card");

  const head = element("header", "card-head");
  const chip = originChip(limits);
  head.append(element("h2", null, agentName(limits.source)), element("span", `origin-chip ${chip.className}`.trim(), chip.text));
  card.append(head);

  const windows = element("div", "windows");
  windows.append(...limits.windows.map((window) => renderWindow(window, limits.origin)));
  card.append(windows);

  if (limits.message) card.append(element("p", "window-note", limits.message));
  return card;
}

// Limits are the reason this page exists, so a failure says what happened and
// what to do rather than leaving the history as the first thing on screen.
function renderLimitFailure(message) {
  const card = element("section", "card");
  const head = element("header", "card-head");
  head.append(element("h2", null, "Usage limits"), element("span", "origin-chip unavailable", "no reading"));
  card.append(head, element("p", "window-note", message));
  limitsNode.replaceChildren(card);
  limitsNode.hidden = false;
}

// The safelight lamp is the live switch. It reads local agent credentials and
// contacts the configured endpoints, so the control says so rather than only
// showing that it is on.
//
// It is built once and then updated in place, never replaced. Re-mounting it
// takes keyboard focus with it, and the moment that matters most is the one
// right after someone has just operated it.
function renderLamp(data) {
  if (lamp === null) {
    const label = element("label", "lamp");
    const box = document.createElement("input");
    box.type = "checkbox";
    box.addEventListener("change", () => setLive(box.checked, box));
    const text = element("span");
    label.append(box, text);
    liveMount.replaceChildren(label);
    lamp = { label, box, text };
  }
  lamp.box.checked = data.live;
  lamp.box.disabled = false;
  lamp.text.textContent = data.live ? "Live" : "Estimated";
  lamp.label.title = data.live
    ? "Reading local agent credentials and contacting the configured endpoints."
    : "Turn on to read local agent credentials and fetch each agent's own figures.";
}

function renderLimits(data) {
  if (data.sources.length === 0) {
    renderLimitFailure("No agent reported a limit. Run agentmeter scan, or turn on live readings.");
  } else {
    limitsNode.replaceChildren(...data.sources.map(renderLimitCard));
    limitsNode.hidden = false;
  }
  if (data.configurable) renderLamp(data);
  else liveMount.replaceChildren();
}

function setLiveError(message) {
  const existing = liveMount.parentElement.querySelector(".lamp-error");
  if (existing) existing.remove();
  if (message) liveMount.parentElement.insertBefore(element("span", "lamp-error", message), liveMount);
}

async function setLive(enabled, control) {
  control.disabled = true;
  setLiveError("");
  try {
    const response = await fetch("/api/v1/limits/live", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    });
    if (!response.ok) throw new Error("request failed");
    // The server replies with the report the switch produced, so the cards
    // show the consequence rather than an optimistic guess.
    const data = await response.json();
    renderLimits(data);
    // Turning the switch on can succeed while the credentials behind it do
    // not. That reason lives on the source, and saying nothing about it here
    // would leave the lamp lit over an unchanged reading.
    const rejected = data.sources.find((source) => source.origin !== "live" && source.message);
    setLiveError(enabled && rejected ? rejected.message : "");
  } catch (_) {
    control.checked = !enabled;
    control.disabled = false;
    setLiveError("The preference could not be saved. Check that agentmeter is still running, then try again.");
  }
}

// Limits load independently of the usage history. A limits failure must never
// blank the history, and vice versa.
async function loadLimits() {
  try {
    const response = await fetch("/api/v1/limits");
    if (!response.ok) throw new Error("request failed");
    renderLimits(await response.json());
  } catch (_) {
    renderLimitFailure("The limits could not be read from the local server.");
  }
}

function render(data) {
  const totals = data.totals;
  setText("#input-total", number.format(totals.input_tokens));
  setText("#output-total", number.format(totals.output_tokens));
  setText("#cache-read-total", number.format(totals.cache_read_input_tokens));
  setText("#cost-total", money(totals.cost_usd));
  // Go reports an unset TZ as the literal "Local", which names nothing. The
  // browser knows the real zone.
  const zone = data.timezone === "Local" || !data.timezone
    ? Intl.DateTimeFormat().resolvedOptions().timeZone
    : data.timezone;
  setText("#timezone", zone);
  const warning = document.querySelector("#pricing-warning");
  warning.hidden = data.cost_complete;
  warning.textContent = data.cost_complete ? "" : `Cost excludes unpriced models: ${namedModels(data.unpriced_models)}`;
  renderChart(data.daily);
  renderTable("#source-rows", data.by_source);
  renderTable("#model-rows", data.by_model);
  statusNode.hidden = true;
  dashboardNode.hidden = false;
}

// An unbounded model list runs the warning across the full width. Naming a few
// and counting the rest keeps the sentence readable and says the same thing.
function namedModels(models) {
  if (models.length <= namedModelLimit) return models.join(", ");
  const rest = models.length - namedModelLimit;
  return `${models.slice(0, namedModelLimit).join(", ")}, and ${rest} more`;
}

// A poll must not throw the page back to its loading state; only a range the
// operator just chose earns that.
async function loadDashboard(quiet = false) {
  if (!quiet) {
    statusNode.hidden = false;
    statusNode.textContent = "Reading local session logs…";
  }
  try {
    const response = await fetch(`/api/v1/dashboard?range=${encodeURIComponent(activeRange)}`);
    if (!response.ok) throw new Error("request failed");
    const data = await response.json();
    if (data.daily.length === 0) {
      dashboardNode.hidden = true;
      statusNode.hidden = false;
      statusNode.textContent = "Nothing recorded yet. Run agentmeter scan, then reload.";
      return;
    }
    render(data);
  } catch (_) {
    if (quiet) return;
    dashboardNode.hidden = true;
    statusNode.textContent = "The local usage database could not be read.";
  }
}

loadDashboard();
loadLimits();
setInterval(loadLimits, limitRefreshMs);
setInterval(() => loadDashboard(true), historyRefreshMs);
