---
description: Route directly to the Phantasos agent via Janus
name: drmlnd-phantasos
agent: janus
---

Delegate this request to the `janus` agent with an explicit instruction: route directly to `phantasos`, overriding Janus's own judgment about which agent fits. Janus still performs the hand-off (including its normal identity/telemetry steps) — this command just fixes the destination.
