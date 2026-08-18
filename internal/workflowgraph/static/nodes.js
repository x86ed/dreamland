// nodes.js — the two connectable litegraph.js node classes: dreamland/project
// (singleton) and dreamland/agent. Routing edges (Agent.routes_to ->
// Agent.routed_from) stay as real litegraph connections — that relationship
// is genuinely peer-to-peer, one agent handing off to another, and litegraph's
// own typed-slot check (LiteGraph.isValidConnection) is exactly the right
// mechanism for it.
//
// Hooks and skills are NOT separate connectable nodes (a prior version of
// this file had dreamland/hook and dreamland/skill node classes with their
// own canvas nodes and wires — removed). A hook binding is a lifecycle
// property of the agent (or the project) it's bound to, not a relationship
// between two independent peers the way agent routing is; modeling it as a
// wire-and-node pair produced a cluttered graph (every agent's identical
// baseline hooks fanned out as near-duplicate nodes across the whole
// canvas) for something that's really just "this agent has these hooks."
// Hooks and skills now render as litegraph button widgets directly inside
// the owning Agent/Project node's body — attach/detach is a widget click,
// not a drag-connect gesture, but it drives the exact same
// create_edge/delete_edge operations against the server either way (see
// app.js's attachHookSkillWidgets).
//
// API calls here (registerNodeType, addInput/addOutput, addWidget,
// onConnectionsChange signature) are all confirmed against litegraph.js
// 0.7.18's real source (npm-fetched, checked directly — not assumed from
// memory), since a wrong method name here fails silently in a way this
// environment can't catch without a browser.

(function () {
  "use strict";

  // initBaseNode fills in the instance state litegraph.js's own LGraphNode
  // constructor (_ctor) normally sets up — flags, mode, id, graph,
  // connections — which never runs for these classes since they're built via
  // `new DreamlandNodes.AgentNode(data)` directly rather than through
  // `LiteGraph.createNode`, the only path that back-fills these
  // (LiteGraph.createNode's own source patches in exactly these fields if
  // missing, confirmed by reading it directly). Real bug this call fixes:
  // without `this.flags` initialized, LGraphNode.prototype.getConnectionPos
  // throws "Cannot read properties of undefined (reading 'collapsed')" the
  // first time the canvas tries to draw a link — silently blanking the whole
  // canvas. Only caught by an actual browser; headless Node verification
  // never exercised drawing, only connect/disconnect logic.
  function initBaseNode(node) {
    node.flags = {};
    node.mode = LiteGraph.ALWAYS;
    node.connections = [];
    node.graph = null;
    node.id = -1;
  }

  function ProjectNode() {
    initBaseNode(this);
    this.title = "Project";
    this.color = "#2b2f3a";
    this.bgcolor = "#1a1d24";
    this.size = [220, 60];
  }
  ProjectNode.title = "Project";
  ProjectNode.desc = "Workspace scope — the target for project-level hook bindings.";
  LiteGraph.registerNodeType("dreamland/project", ProjectNode);

  function AgentNode(data) {
    initBaseNode(this);
    this.properties = { agentId: data && data.id, tier: data && data.tier, unresolvedRouting: !!(data && data.unresolvedRouting) };
    this.title = (data && data.id) || "agent";
    this.addOutput("routes_to", "routing");
    this.addInput("routed_from", "routing"); // growable — Growable.onInputChanged (below) keeps one trailing empty slot
    this.size = [220, 90];
    this._applyUnresolvedStyle();
  }
  AgentNode.title = "Agent";
  AgentNode.desc = "One installed agent (id, description, tool tier, instruction body).";
  AgentNode.prototype._applyUnresolvedStyle = function () {
    if (this.properties.unresolvedRouting) {
      this.color = "#5a3a1a";
      this.boxcolor = "#e0a030";
    } else {
      this.color = "#1a3a4a";
      this.boxcolor = "#3ab0e0";
    }
  };
  AgentNode.prototype.onConnectionsChange = function (type, slotIndex, isConnected, linkInfo, ioSlot) {
    if (type !== LiteGraph.INPUT) return;
    Growable.onInputChanged(this, "routed_from", "routing", slotIndex, isConnected);
    if (window.DreamlandApp) window.DreamlandApp.onEdgeChanged(this, ioSlot.type, isConnected, ioSlot, linkInfo);
  };
  LiteGraph.registerNodeType("dreamland/agent", AgentNode);

  window.DreamlandNodes = { ProjectNode: ProjectNode, AgentNode: AgentNode };
})();
