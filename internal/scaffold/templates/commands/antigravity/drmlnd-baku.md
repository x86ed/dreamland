---
name: drmlnd-baku
description: Route directly to the Baku agent via Janus
---

Delegate this request to the `janus` agent with an explicit instruction: route directly to `baku`, overriding Janus's own judgment about which agent fits. Janus still performs the hand-off (including its normal identity/telemetry steps) — this command just fixes the destination.
