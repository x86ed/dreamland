import re

path = "internal/zhougongdash/static/app.js"
with open(path) as f:
    src = f.read()

# ordered (old, new, expected_count) replacements
reps = [
    # palette: 10 saturated theme-consistent hues
    ('const PALETTE = ["#00ff9c","#00e5ff","#ff2bd6","#ffb000","#7c8cff","#ff6b6b","#b6ff00","#c78bff","#4dffd2","#ff9de2"];',
     'const PALETTE = ["#00fff2","#ff00c8","#fff200","#ff2c1c","#20e8cf","#e3b520","#8a7cff","#ff7a33","#4dffb0","#ff6fae"];', 1),

    # hbarPanel: title text
    ('let g = `<text x="${lw}" y="14" fill="#00e5ff" font-size="11">${esc(title)}</text>`;',
     'let g = `<text x="${lw}" y="14" fill="#93907f" font-size="11">${esc(title)}</text>`;', 1),
    # hbarPanel: current-row highlight band
    ('if (it.cur) g += `<rect x="0" y="${y}" width="${PW}" height="${ROW - 2}" fill="rgba(0,255,156,.12)"/>`;',
     'if (it.cur) g += `<rect x="0" y="${y}" width="${PW}" height="${ROW - 2}" fill="rgba(255,44,28,.10)"/>`;', 1),
    # hbarPanel: label color cur/non-cur
    ('if (opts.labels) g += `<text x="2" y="${y + 17}" fill="${it.cur ? "#00ff9c" : "#0a9f68"}" font-size="11">${esc(it.label.length > 24 ? it.label.slice(0, 23) + "…" : it.label)}${it.cur ? " [HEAD]" : ""}</text>`;',
     'if (opts.labels) g += `<text x="2" y="${y + 17}" fill="${it.cur ? "#ff2c1c" : "#5f5c50"}" font-size="11">${esc(it.label.length > 24 ? it.label.slice(0, 23) + "…" : it.label)}${it.cur ? " [HEAD]" : ""}</text>`;', 1),
    # hbarPanel: secondary outline stroke
    ('if (opts.secondary && it.v2 !== undefined) g += `<rect x="${lw}" y="${y + 3}" width="${w2}" height="${ROW - 8}" fill="none" stroke="#ff2bd6" stroke-width="1"><title>${esc(it.label)} total tokens: ${fmt(it.v2)}</title></rect>`;',
     'if (opts.secondary && it.v2 !== undefined) g += `<rect x="${lw}" y="${y + 3}" width="${w2}" height="${ROW - 8}" fill="none" stroke="#ff00c8" stroke-width="1"><title>${esc(it.label)} total tokens: ${fmt(it.v2)}</title></rect>`;', 1),
    # hbarPanel: main bar - drop glow filter
    ('g += `<rect x="${lw}" y="${y + 7}" width="${w}" height="${ROW - 16}" fill="${color}" filter="url(#glow)"><title>${esc(it.label)}: ${fmt(it.v)}</title></rect>` +',
     'g += `<rect x="${lw}" y="${y + 7}" width="${w}" height="${ROW - 16}" fill="${color}"><title>${esc(it.label)}: ${fmt(it.v)}</title></rect>` +', 1),
    # hbarPanel: drop defs/filter from returned svg
    ('return `<svg class="chart" viewBox="0 0 ${PW} ${H}" width="${PW}" height="${H}" role="img" aria-label="${esc(title)}"><defs><filter id="glow"><feGaussianBlur stdDeviation="1.5" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs>${g}</svg>`;',
     'return `<svg class="chart" viewBox="0 0 ${PW} ${H}" width="${PW}" height="${H}" role="img" aria-label="${esc(title)}">${g}</svg>`;', 1),

    # metricPairs call-site colors
    ('hbarPanel("OUTPUT TOKENS (magenta outline = total tokens)", tok, "#00ff9c", { labels: true, secondary: true }) +',
     'hbarPanel("OUTPUT TOKENS (magenta outline = total tokens)", tok, "#00fff2", { labels: true, secondary: true }) +', 1),
    ('hbarPanel("CODE LINES (added+removed)", ln, "#00e5ff") +',
     'hbarPanel("CODE LINES (added+removed)", ln, "#fff200") +', 1),
    ('hbarPanel("TOKENS / LINE (n/a when 0 lines)", rt, "#ffb000") + `</div>` + scatter(rows);',
     'hbarPanel("TOKENS / LINE (n/a when 0 lines)", rt, "#ff00c8") + `</div>` + scatter(rows);', 1),

    # scatter(): axis lines + titles + ticks
    ('let g = `<line x1="${L}" y1="${H - B}" x2="${W - R}" y2="${H - B}" stroke="#0a9f68"/><line x1="${L}" y1="${T}" x2="${L}" y2="${H - B}" stroke="#0a9f68"/>` +\n    `<text x="${(L + W - R) / 2}" y="${H - 8}" fill="#00e5ff" font-size="11" text-anchor="middle">code lines changed</text>` +\n    `<text x="12" y="${(T + H - B) / 2}" fill="#00e5ff" font-size="11" text-anchor="middle" transform="rotate(-90 12 ${(T + H - B) / 2})">output tokens</text>` +\n    `<text x="${L}" y="${H - B + 13}" fill="#0a9f68" font-size="10">0</text><text x="${W - R}" y="${H - B + 13}" fill="#0a9f68" font-size="10" text-anchor="end">${short(mx)}</text>` +\n    `<text x="${L - 4}" y="${T + 8}" fill="#0a9f68" font-size="10" text-anchor="end">${short(my)}</text>`;',
     'let g = `<line x1="${L}" y1="${H - B}" x2="${W - R}" y2="${H - B}" stroke="#5c584a"/><line x1="${L}" y1="${T}" x2="${L}" y2="${H - B}" stroke="#5c584a"/>` +\n    `<text x="${(L + W - R) / 2}" y="${H - 8}" fill="#93907f" font-size="11" text-anchor="middle">code lines changed</text>` +\n    `<text x="12" y="${(T + H - B) / 2}" fill="#93907f" font-size="11" text-anchor="middle" transform="rotate(-90 12 ${(T + H - B) / 2})">output tokens</text>` +\n    `<text x="${L}" y="${H - B + 13}" fill="#5f5c50" font-size="10">0</text><text x="${W - R}" y="${H - B + 13}" fill="#5f5c50" font-size="10" text-anchor="end">${short(mx)}</text>` +\n    `<text x="${L - 4}" y="${T + 8}" fill="#5f5c50" font-size="10" text-anchor="end">${short(my)}</text>`;', 1),
    # scatter(): point color cur/non-cur, drop glow filter
    ('const c = p.cur ? "#00ff9c" : "#ff2bd6", x = X(p.lines), y = Y(p.out);',
     'const c = p.cur ? "#ff2c1c" : "#ff00c8", x = X(p.lines), y = Y(p.out);', 1),
    ('g += `<circle cx="${x}" cy="${y}" r="${p.cur ? 7 : 5}" fill="${c}" filter="url(#glow)"><title>${esc(p.label)}: ${fmt(p.lines)} lines, ${fmt(p.out)} output tokens, ${fmt(p.ratio)} tokens/line</title></circle>` +',
     'g += `<circle cx="${x}" cy="${y}" r="${p.cur ? 7 : 5}" fill="${c}"><title>${esc(p.label)}: ${fmt(p.lines)} lines, ${fmt(p.out)} output tokens, ${fmt(p.ratio)} tokens/line</title></circle>` +', 1),
    # scatter(): drop defs/filter from returned svg
    ('return `<h4>Scatter: lines vs output tokens</h4><svg class="chart" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" role="img" aria-label="scatter">` +\n    `<defs><filter id="glow"><feGaussianBlur stdDeviation="1.5" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs>${g}</svg>` +',
     'return `<h4>Scatter: lines vs output tokens</h4><svg class="chart" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" role="img" aria-label="scatter">${g}</svg>` +', 1),

    # vbars(): title + axis line + ticks
    ('let g = `<text x="${L}" y="12" fill="#00e5ff" font-size="11">${esc(title)}</text><line x1="${L}" y1="${H - B}" x2="${W}" y2="${H - B}" stroke="#0a9f68"/>` +\n    `<text x="${L - 4}" y="${T + 8}" fill="#0a9f68" font-size="10" text-anchor="end">${short(max)}</text><text x="${L - 4}" y="${H - B}" fill="#0a9f68" font-size="10" text-anchor="end">0</text>`;',
     'let g = `<text x="${L}" y="12" fill="#93907f" font-size="11">${esc(title)}</text><line x1="${L}" y1="${H - B}" x2="${W}" y2="${H - B}" stroke="#5c584a"/>` +\n    `<text x="${L - 4}" y="${T + 8}" fill="#5f5c50" font-size="10" text-anchor="end">${short(max)}</text><text x="${L - 4}" y="${H - B}" fill="#5f5c50" font-size="10" text-anchor="end">0</text>`;', 1),
    # vbars(): secondary outline + index label
    ('if (vals2) g += `<rect x="${x + 1}" y="${H - B - h2}" width="${bw - 3}" height="${h2}" fill="none" stroke="#ff2bd6"><title>#${i + 1} ${esc(labels[i])} total tokens: ${fmt(vals2[i])}</title></rect>`;',
     'if (vals2) g += `<rect x="${x + 1}" y="${H - B - h2}" width="${bw - 3}" height="${h2}" fill="none" stroke="#ff00c8"><title>#${i + 1} ${esc(labels[i])} total tokens: ${fmt(vals2[i])}</title></rect>`;', 1),
    ('`<text x="${x + bw / 2}" y="${H - 8}" fill="#0a9f68" font-size="9" text-anchor="middle">${i + 1}</text>`;',
     '`<text x="${x + bw / 2}" y="${H - 8}" fill="#5f5c50" font-size="9" text-anchor="middle">${i + 1}</text>`;', 1),

    # runPair() call-site colors
    ('vbars("OUTPUT TOKENS per run (outline = total)", d.runs.map(r => r.output), "#00ff9c", labels, d.runs.map(r => r.total)) +',
     'vbars("OUTPUT TOKENS per run (outline = total)", d.runs.map(r => r.output), "#00fff2", labels, d.runs.map(r => r.total)) +', 1),
    ('vbars("CODE LINES per run", d.runs.map(r => r.linesAdded + r.linesRemoved), "#00e5ff", labels) + `</div>`;',
     'vbars("CODE LINES per run", d.runs.map(r => r.linesAdded + r.linesRemoved), "#fff200", labels) + `</div>`;', 1),

    # renderAgentMatrix(): heat cell background -> cyan tint
    ('c.commits || c.total ? `<td class="heat" style="background:rgba(0,255,156,${(0.08 + 0.7 * c.total / maxTok).toFixed(2)})" title="${esc(a)}: ${c.commits} commits, ${fmt(c.total)} total, ${fmt(c.output)} output">${c.commits} / ${short(c.total)}</td>` : `<td class="empty">-</td>`).join("") + `<td>${short(tot)}</td></tr>`;',
     'c.commits || c.total ? `<td class="heat" style="background:rgba(0,255,242,${(0.08 + 0.7 * c.total / maxTok).toFixed(2)})" title="${esc(a)}: ${c.commits} commits, ${fmt(c.total)} total, ${fmt(c.output)} output">${c.commits} / ${short(c.total)}</td>` : `<td class="empty">-</td>`).join("") + `<td>${short(tot)}</td></tr>`;', 1),

    # stacked(): branch label color cur/non-cur + n/a text
    ('g += `<text x="2" y="${y + 16}" fill="${isCur(b) ? "#00ff9c" : "#0a9f68"}" font-size="11">${esc(b.length > 24 ? b.slice(0, 23) + "…" : b)}</text>`;',
     'g += `<text x="2" y="${y + 16}" fill="${isCur(b) ? "#ff2c1c" : "#5f5c50"}" font-size="11">${esc(b.length > 24 ? b.slice(0, 23) + "…" : b)}</text>`;', 1),
    ('if (!tot) { g += `<text x="${L}" y="${y + 16}" fill="#0a9f68" font-size="11">n/a</text>`; return; }',
     'if (!tot) { g += `<text x="${L}" y="${y + 16}" fill="#5f5c50" font-size="11">n/a</text>`; return; }', 1),
]

for old, new, expected in reps:
    n = src.count(old)
    if n != expected:
        raise SystemExit(f"MISMATCH (found {n}, expected {expected}):\n{old[:120]}")
    src = src.replace(old, new)

with open(path, "w") as f:
    f.write(src)

print("all", len(reps), "replacements applied successfully")
