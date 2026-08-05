"use strict";

const statusNode = document.querySelector("#status");
const dashboardNode = document.querySelector("#dashboard");
const number = new Intl.NumberFormat();

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

function renderTable(selector, rows) {
  const body = document.querySelector(selector);
  body.replaceChildren(...rows.map((row) => {
    const tr = document.createElement("tr");
    [row.name, number.format(tokens(row)), `$${row.cost_usd}`].forEach((value) => {
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
  const width = 860 / Math.max(days.length, 1);
  const namespace = "http://www.w3.org/2000/svg";
  days.forEach((day, index) => {
    const height = (tokens(day) / maximum) * 220;
    const rect = document.createElementNS(namespace, "rect");
    rect.setAttribute("x", String(20 + index * width));
    rect.setAttribute("y", String(245 - height));
    rect.setAttribute("width", String(Math.max(width - 3, 2)));
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
  setText("#cost-total", `$${totals.cost_usd}`);
  setText("#timezone", data.timezone);
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
