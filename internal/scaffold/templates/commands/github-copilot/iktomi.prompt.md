---
description: Route directly to the Iktomi agent via Janus
name: drmlnd-iktomi
agent: janus
---

Delegate this request to the `janus` agent with an explicit instruction: route directly to `iktomi`, overriding Janus's own judgment about which agent fits. Janus still performs the hand-off (including its normal identity/telemetry steps) — this command just fixes the destination.
