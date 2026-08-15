// app.js — loads the graph from the server, builds the litegraph.js canvas,
// and (interactive mode only) turns user edits into POST /api/mutate calls.
//
// Design choice, made because this environment has no browser to visually
// verify drag/drop gestures in: routing/hook/skill EDGES use litegraph's
// native drag-to-connect (its core, best-tested mechanic — see nodes.js's
// onConnectionsChange hooks). Node CREATE/DELETE and position save go through
// explicit toolbar buttons with plain prompt()/confirm() dialogs instead of
// litegraph's generic canvas search-box/keyboard-delete affordances, which
// are harder to get exactly right without visual feedback. Every successful
// mutation triggers a full re-fetch-and-rebuild rather than an incremental
// client-side patch, consistent with the "server is the source of truth,
// nothing client-side is trusted between rebuilds" design throughout this
// change.
(function () {
  "use strict";

  var graph = new LGraph();
  var canvasEl = document.getElementById("graph-canvas");
  var graphcanvas = new LGraphCanvas(canvasEl, graph);

  var interactive = false;
  var loadingFromServer = false;
  var nodesById = {}; // "project" | "agent:<id>" | "hook:<id>" | "skill:<id>" -> LGraphNode

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

  function gridPos(index) {
    var col = index % 6;
    var row = Math.floor(index / 6);
    return [80 + col * 220, 400 + row * 140];
  }

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

  function buildGraph(data) {
    loadingFromServer = true;
    graph.clear();
    nodesById = {};

    var project = new DreamlandNodes.ProjectNode();
    project.pos = [80, 80];
    graph.add(project);
    nodesById["project"] = project;

    var agentIds = Object.keys(data.agents || {}).sort();
    agentIds.forEach(function (id, i) {
      var a = data.agents[id];
      var node = new DreamlandNodes.AgentNode(a);
      node.pos = [a.posX || 80 + i * 220, a.posY || 240];
      graph.add(node);
      nodesById["agent:" + id] = node;
    });

    var hookIds = Object.keys(data.hooks || {}).sort();
    hookIds.forEach(function (id, i) {
      var h = data.hooks[id];
      var node = new DreamlandNodes.HookNode(h);
      node.pos = gridPos(i);
      graph.add(node);
      nodesById["hook:" + id] = node;
    });

    var skillIds = Object.keys(data.skills || {}).sort();
    skillIds.forEach(function (id, i) {
      var s = data.skills[id];
      var node = new DreamlandNodes.SkillNode(s);
      var p = gridPos(i);
      node.pos = [p[0], p[1] + 700];
      graph.add(node);
      nodesById["skill:" + id] = node;
    });

    (data.edges || []).forEach(function (e) {
      var fromNode, toNode, outputIndex = 0;
      if (e.kind === "routing") {
        fromNode = nodesById["agent:" + e.from];
        toNode = nodesById["agent:" + e.to];
        if (fromNode && toNode) connectGrowable(fromNode, 0, toNode, "routing");
      } else if (e.kind === "hookbinding") {
        fromNode = nodesById["hook:" + e.from];
        toNode = e.to === "project" ? nodesById["project"] : nodesById["agent:" + e.to];
        if (fromNode && toNode) connectGrowable(fromNode, 0, toNode, "hookbinding");
      } else if (e.kind === "attachment") {
        fromNode = nodesById["skill:" + e.from];
        toNode = nodesById["agent:" + e.to];
        if (fromNode && toNode) connectGrowable(fromNode, 0, toNode, "attachment");
      }
    });

    loadingFromServer = false;
    setStatus(agentIds.length + " agent(s), " + hookIds.length + " hook(s), " + skillIds.length + " skill(s)");
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

  // --- edge changes from user interaction (drag-connect / disconnect) -----

  window.DreamlandApp = {
    // Called by nodes.js's onConnectionsChange after Growable bookkeeping.
    // Only fires a mutation for a *user*-driven connect/disconnect — never
    // during buildGraph's own programmatic wiring (loadingFromServer), and
    // never in view mode (whose server has no /api/mutate route anyway).
    onEdgeChanged: function (targetNode, slotType, isConnected, ioSlot, linkInfo) {
      if (loadingFromServer || !interactive) return;
      if (!isConnected) return; // disconnects are handled via explicit "Detach" controls below, not this hook — see note in index.html
      var kind = slotType === "routing" ? "routing" : slotType === "hookbinding" ? "hookbinding" : slotType === "attachment" ? "attachment" : null;
      if (!kind) return;

      var sourceNode = linkInfo && graph.getNodeById(linkInfo.origin_id);
      if (!sourceNode) return;

      var targetId = idOf(targetNode);
      var sourceId = idOf(sourceNode);
      if (!targetId || !sourceId) return;

      if (kind === "routing") {
        withMutation([{ type: "create_edge", edgeKind: "routing", from: sourceId.id, to: targetId.id }]);
      } else if (kind === "attachment") {
        withMutation([{ type: "create_edge", edgeKind: "attachment", from: sourceId.id, to: targetId.id }]);
      } else if (kind === "hookbinding") {
        var hook = sourceNode.properties;
        withMutation([{ type: "create_edge", edgeKind: "hookbinding", to: targetId.id, event: hook.event, command: hook.command }]);
      }
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
      '<button id="btn-attach-hook">Attach Hook</button> ' +
      '<button id="btn-detach-hook">Detach Hook</button> ' +
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

    document.getElementById("btn-attach-hook").onclick = function () {
      var target = prompt('Target ("project" or an agent id):', "project");
      if (!target) return;
      var event = prompt('Event (session_start | pre_tool_use | post_tool_use | stop | subagent_start | subagent_stop):', "stop");
      var command = event && prompt("Command:");
      if (!event || !command) return;
      withMutation([{ type: "create_edge", edgeKind: "hookbinding", to: target, event: event, command: command }]);
    };

    document.getElementById("btn-detach-hook").onclick = function () {
      var target = prompt('Target ("project" or an agent id):', "project");
      if (!target) return;
      var event = prompt("Event:");
      var command = event && prompt("Command:");
      if (!event || !command) return;
      withMutation([{ type: "delete_edge", edgeKind: "hookbinding", to: target, event: event, command: command }]);
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
