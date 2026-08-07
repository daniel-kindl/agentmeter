"use strict";

const statusNode = document.querySelector("#status");
const dashboardNode = document.querySelector("#dashboard");
const limitsNode = document.querySelector("#limits");
const liveMount = document.querySelector("#live-mount");

// Numbers are formatted in English because they sit inside English copy: a
// machine-locale "574,7 tis." reads as a defect next to "against your busiest
// window". Times keep the machine locale, where the 12- or 24-hour convention
// is genuinely regional and the value stands on its own.
const number = new Intl.NumberFormat("en");
const compact = new Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: 1 });
// Weekday names are copy, not a numeric convention, so they follow the page's
// English rather than the machine locale: "resets út 05:00" is a defect in an
// otherwise English sentence. The 24-hour clock stays because en-GB keeps it.
const clock = new Intl.DateTimeFormat("en-GB", { hour: "2-digit", minute: "2-digit" });
const dayClock = new Intl.DateTimeFormat("en-GB", { weekday: "short", hour: "2-digit", minute: "2-digit" });

// How often the browser re-reads the limits endpoint. This page is meant to be
// left open on a second screen, so it has to stay current without being
// touched. The server caches provider responses well past this interval, so
// polling costs a local request and nothing more.
const limitRefreshMs = 60_000;

document.querySelectorAll("[data-range]").forEach((button) => {
  button.addEventListener("click", () => {
    document.querySelectorAll("[data-range]").forEach((item) => item.classList.remove("active"));
    button.classList.add("active");
    loadDashboard(button.dataset.range);
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

// The daily history is a test strip: one exposure per day, read by density.
// A bar chart this small cannot be read from across a room, and this page is
// meant to be glanced at rather than studied.
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

function renderChart(days) {
  const chart = document.querySelector("#chart");
  chart.replaceChildren();
  if (days.length === 0) return;
  const maximum = Math.max(...days.map(tokens), 1);
  chart.append(...continuousDays(days).map((day) => {
    const used = day.idle ? 0 : tokens(day);
    const cell = element("div");
    // Paper darkens with exposure, so an idle day stays at the paper tone and
    // a heavy day approaches maximum density. The ramp stops short of the
    // ground: taken all the way down, the busiest day matched the panel behind
    // it and read as a hole in the strip rather than its darkest exposure.
    const light = 90 - (used / maximum) * 72;
    cell.style.setProperty("--cell", `hsl(34 22% ${light}%)`);
    cell.title = `${day.date}: ${number.format(used)} tokens`;
    return cell;
  }));
}

function originChip(limits) {
  if (limits.origin === "live") return limits.stale ? { text: "stale", className: "stale" } : { text: "live", className: "live" };
  if (limits.origin === "unavailable") return { text: "no reading", className: "unavailable" };
  return { text: "uncalibrated", className: "" };
}

// Note text alternates prose and measured values. The measured segments are set
// in the mono face so a reset time or token count lines up with the wedges
// above it instead of drifting with the surrounding sentence.
function noteSegments(window, origin) {
  const segments = [];
  if (origin === "estimated" && window.budget_tokens) {
    segments.push(
      { value: `${compact.format(window.used_tokens)} / ${compact.format(window.budget_tokens)}` },
      " tokens, against your busiest window so far",
    );
  }
  return segments;
}

function renderWedge(window, origin) {
  const row = element("div", "wedge-row");
  const used = Math.min(Math.max(window.utilization, 0), 100);
  // Only a live reading earns the alarm. An estimate sits against the busiest
  // window in local history, so it reaches 100% the moment the current window
  // is the busiest one — an artefact of a thin baseline, not a limit being
  // reached. Lighting that up would be the page telling a lie in colour.
  if (origin === "live") {
    if (used >= 90) row.classList.add("is-critical");
    else if (used >= 75) row.classList.add("is-warn");
  }

  const track = element("div", "wedge");
  const exposed = element("div", "wedge-exposed");
  exposed.style.setProperty("--exposed-width", `${used}%`);
  track.append(exposed, element("div", "wedge-steps"));

  const value = element("div", "wedge-value", String(Math.round(used)));
  value.append(element("span", null, "%"));
  row.append(track, value);

  const group = element("div", "wedge-group");
  const head = element("div", "wedge-head");
  head.append(element("span", "wedge-name", window.label));
  if (window.resets_at) head.append(element("span", "wedge-reset", `resets ${formatReset(window.resets_at)}`));
  group.append(head, row);

  const segments = noteSegments(window, origin);
  if (segments.length > 0) {
    const note = element("p", "wedge-note");
    segments.forEach((segment) => note.append(segment.value ? element("b", null, segment.value) : segment));
    group.append(note);
  }
  return group;
}

function renderLimitSheet(limits) {
  const sheet = element("section", "sheet");
  if (limits.origin !== "live") sheet.classList.add("is-estimated");

  const head = element("header", "sheet-head");
  const chip = originChip(limits);
  head.append(element("h2", null, agentName(limits.source)), element("span", `origin-chip ${chip.className}`.trim(), chip.text));
  sheet.append(head);

  const wedges = element("div", "wedges");
  wedges.append(...limits.windows.map((window) => renderWedge(window, limits.origin)));
  sheet.append(wedges);

  if (limits.message) sheet.append(element("p", "wedge-note", limits.message));
  return sheet;
}

// The safelight lamp is the live switch. It reads local agent credentials and
// contacts the configured endpoints, so the control says so rather than only
// showing that it is on.
function renderLamp(data) {
  const label = element("label", "lamp");
  const box = document.createElement("input");
  box.type = "checkbox";
  box.checked = data.live;
  box.addEventListener("change", () => setLive(box.checked, box));
  label.append(box, element("span", null, data.live ? "Live" : "Estimated"));
  label.title = data.live
    ? "Reading local agent credentials and contacting the configured endpoints."
    : "Turn on to read local agent credentials and fetch each agent's own figures.";
  liveMount.replaceChildren(label);
}

function renderLimits(data) {
  limitsNode.replaceChildren(...data.sources.map(renderLimitSheet));
  limitsNode.hidden = data.sources.length === 0;
  if (data.configurable) renderLamp(data);
  else liveMount.replaceChildren();
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
    // The server replies with the report the switch produced, so the sheets
    // show the consequence rather than an optimistic guess.
    renderLimits(await response.json());
  } catch (_) {
    control.checked = !enabled;
    control.disabled = false;
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
    limitsNode.replaceChildren();
    limitsNode.hidden = true;
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
  warning.textContent = data.cost_complete ? "" : `Cost excludes unpriced models: ${data.unpriced_models.join(", ")}`;
  renderChart(data.daily);
  renderTable("#source-rows", data.by_source);
  renderTable("#model-rows", data.by_model);
  statusNode.hidden = true;
  dashboardNode.hidden = false;
}

async function loadDashboard(range) {
  statusNode.hidden = false;
  statusNode.textContent = "Reading local session logs…";
  try {
    const response = await fetch(`/api/v1/dashboard?range=${encodeURIComponent(range)}`);
    if (!response.ok) throw new Error("request failed");
    const data = await response.json();
    if (data.daily.length === 0) {
      dashboardNode.hidden = true;
      statusNode.textContent = "Nothing recorded yet. Run agentmeter scan, then reload.";
      return;
    }
    render(data);
  } catch (_) {
    dashboardNode.hidden = true;
    statusNode.textContent = "The local usage database could not be read.";
  }
}

loadDashboard("30d");
loadLimits();
setInterval(loadLimits, limitRefreshMs);
