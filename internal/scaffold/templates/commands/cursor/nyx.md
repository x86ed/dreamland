---
name: drmlnd-nyx
description: Route directly to the Nyx agent via Janus
---

# Nyx

Delegate this request to the `janus` agent with an explicit instruction: route directly to `nyx`, overriding Janus's own judgment about which agent fits. Janus still performs the hand-off (including its normal identity/telemetry steps) — this command just fixes the destination.
