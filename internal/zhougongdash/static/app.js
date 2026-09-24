"use strict";
let data = null;
let sortKey = "name", sortDir = 1;

const isCur = n => data.currentDataset && n === data.currentDataset;
function selected() {
  const el = $("picker");
  const sel = el ? [...el.querySelectorAll("input:checked")].map(i => i.value) : [];
  const pool = sel.length ? sel : (data.currentDataset ? [data.currentDataset] : []);
  const out = data.datasets.filter(d => pool.includes(d.summary.name));
  return out.length ? out : data.datasets;
}
function renderDetails() { renderAgents(); renderRatio(); renderFlow(); }

const $ = id => document.getElementById(id);
const esc = s => String(s).replace(/[&<>"]/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c]));
const fmt = v => v === null || v === undefined ? "n/a" : (Number.isInteger(v) ? v.toLocaleString() : v.toFixed(2));

const COLUMNS = [
  ["name","branch"],["runs","runs"],["commits","commits"],["total","total tokens"],["output","output tokens"],
  ["linesChanged","code lines"],["tokenToCode","tokens/line"],["avgTotalPerRun","avg tokens/run"],["avgCommitsPerRun","avg commits/run"],
];

function bars(rows, label, value) {
  const max = Math.max(1, ...rows.map(value));
  return "<table>" + rows.map(r =>
    `<tr><td>${esc(label(r))}</td><td style="text-align:left"><span class="bar" style="width:${Math.round(180*value(r)/max)}px"></span>${fmt(value(r))}</td></tr>`
  ).join("") + "</table>";
}

function renderOverview() {
  const rows = data.datasets.map(d => d.summary).slice();
  rows.sort((a, b) => {
    const x = a[sortKey], y = b[sortKey];
    if (x === y) return 0;
    if (x === null || x === undefined) return 1;
    if (y === null || y === undefined) return -1;
    return (x < y ? -1 : 1) * sortDir;
  });
  let h = "<table><tr>" + COLUMNS.map(([k, t]) => `<th data-k="${k}">${t}${k === sortKey ? (sortDir > 0 ? " ▲" : " ▼") : ""}</th>`).join("") + "</tr>";
  for (const s of rows) {
    h += `<tr${isCur(s.name) ? ' class="current"' : ""}>` + COLUMNS.map(([k]) => k === "name"
      ? `<td>${esc(s.name)}<span class="tag">${esc(s.source)}</span>${isCur(s.name) ? '<span class="tag head">HEAD</span>' : ""}</td>` : `<td>${fmt(s[k])}</td>`).join("") + "</tr>";
  }
  $("overview").innerHTML = h + "</table>";
  $("overview").querySelectorAll("th").forEach(th => th.onclick = () => {
    const k = th.dataset.k;
    if (k === sortKey) sortDir = -sortDir; else { sortKey = k; sortDir = 1; }
    renderOverview();
  });
  $("overview").insertAdjacentHTML("beforeend", "<h3>Runs per branch</h3>" + bars(rows, r => r.name, r => r.runs));
}

function renderAgents() {
  $("agents").innerHTML = selected().map(d =>
    `<h3>${esc(d.summary.name)}</h3><table><tr><th>agent</th><th>calls (commits)</th><th>runs</th><th>total tokens</th><th>output tokens</th></tr>` +
    d.agents.map(a => `<tr><td>${esc(a.agent)}</td><td>${fmt(a.commits)}</td><td>${fmt(a.runs)}</td><td>${fmt(a.total)}</td><td>${fmt(a.output)}</td></tr>`).join("") +
    `</table>` + bars(d.agents, a => a.agent + " tokens", a => a.total)).join("");
}

function renderRatio() {
  $("ratio").innerHTML = selected().map(d =>
    `<h3>${esc(d.summary.name)}</h3><table>` + d.runs.map((r, i) =>
      `<tr><td>#${i + 1} ${esc(r.agent)}</td><td style="text-align:left">${r.tokenToCode === null ? "n/a" :
        `<span class="bar" style="width:${Math.min(300, Math.round(r.tokenToCode))}px"></span>${fmt(r.tokenToCode)}`}</td></tr>`).join("") + "</table>").join("");
}

function flowHtml(path) {
  return path.length ? `<div class="flow">${path.map(a => `<span>${esc(a)}</span>`).join(" → ")}</div>` : "<em>no runs</em>";
}
function trHtml(tr) {
  return tr.length ? "<table><tr><th>from</th><th>to</th><th>count</th></tr>" +
    tr.map(t => `<tr><td>${esc(t.from)}</td><td>${esc(t.to)}</td><td>${t.count}</td></tr>`).join("") + "</table>" : "";
}
function renderFlow() {
  $("flow").innerHTML = "<h3>Typical flow (most frequent across branches)</h3>" + flowHtml(data.typicalFlow) +
    selected().map(d => `<h3>${esc(d.summary.name)}</h3>${flowHtml(d.summary.flow)}${trHtml(d.transitions)}`).join("");
}

function renderPicker() {
  $("picker").className = "picker";
  $("picker").innerHTML = data.datasets.map(d =>
    `<label><input type="checkbox" value="${esc(d.summary.name)}"${isCur(d.summary.name) ? " checked" : ""}> ${esc(d.summary.name)}</label>`).join("");
  const sync = () => {
    const sel = [...$("picker").querySelectorAll("input:checked")].map(i => i.value);
    const cur = $("baseline").value;
    $("baseline").innerHTML = sel.map(n => `<option${n === cur ? " selected" : ""}>${esc(n)}</option>`).join("");
  };
  $("picker").onchange = () => { sync(); renderDetails(); };
  sync();
  $("go").onclick = runCompare;
}

async function runCompare() {
  const sel = [...$("picker").querySelectorAll("input:checked")].map(i => i.value);
  $("cmp-error").textContent = "";
  const q = new URLSearchParams({ branches: sel.join(","), baseline: $("baseline").value });
  const res = await fetch("/api/compare?" + q);
  const body = await res.json();
  if (!res.ok) { $("cmp-error").textContent = body.error; $("compare").innerHTML = ""; return; }
  const names = body.branches.map(b => b.name);
  let h = "<table><tr><th>metric</th>" + body.branches.map(b =>
    `<th>${esc(b.name)}${b.source === "archived" ? '<span class="tag">archived</span>' : ""}${b.name === body.baseline ? '<span class="tag">baseline</span>' : ""}${b.noData ? '<span class="tag">no data</span>' : ""}</th>`).join("") + "</tr>";
  for (const m of body.metrics) {
    h += `<tr><td>${esc(m.metric)}</td>` + m.cells.map(c => {
      let d = "";
      if (c.delta !== null) {
        const cls = c.delta > 0 ? "up" : c.delta < 0 ? "down" : "";
        const sign = c.delta > 0 ? "+" : "";
        d = ` <small class="${cls}">${sign}${fmt(c.delta)}${c.deltaPct === null ? "" : ` / ${sign}${c.deltaPct.toFixed(0)}%`}</small>`;
      }
      return `<td>${c.value === null ? "n/a" : fmt(c.value)}${d}</td>`;
    }).join("") + "</tr>";
  }
  h += "</table>";
  for (const m of ["runs", "total", "tokenToCode"]) {
    const row = body.metrics.find(r => r.metric === m);
    h += `<h3>${esc(m)}</h3>` + bars(names.map((n, i) => ({ n, v: row.cells[i].value || 0 })), r => r.n, r => r.v);
  }
  h += "<h3>Flow paths</h3>" + body.branches.map((b, i) => `<h4>${esc(b.name)}</h4>${flowHtml(b.flow)}`).join("");
  h += "<h3>Transition matrix</h3>" + transitionMatrix(names, body.transitions);
  $("compare").innerHTML = h;
}

function transitionMatrix(names, trs) {
  const pairs = [...new Set(trs.flat().map(t => t.from + " → " + t.to))];
  if (!pairs.length) return "<em>no transitions</em>";
  return "<table><tr><th>transition</th>" + names.map(n => `<th>${esc(n)}</th>`).join("") + "</tr>" +
    pairs.map(p => `<tr><td>${esc(p)}</td>` + trs.map(tr => {
      const t = tr.find(x => x.from + " → " + x.to === p); return `<td>${t ? t.count : 0}</td>`;
    }).join("") + "</tr>").join("") + "</table>";
}

async function load() {
  data = await (await fetch("/api/summary")).json();
  const busy = data.collecting && data.collecting.length;
  $("banner").textContent = data.attribution +
    (busy ? " Collecting branches: " + data.collecting.join(", ") + "..." : "") +
    (data.collectError ? " Collect failed: " + data.collectError : "");
  $("foot").textContent = "Code lines exclude: " + data.exclusions.join(", ") + ". Token-to-code = output tokens / (lines added + removed); n/a when 0 lines.";
  renderPicker(); renderOverview(); renderDetails();
  if (busy) setTimeout(load, 2000);
}
$("banner").textContent = "collecting current branch...";
load();
