// nodes.js — the four registered litegraph.js node classes from design.md's
// "Graph Node Taxonomy": dreamland/project (singleton), dreamland/agent,
// dreamland/hook, dreamland/skill. Typed slots ("routing", "hookbinding",
// "attachment") lean on litegraph's own LiteGraph.isValidConnection(outputType,
// inputType) type check (verified in the vendored source, litegraph.min.js) —
// a routing edge genuinely cannot be dropped onto a hookbinding input; the
// canvas UI itself refuses the connection, not just application code.
//
// API calls here (registerNodeType, addInput/addOutput, onConnectionsChange
// signature) are all confirmed against litegraph.js 0.7.18's real source
// (npm-fetched, checked directly — not assumed from memory), since a wrong
// method name here fails silently in a way this environment can't catch
// without a browser.

(function () {
  "use strict";

  // initBaseNode fills in the instance state litegraph.js's own LGraphNode
  // constructor (_ctor) normally sets up — flags, mode, id, graph, connections
  // — which never runs for these classes since they're built via `new
  // DreamlandNodes.AgentNode(data)` directly rather than through
  // `LiteGraph.createNode`, the only path that back-fills these
  // (LiteGraph.createNode's own source patches in exactly these fields
  // if missing, confirmed by reading it directly). Real bug this call fixes:
  // without `this.flags` initialized, LGraphNode.prototype.getConnectionPos
  // throws "Cannot read properties of undefined (reading 'collapsed')" the
  // first time the canvas tries to draw a link — silently blanking the whole
  // canvas. Only caught by an actual browser; the headless Node verification
  // in §2.4 never exercised drawing, only connect/disconnect logic.
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
    Growable.init(this, "hooks", "hookbinding");
    this.size = [160, 60];
  }
  ProjectNode.title = "Project";
  ProjectNode.desc = "Workspace scope — the target for project-level hook bindings.";
  ProjectNode.prototype.onConnectionsChange = function (type, slotIndex, isConnected, linkInfo, ioSlot) {
    if (type !== LiteGraph.INPUT) return;
    Growable.onInputChanged(this, "hooks", "hookbinding", slotIndex, isConnected);
    if (window.DreamlandApp) window.DreamlandApp.onEdgeChanged(this, ioSlot.type, isConnected, ioSlot, linkInfo);
  };
  LiteGraph.registerNodeType("dreamland/project", ProjectNode);

  function AgentNode(data) {
    initBaseNode(this);
    this.properties = { agentId: data && data.id, tier: data && data.tier, unresolvedRouting: !!(data && data.unresolvedRouting) };
    this.title = (data && data.id) || "agent";
    this.addOutput("routes_to", "routing");
    Growable.init(this, "routed_from", "routing");
    Growable.init(this, "hooks", "hookbinding");
    Growable.init(this, "skills", "attachment");
    this.size = [180, 100];
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
    var slot = this.inputs[slotIndex];
    if (!slot) return;
    if (slot.type === "routing") {
      Growable.onInputChanged(this, "routed_from", "routing", slotIndex, isConnected);
    } else if (slot.type === "hookbinding") {
      Growable.onInputChanged(this, "hooks", "hookbinding", slotIndex, isConnected);
    } else if (slot.type === "attachment") {
      Growable.onInputChanged(this, "skills", "attachment", slotIndex, isConnected);
    }
    if (window.DreamlandApp) window.DreamlandApp.onEdgeChanged(this, slot.type, isConnected, ioSlot, linkInfo);
  };
  LiteGraph.registerNodeType("dreamland/agent", AgentNode);

  function HookNode(data) {
    initBaseNode(this);
    this.properties = { command: data && data.command, event: data && data.event, scope: data && data.scope };
    this.title = (data && data.command) || "hook";
    this.addOutput("bound_to", "hookbinding");
    this.color = "#3a2a4a";
    this.size = [200, 50];
  }
  HookNode.title = "Hook";
  HookNode.desc = "One (command, event) binding, project- or agent-scoped.";
  LiteGraph.registerNodeType("dreamland/hook", HookNode);

  function SkillNode(data) {
    initBaseNode(this);
    this.properties = { skillId: data && data.id, owner: (data && data.owner) || "external" };
    this.title = (data && data.id) || "skill";
    this.addOutput("available_to", "attachment");
    this._applyOwnerStyle();
    this.size = [180, 50];
  }
  SkillNode.title = "Skill";
  SkillNode.desc = "An installed skill — read/attach-only unless owner is dreamland.";
  SkillNode.prototype._applyOwnerStyle = function () {
    if (this.properties.owner === "dreamland") {
      this.color = "#1a4a2a";
      this.boxcolor = "#30c060";
    } else {
      // owner: external — visually locked/greyed, per design.md's
      // "read/attach-only" boundary (§9.7).
      this.color = "#3a3a3a";
      this.boxcolor = "#808080";
    }
  };
  LiteGraph.registerNodeType("dreamland/skill", SkillNode);

  window.DreamlandNodes = { ProjectNode: ProjectNode, AgentNode: AgentNode, HookNode: HookNode, SkillNode: SkillNode };
})();
