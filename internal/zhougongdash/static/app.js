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
function renderDetails() { renderAgents(); renderRatio(); renderRunPairs(); renderFlow(); }

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

const PALETTE = ["#00ff9c","#00e5ff","#ff2bd6","#ffb000","#7c8cff","#ff6b6b","#b6ff00","#c78bff","#4dffd2","#ff9de2"];
const short = v => v === null || v === undefined ? "n/a" : Math.abs(v) >= 1e6 ? (v / 1e6).toFixed(1) + "M" : Math.abs(v) >= 1e3 ? (v / 1e3).toFixed(1) + "k" : fmt(v);
const num = v => typeof v === "number" && isFinite(v) ? v : 0;
const ROW = 28, PW = 420;

// Horizontal bar panel; rows align across panels because ROW/top offset are shared.
function hbarPanel(title, items, color, opts) {
  opts = opts || {};
  const lw = opts.labels ? 170 : 0, bw = PW - lw - 70, H = 22 + ROW * items.length;
  const max = Math.max(1, ...items.map(i => Math.max(num(i.v), num(i.v2))));
  let g = `<text x="${lw}" y="14" fill="#00e5ff" font-size="11">${esc(title)}</text>`;
  items.forEach((it, i) => {
    const y = 22 + i * ROW, w = Math.round(bw * num(it.v) / max), w2 = Math.round(bw * num(it.v2) / max);
    if (it.cur) g += `<rect x="0" y="${y}" width="${PW}" height="${ROW - 2}" fill="rgba(0,255,156,.12)"/>`;
    if (opts.labels) g += `<text x="2" y="${y + 17}" fill="${it.cur ? "#00ff9c" : "#0a9f68"}" font-size="11">${esc(it.label.length > 24 ? it.label.slice(0, 23) + "…" : it.label)}${it.cur ? " [HEAD]" : ""}</text>`;
    if (opts.secondary && it.v2 !== undefined) g += `<rect x="${lw}" y="${y + 3}" width="${w2}" height="${ROW - 8}" fill="none" stroke="#ff2bd6" stroke-width="1"><title>${esc(it.label)} total tokens: ${fmt(it.v2)}</title></rect>`;
    g += `<rect x="${lw}" y="${y + 7}" width="${w}" height="${ROW - 16}" fill="${color}" filter="url(#glow)"><title>${esc(it.label)}: ${fmt(it.v)}</title></rect>` +
      `<text x="${lw + Math.max(w, w2) + 5}" y="${y + 17}" fill="${color}" font-size="11">${it.v === null || it.v === undefined ? "n/a" : short(it.v)}</text>`;
  });
  return `<svg class="chart" viewBox="0 0 ${PW} ${H}" width="${PW}" height="${H}" role="img" aria-label="${esc(title)}"><defs><filter id="glow"><feGaussianBlur stdDeviation="1.5" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs>${g}</svg>`;
}

function metricPairs(sums) {
  const rows = sums.map(s => ({ label: s.name, cur: isCur(s.name), nd: s.noData, out: s.noData ? null : s.output, tot: s.noData ? null : s.total,
    lines: s.noData ? null : s.linesChanged, ratio: s.noData ? null : s.tokenToCode }));
  const tok = rows.map(r => ({ label: r.label, cur: r.cur, v: r.out, v2: r.tot }));
  const ln = rows.map(r => ({ label: r.label, cur: r.cur, v: r.lines }));
  const rt = rows.map(r => ({ label: r.label, cur: r.cur, v: r.ratio }));
  return `<div class="pair">` +
    hbarPanel("OUTPUT TOKENS (magenta outline = total tokens)", tok, "#00ff9c", { labels: true, secondary: true }) +
    hbarPanel("CODE LINES (added+removed)", ln, "#00e5ff") +
    hbarPanel("TOKENS / LINE (n/a when 0 lines)", rt, "#ffb000") + `</div>` + scatter(rows);
}

function scatter(rows) {
  const pts = rows.filter(r => !r.nd && r.lines > 0 && r.out !== null);
  if (!pts.length) return "<em>no branches with code lines to plot</em>";
  const W = 560, H = 320, L = 60, B = 36, T = 16, R = 90;
  const mx = Math.max(...pts.map(p => p.lines)), my = Math.max(1, ...pts.map(p => p.out));
  const X = v => L + (W - L - R) * v / mx, Y = v => H - B - (H - B - T) * v / my;
  let g = `<line x1="${L}" y1="${H - B}" x2="${W - R}" y2="${H - B}" stroke="#0a9f68"/><line x1="${L}" y1="${T}" x2="${L}" y2="${H - B}" stroke="#0a9f68"/>` +
    `<text x="${(L + W - R) / 2}" y="${H - 8}" fill="#00e5ff" font-size="11" text-anchor="middle">code lines changed</text>` +
    `<text x="12" y="${(T + H - B) / 2}" fill="#00e5ff" font-size="11" text-anchor="middle" transform="rotate(-90 12 ${(T + H - B) / 2})">output tokens</text>` +
    `<text x="${L}" y="${H - B + 13}" fill="#0a9f68" font-size="10">0</text><text x="${W - R}" y="${H - B + 13}" fill="#0a9f68" font-size="10" text-anchor="end">${short(mx)}</text>` +
    `<text x="${L - 4}" y="${T + 8}" fill="#0a9f68" font-size="10" text-anchor="end">${short(my)}</text>`;
  for (const p of pts) {
    const c = p.cur ? "#00ff9c" : "#ff2bd6", x = X(p.lines), y = Y(p.out);
    g += `<circle cx="${x}" cy="${y}" r="${p.cur ? 7 : 5}" fill="${c}" filter="url(#glow)"><title>${esc(p.label)}: ${fmt(p.lines)} lines, ${fmt(p.out)} output tokens, ${fmt(p.ratio)} tokens/line</title></circle>` +
      `<text x="${x + 9}" y="${y + 4}" fill="${c}" font-size="10">${esc(p.label.length > 14 ? p.label.slice(0, 13) + "…" : p.label)}</text>`;
  }
  const skipped = rows.length - pts.length;
  return `<h4>Scatter: lines vs output tokens</h4><svg class="chart" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" role="img" aria-label="scatter">` +
    `<defs><filter id="glow"><feGaussianBlur stdDeviation="1.5" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs>${g}</svg>` +
    (skipped ? `<small> ${skipped} branch(es) omitted (0 lines or no data)</small>` : "");
}

function vbars(title, vals, color, labels, vals2) {
  const W = Math.max(260, 30 * vals.length + 60), H = 170, L = 44, B = 22, T = 18;
  const max = Math.max(1, ...vals.map(num), ...(vals2 || []).map(num)), bw = (W - L - 8) / Math.max(1, vals.length);
  let g = `<text x="${L}" y="12" fill="#00e5ff" font-size="11">${esc(title)}</text><line x1="${L}" y1="${H - B}" x2="${W}" y2="${H - B}" stroke="#0a9f68"/>` +
    `<text x="${L - 4}" y="${T + 8}" fill="#0a9f68" font-size="10" text-anchor="end">${short(max)}</text><text x="${L - 4}" y="${H - B}" fill="#0a9f68" font-size="10" text-anchor="end">0</text>`;
  vals.forEach((v, i) => {
    const x = L + i * bw, h = (H - B - T) * num(v) / max, h2 = vals2 ? (H - B - T) * num(vals2[i]) / max : 0;
    if (vals2) g += `<rect x="${x + 1}" y="${H - B - h2}" width="${bw - 3}" height="${h2}" fill="none" stroke="#ff2bd6"><title>#${i + 1} ${esc(labels[i])} total tokens: ${fmt(vals2[i])}</title></rect>`;
    g += `<rect x="${x + 4}" y="${H - B - h}" width="${Math.max(1, bw - 9)}" height="${h}" fill="${color}"><title>#${i + 1} ${esc(labels[i])}: ${v === null || v === undefined ? "n/a" : fmt(v)}</title></rect>` +
      `<text x="${x + bw / 2}" y="${H - 8}" fill="#0a9f68" font-size="9" text-anchor="middle">${i + 1}</text>`;
  });
  return `<svg class="chart" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" role="img" aria-label="${esc(title)}">${g}</svg>`;
}

function runPair(d) {
  if (!d.runs.length) return "<em>no runs</em>";
  const labels = d.runs.map(r => r.agent);
  return `<div class="pair">` +
    vbars("OUTPUT TOKENS per run (outline = total)", d.runs.map(r => r.output), "#00ff9c", labels, d.runs.map(r => r.total)) +
    vbars("CODE LINES per run", d.runs.map(r => r.linesAdded + r.linesRemoved), "#00e5ff", labels) + `</div>`;
}
function renderRunPairs() {
  $("runpairs").innerHTML = selected().map(d => `<h3>${esc(d.summary.name)}${isCur(d.summary.name) ? '<span class="tag head">HEAD</span>' : ""}</h3>${runPair(d)}`).join("");
}

function renderGraphs() {
  $("graphs").innerHTML = metricPairs(data.datasets.map(d => d.summary));
}

function agentColor(m, a) { return PALETTE[m.agents.indexOf(a) % PALETTE.length]; }
function renderAgentMatrix() {
  const m = data.agentMatrix;
  if (!m || !m.agents.length) { $("agentmatrix").innerHTML = "<em>no agent data</em>"; return; }
  const maxTok = Math.max(1, ...m.cells.flat().map(c => c.total));
  let h = `<table><tr><th>agent \\ branch</th>` + m.branches.map(b => `<th title="${esc(b)}">${esc(b.length > 18 ? b.slice(0, 17) + "…" : b)}${isCur(b) ? '<span class="tag head">HEAD</span>' : ""}</th>`).join("") + `<th>total</th></tr>`;
  m.agents.forEach((a, i) => {
    const tot = m.cells[i].reduce((s, c) => s + c.total, 0);
    h += `<tr><td><span class="swatch" style="background:${agentColor(m, a)}"></span>${esc(a)}</td>` + m.cells[i].map(c =>
      c.commits || c.total ? `<td class="heat" style="background:rgba(0,255,156,${(0.08 + 0.7 * c.total / maxTok).toFixed(2)})" title="${esc(a)}: ${c.commits} commits, ${fmt(c.total)} total, ${fmt(c.output)} output">${c.commits} / ${short(c.total)}</td>` : `<td class="empty">-</td>`).join("") + `<td>${short(tot)}</td></tr>`;
  });
  h += `</table><small>cell = calls (commits) / total tokens; brighter = more tokens</small><h3>Agents used per branch</h3><table>`;
  m.branches.forEach((b, j) => {
    const used = m.agents.filter((a, i) => m.cells[i][j].commits || m.cells[i][j].total);
    h += `<tr${isCur(b) ? ' class="current"' : ""}><td>${esc(b)}</td><td style="text-align:left">${used.length ? used.map(a => `<span class="tag" style="color:${agentColor(m, a)};border-color:${agentColor(m, a)}">${esc(a)}</span>`).join("") : "<em>none</em>"}</td></tr>`;
  });
  h += `</table><h3>Token share by agent</h3>` + stacked(m);
  $("agentmatrix").innerHTML = h;
}
function stacked(m) {
  const W = 620, L = 170, H = 22 + ROW * m.branches.length;
  let g = "";
  m.branches.forEach((b, j) => {
    const y = 6 + j * ROW, tot = m.agents.reduce((s, a, i) => s + m.cells[i][j].total, 0);
    g += `<text x="2" y="${y + 16}" fill="${isCur(b) ? "#00ff9c" : "#0a9f68"}" font-size="11">${esc(b.length > 24 ? b.slice(0, 23) + "…" : b)}</text>`;
    if (!tot) { g += `<text x="${L}" y="${y + 16}" fill="#0a9f68" font-size="11">n/a</text>`; return; }
    let x = L;
    m.agents.forEach((a, i) => {
      const t = m.cells[i][j].total, w = (W - L - 4) * t / tot;
      if (!t) return;
      g += `<rect x="${x}" y="${y + 3}" width="${w}" height="${ROW - 8}" fill="${agentColor(m, a)}" fill-opacity=".85"><title>${esc(b)} / ${esc(a)}: ${fmt(t)} tokens (${(100 * t / tot).toFixed(1)}%)</title></rect>`;
      x += w;
    });
  });
  const legend = m.agents.map(a => `<span class="tag" style="color:${agentColor(m, a)};border-color:${agentColor(m, a)}">${esc(a)}</span>`).join("");
  return `<svg class="chart" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" role="img" aria-label="token share by agent">${g}</svg><div>${legend}</div>`;
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
  const bySummary = new Map(data.datasets.map(d => [d.summary.name, d]));
  h += "<h3>Tokens vs code lines (selected)</h3>" + metricPairs(body.branches);
  h += "<h3>Per run (selected)</h3>" + body.branches.map(b => `<h4>${esc(b.name)}</h4>` + (bySummary.has(b.name) ? runPair(bySummary.get(b.name)) : "<em>no data</em>")).join("");
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
  renderPicker(); renderOverview(); renderGraphs(); renderAgentMatrix(); renderDetails();
  if (busy) setTimeout(load, 2000);
}
$("banner").textContent = "collecting current branch...";
load();
