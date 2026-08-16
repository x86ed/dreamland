// app.js — loads the graph from the server, builds the litegraph.js canvas,
// and (interactive mode only) turns user edits into POST /api/mutate calls.
//
// Design choices, made because this environment has no browser to visually
// verify drag/drop gestures in: routing EDGES use litegraph's native
// drag-to-connect (its core, best-tested mechanic — see nodes.js's
// onConnectionsChange). Hooks and skills are NOT separate connectable nodes
// — a hook binding is a lifecycle property of the agent/project it's bound
// to, not a peer relationship the way agent routing is, and modeling it as a
// wire-and-node pair produced a cluttered graph (every agent's identical
// baseline hooks fanning out as near-duplicate nodes). They render as
// litegraph button widgets directly inside the owning node, attached/detached
// by clicking, driving the same create_edge/delete_edge operations either
// way. Node CREATE/DELETE and position save go through explicit toolbar
// buttons with plain prompt()/confirm() dialogs rather than litegraph's
// generic canvas search-box/keyboard-delete affordances, which are harder to
// get exactly right without visual feedback. Every successful mutation
// triggers a full re-fetch-and-rebuild rather than an incremental
// client-side patch, consistent with "the server is the source of truth,
// nothing client-side is trusted between rebuilds" throughout this change.
(function () {
  "use strict";

  var graph = new LGraph();
  var canvasEl = document.getElementById("graph-canvas");
  var graphcanvas = new LGraphCanvas(canvasEl, graph);

  var interactive = false;
  var loadingFromServer = false;
  var nodesById = {}; // "project" | "agent:<id>" -> LGraphNode (only connectable node kinds)

  function resizeCanvas() {
    canvasEl.width = window.innerWidth;
    canvasEl.height = window.innerHeight - document.getElementById("toolbar").offsetHeight;
  }
  window.addEventListener("resize", resizeCanvas);

  function setStatus(text) {
    var el = document.getElementById("status");
    if (el) el.textContent = text;
  }

  // --- server communication ---------------------------------------------

  function apiGet(path) {
    return fetch(path).then(function (r) {
      if (!r.ok) throw new Error(path + ": " + r.status);
      return r.json();
    });
  }

  function apiMutate(ops) {
    return fetch("/api/mutate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(ops),
    }).then(function (r) {
      if (!r.ok) {
        return r.text().then(function (msg) {
          throw new Error(msg || (r.status + ""));
        });
      }
      return r.json();
    });
  }

  function withMutation(ops) {
    return apiMutate(ops)
      .then(function () {
        setStatus("saved");
        return loadGraph();
      })
      .catch(function (err) {
        setStatus("error: " + err.message);
        // Never trust client-side state after a failed mutation — reload
        // from the server so the canvas reflects what's actually on disk.
        return loadGraph();
      });
  }

  // --- graph construction --------------------------------------------------

  function connectGrowable(sourceNode, outputIndex, targetNode, type) {
    var idxs = Growable.slotsOfType(targetNode, type);
    var emptyIdx = -1;
    for (var i = 0; i < idxs.length; i++) {
      if (targetNode.inputs[idxs[i]].link == null) {
        emptyIdx = idxs[i];
        break;
      }
    }
    if (emptyIdx === -1) {
      // Invariant violated (shouldn't happen) — grow one defensively.
      targetNode.addInput("_", type);
      emptyIdx = targetNode.inputs.length - 1;
    }
    sourceNode.connect(outputIndex, targetNode, emptyIdx);
  }

  // attachHookSkillWidgets adds one button widget per hook (and, for an
  // agent, per attached skill) bound to nodeId, plus "+ Attach ..." buttons
  // in interactive mode. This is the whole hooks/skills-as-properties
  // rendering this file's top comment describes — no separate canvas nodes.
  // Widget label text is left to litegraph's own layout otherwise, which
  // doesn't wrap and doesn't respect a wider node.size for text width — long
  // hook commands broke out of the node's drawn box. Truncating here is
  // simpler and more reliable than fighting litegraph's internal
  // computeSize/button-drawing logic to make it wrap or auto-widen instead.
  var WIDGET_LABEL_MAX = 36;
  function truncateLabel(text) {
    if (text.length <= WIDGET_LABEL_MAX) return text;
    return text.slice(0, WIDGET_LABEL_MAX - 1) + "…";
  }

  function attachHookSkillWidgets(node, nodeKind, nodeId, data) {
    (data.edges || []).forEach(function (e) {
      if (e.kind !== "hookbinding" || e.to !== nodeId) return;
      var hook = (data.hooks || {})[e.from];
      if (!hook) return;
      var label = truncateLabel("🪝 " + hook.event + ": " + hook.command);
      node.addWidget("button", label, null, function () {
        if (!interactive) return;
        withMutation([{ type: "delete_edge", edgeKind: "hookbinding", to: nodeId, event: hook.event, command: hook.command }]);
      });
    });

    if (nodeKind === "agent") {
      (data.edges || []).forEach(function (e) {
        if (e.kind !== "attachment" || e.to !== nodeId) return;
        var skill = (data.skills || {})[e.from];
        if (!skill) return;
        var label = truncateLabel("🧩 " + skill.id + " (" + skill.owner + ")");
        node.addWidget("button", label, null, function () {
          if (!interactive) return;
          withMutation([{ type: "delete_edge", edgeKind: "attachment", from: skill.id, to: nodeId }]);
        });
      });
    } else if (nodeKind === "project") {
      // A skill with no attachment edge at all defaults to showing here,
      // same as hooks default to project scope — no platform file scopes a
      // skill to one specific agent today (confirmed: nothing represents
      // "which agent uses which skill" in any real config), so "available
      // project-wide" is the honest default rather than leaving it invisible
      // just because nothing has claimed it yet. Rendering-only convention —
      // no edge is fabricated in the data model. There's deliberately no
      // "project" attachment target in the Go layer (AttachSkill/DetachSkill
      // only accept a real agent id) — a skill only ever leaves this default
      // list by being explicitly attached to a specific agent, at which
      // point it shows there instead. No click action here: there's nothing
      // to detach for a skill that was never explicitly attached to
      // anything.
      var attachedSkillIds = {};
      (data.edges || []).forEach(function (e) {
        if (e.kind === "attachment") attachedSkillIds[e.from] = true;
      });
      Object.keys(data.skills || {}).sort().forEach(function (skillId) {
        if (attachedSkillIds[skillId]) return; // explicitly attached to a specific agent — shown there instead
        var skill = data.skills[skillId];
        var label = truncateLabel("🧩 " + skill.id + " (project-wide, " + skill.owner + ")");
        node.addWidget("button", label, null, function () {}); // no click action — nothing to detach for a never-explicitly-attached skill
      });
    }

    if (!interactive) return;

    node.addWidget("button", "+ Attach Hook", null, function () {
      var event = prompt("Event (session_start | pre_tool_use | post_tool_use | stop | subagent_start | subagent_stop):", "stop");
      var command = event && prompt("Command:");
      if (!event || !command) return;
      withMutation([{ type: "create_edge", edgeKind: "hookbinding", to: nodeId, event: event, command: command }]);
    });

    if (nodeKind === "agent") {
      node.addWidget("button", "+ Attach Skill", null, function () {
        var skillId = prompt("Skill id to attach:");
        if (!skillId) return;
        withMutation([{ type: "create_edge", edgeKind: "attachment", from: skillId, to: nodeId }]);
      });
    }
  }

  // computeAgentLayout arranges agents by real routing topology instead of a
  // flat alphabetical row, so distinct flows (e.g. nyx->morpheus->phobetor
  // vs. hypnos->phobetor->phantasos) visually branch instead of collapsing
  // into one line. Agents are layered by longest routing-path distance from
  // an entry point (columns), stacked within a layer (rows). A genuine
  // cycle in the real data (phobetor <-> morpheus, the validate/fix-bug
  // loop) is handled by only counting the first-seen direction of any
  // mutual pair toward layering, so it can't oscillate layers upward.
  // Agents with zero routing edges at all (broad-routing agents with no
  // fixed position in the pipeline) get their own row below the layered
  // flows rather than being layered in at column 0 alongside real entry
  // points.
  function computeAgentLayout(agentIds, edges) {
    var routingEdges = (edges || []).filter(function (e) { return e.kind === "routing"; });
    var degree = {};
    agentIds.forEach(function (id) { degree[id] = 0; });
    routingEdges.forEach(function (e) {
      if (degree[e.from] !== undefined) degree[e.from]++;
      if (degree[e.to] !== undefined) degree[e.to]++;
    });

    var connected = agentIds.filter(function (id) { return degree[id] > 0; });
    var disconnected = agentIds.filter(function (id) { return degree[id] === 0; });

    // Mutual pairs (A->B and B->A both exist, e.g. phobetor <-> morpheus for
    // the validate/fix-bug loop) would otherwise oscillate each other's
    // layer upward every pass. Only the first-seen direction of a mutual
    // pair counts for layering; the reverse edge still renders as a link
    // (buildGraph draws straight from data.edges, not from this list) but
    // doesn't push columns outward.
    var layoutEdges = [];
    var seenPair = {};
    routingEdges.forEach(function (e) {
      if (seenPair[e.to + ">" + e.from]) return;
      seenPair[e.from + ">" + e.to] = true;
      layoutEdges.push(e);
    });

    var layer = {};
    connected.forEach(function (id) { layer[id] = 0; });
    for (var iter = 0; iter < connected.length; iter++) {
      var changed = false;
      layoutEdges.forEach(function (e) {
        if (layer[e.from] === undefined || layer[e.to] === undefined) return;
        var want = layer[e.from] + 1;
        if (want > layer[e.to] && want <= connected.length) {
          layer[e.to] = want;
          changed = true;
        }
      });
      if (!changed) break;
    }

    var byLayer = {};
    connected.forEach(function (id) {
      var l = layer[id];
      (byLayer[l] = byLayer[l] || []).push(id);
    });
    Object.keys(byLayer).forEach(function (l) { byLayer[l].sort(); });

    // baseY=560: Project node (fixed at x=80,y=80) can grow tall with many
    // project-scoped hooks/skills widgets (10 in this repo alone); layer-0
    // agents share Project's x=80 column, so they need headroom below it
    // rather than colliding at a shallower y.
    var colSpacing = 280, rowSpacing = 140, baseX = 80, baseY = 560;
    var positions = {};
    var layerKeys = Object.keys(byLayer).sort(function (a, b) { return a - b; });
    layerKeys.forEach(function (l) {
      byLayer[l].forEach(function (id, row) {
        positions[id] = [baseX + Number(l) * colSpacing, baseY + row * rowSpacing];
      });
    });

    var maxRows = layerKeys.reduce(function (m, l) { return Math.max(m, byLayer[l].length); }, 1);
    var disconnectedY = baseY + maxRows * rowSpacing + 120;
    disconnected.sort().forEach(function (id, i) {
      positions[id] = [baseX + i * colSpacing, disconnectedY];
    });

    return positions;
  }

  function buildGraph(data) {
    loadingFromServer = true;
    graph.clear();
    nodesById = {};

    var project = new DreamlandNodes.ProjectNode();
    project.pos = [80, 80];
    attachHookSkillWidgets(project, "project", "project", data);
    graph.add(project);
    nodesById["project"] = project;

    var agentIds = Object.keys(data.agents || {}).sort();
    var layoutPositions = computeAgentLayout(agentIds, data.edges);
    agentIds.forEach(function (id) {
      var a = data.agents[id];
      var node = new DreamlandNodes.AgentNode(a);
      var defaultPos = layoutPositions[id] || [80, 560];
      node.pos = [a.posX || defaultPos[0], a.posY || defaultPos[1]];
      attachHookSkillWidgets(node, "agent", id, data);
      graph.add(node);
      nodesById["agent:" + id] = node;
    });

    (data.edges || []).forEach(function (e) {
      if (e.kind !== "routing") return; // hookbinding/attachment render as widgets, not links
      var fromNode = nodesById["agent:" + e.from];
      var toNode = nodesById["agent:" + e.to];
      if (fromNode && toNode) connectGrowable(fromNode, 0, toNode, "routing");
    });

    loadingFromServer = false;
    var hookCount = Object.keys(data.hooks || {}).length;
    var skillCount = Object.keys(data.skills || {}).length;
    setStatus(agentIds.length + " agent(s), " + hookCount + " hook(s), " + skillCount + " skill(s)");
  }

  function loadGraph() {
    return apiGet("/api/graph").then(buildGraph).then(loadStatus);
  }

  // --- task/change status overlay -------------------------------------------
  //
  // Real-data scope note (see cmd/status.go's StatusResponse doc comment):
  // there's no per-task agent-assignment data anywhere in this system, only
  // a session-wide "current agent" (from .dreamland-session.json) and
  // OpenSpec's own change-level task-completion counts. This highlights the
  // one current-agent node and shows change progress as a panel — it does
  // not attempt a specific-task-to-specific-agent mapping, because that data
  // doesn't exist to map from.

  function applyStatus(status) {
    var panel = document.getElementById("status-panel");
    var changes = status.changes || [];
    if (changes.length === 0) {
      panel.textContent = "";
    } else {
      panel.textContent = changes
        .map(function (c) {
          return c.name + " (" + c.completedTasks + "/" + c.totalTasks + ", " + c.status + ")";
        })
        .join("  ·  ");
    }

    Object.keys(nodesById).forEach(function (key) {
      if (key.indexOf("agent:") !== 0) return;
      var node = nodesById[key];
      var agentId = key.slice(6);
      if (status.currentAgent && agentId === status.currentAgent) {
        node.color = "#4a3a00";
        node.boxcolor = "#ffcc00";
        node.title = agentId + " ● active";
      } else {
        node.title = agentId;
      }
    });
    graph.setDirtyCanvas(true, true);
  }

  function loadStatus() {
    return apiGet("/api/status")
      .then(applyStatus)
      .catch(function () {
        // Status is best-effort telemetry — a fetch failure shouldn't break
        // the graph view itself.
      });
  }

  // --- edge changes from user interaction (drag-connect on routing only) ---

  window.DreamlandApp = {
    // Called by nodes.js's onConnectionsChange after Growable bookkeeping.
    // Only fires a mutation for a *user*-driven connect — never during
    // buildGraph's own programmatic wiring (loadingFromServer), and never in
    // view mode (whose server has no /api/mutate route anyway). Routing is
    // the only edge kind left that's a real litegraph connection; hooks and
    // skills are widget interactions (attachHookSkillWidgets), not this path.
    onEdgeChanged: function (targetNode, slotType, isConnected, ioSlot, linkInfo) {
      if (loadingFromServer || !interactive) return;
      if (!isConnected) return; // disconnects go through "Remove Routing Edge" instead, not this hook
      if (slotType !== "routing") return;

      var sourceNode = linkInfo && graph.getNodeById(linkInfo.origin_id);
      if (!sourceNode) return;

      var targetId = idOf(targetNode);
      var sourceId = idOf(sourceNode);
      if (!targetId || !sourceId) return;

      withMutation([{ type: "create_edge", edgeKind: "routing", from: sourceId.id, to: targetId.id }]);
    },
  };

  function idOf(node) {
    for (var key in nodesById) {
      if (nodesById[key] === node) {
        var parts = key.split(":");
        return { scope: parts[0], id: parts[1] || parts[0] };
      }
    }
    return null;
  }

  // --- toolbar (interactive mode only) -------------------------------------

  function setupToolbar() {
    var toolbar = document.getElementById("toolbar");
    if (!interactive) {
      toolbar.innerHTML =
        '<span id="status"></span> <em>(read-only — /hypnos-interactive to edit)</em>' +
        ' <span id="status-panel"></span>';
      return;
    }

    toolbar.innerHTML =
      '<button id="btn-create-agent">Create Agent</button> ' +
      '<button id="btn-create-skill">Create Skill</button> ' +
      '<button id="btn-add-route">Add Routing Edge</button> ' +
      '<button id="btn-remove-route">Remove Routing Edge</button> ' +
      '<button id="btn-delete-agent">Delete Agent</button> ' +
      '<button id="btn-save-positions">Save Positions</button> ' +
      '<span id="status"></span> <span id="status-panel"></span>';

    document.getElementById("btn-create-agent").onclick = function () {
      var id = prompt("New agent id (kebab-case):");
      if (!id) return;
      var description = prompt("Description:") || "";
      var tier = prompt('Tier ("router-excluded" | "full-edit" | "write-only-no-edit"):', "full-edit") || "full-edit";
      withMutation([{ type: "create_node", kind: "agent", id: id, description: description, tier: tier }]);
    };

    document.getElementById("btn-create-skill").onclick = function () {
      var id = prompt("New skill id (kebab-case):");
      if (!id) return;
      var description = prompt("Description:") || "";
      withMutation([{ type: "create_node", kind: "skill", id: id, description: description }]);
    };

    document.getElementById("btn-add-route").onclick = function () {
      var from = prompt("Source agent id:");
      var to = from && prompt("Target agent id:");
      if (!from || !to) return;
      withMutation([{ type: "create_edge", edgeKind: "routing", from: from, to: to }]);
    };

    document.getElementById("btn-remove-route").onclick = function () {
      var from = prompt("Source agent id:");
      var to = from && prompt("Target agent id:");
      if (!from || !to) return;
      withMutation([{ type: "delete_edge", edgeKind: "routing", from: from, to: to }]);
    };

    document.getElementById("btn-delete-agent").onclick = function () {
      var id = prompt("Agent id to delete:");
      if (!id) return;
      if (!confirm('Delete agent "' + id + '" on every installed platform?')) return;
      withMutation([{ type: "delete_node", kind: "agent", id: id }]);
    };

    document.getElementById("btn-save-positions").onclick = function () {
      var ops = [];
      Object.keys(nodesById).forEach(function (key) {
        if (key.indexOf("agent:") !== 0) return;
        var node = nodesById[key];
        ops.push({ type: "update_node", kind: "agent", id: key.slice(6), posX: node.pos[0], posY: node.pos[1] });
      });
      if (ops.length === 0) return;
      withMutation(ops);
    };
  }

  // --- live refresh (SSE) ---------------------------------------------------

  function subscribeEvents() {
    var es = new EventSource("/api/events");
    var pending = null;
    es.onmessage = function () {
      // Coalesce bursts of refresh events into a single reload.
      if (pending) return;
      pending = setTimeout(function () {
        pending = null;
        loadGraph();
      }, 150);
    };
    es.onerror = function () {
      // EventSource auto-reconnects; nothing to do here.
    };
  }

  // --- boot ------------------------------------------------------------------

  resizeCanvas();
  apiGet("/api/mode")
    .then(function (mode) {
      interactive = !!mode.interactive;
      graphcanvas.read_only = !interactive;
      setupToolbar();
      return loadGraph();
    })
    .then(subscribeEvents)
    .catch(function (err) {
      setStatus("failed to load: " + err.message);
    });

  graph.start();
})();
