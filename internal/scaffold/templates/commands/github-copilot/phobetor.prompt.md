---
description: Route directly to the Phobetor agent via Janus
name: drmlnd-phobetor
agent: janus
---

Delegate this request to the `janus` agent with an explicit instruction: route directly to `phobetor`, overriding Janus's own judgment about which agent fits. Janus still performs the hand-off (including its normal identity/telemetry steps) — this command just fixes the destination.
