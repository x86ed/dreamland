// growable.js — the dynamic multi-input-slot pattern design.md calls for:
// litegraph's native inputs accept exactly one incoming link each (a second
// link to the same input replaces the first), which is wrong for "many
// sources hand off to one target" (routing) or "many hooks bind to one
// agent." This keeps exactly one trailing *empty* slot of a given type on a
// node at all times: connecting to it grows a new empty slot right after;
// disconnecting a non-trailing slot removes it and closes the gap.
//
// Verified against the real litegraph.js 0.7.18 source (vendored in
// vendor/litegraph.min.js) — LGraphNode.prototype.addInput/removeInput exist
// with the signatures used here, and removeInput already reindexes link
// target_slot bookkeeping internally, so no manual index fixup is needed
// beyond re-deriving slot lists fresh after each call (never caching indices
// across a mutation).
var Growable = (function () {
  "use strict";

  function slotsOfType(node, type) {
    var out = [];
    for (var i = 0; i < node.inputs.length; i++) {
      if (node.inputs[i].type === type) out.push(i);
    }
    return out;
  }

  function ensureTrailingEmpty(node, baseName, type) {
    var idxs = slotsOfType(node, type);
    var hasEmpty = idxs.some(function (i) {
      return node.inputs[i].link == null;
    });
    if (!hasEmpty) {
      node.addInput(baseName, type);
    }
  }

  // Call from a growable-input node's onConnectionsChange when
  // type === LiteGraph.INPUT. baseName/type identify which growable group
  // slotIndex belongs to (an input's .type field is authoritative — the name
  // is display-only and may repeat across the group, e.g. "routed_from",
  // "routed_from", ...).
  function onInputChanged(node, baseName, type, slotIndex, isConnected) {
    if (isConnected) {
      ensureTrailingEmpty(node, baseName, type);
      return;
    }
    var idxs = slotsOfType(node, type);
    var pos = idxs.indexOf(slotIndex);
    if (pos !== -1 && pos !== idxs.length - 1) {
      node.removeInput(slotIndex);
    }
    ensureTrailingEmpty(node, baseName, type);
  }

  // Initializes a growable group on node with a single empty starter slot.
  function init(node, baseName, type) {
    node.addInput(baseName, type);
  }

  return { onInputChanged: onInputChanged, init: init, slotsOfType: slotsOfType };
})();
