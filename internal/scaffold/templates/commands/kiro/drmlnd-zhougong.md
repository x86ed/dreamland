---
name: drmlnd-zhougong
description: Route directly to the Zhou Gong agent via Janus
inclusion: manual
---

# Zhou Gong

Delegate this request to the `janus` agent with an explicit instruction: route directly to `zhougong`, overriding Janus's own judgment about which agent fits. Janus still performs the hand-off (including its normal identity/telemetry steps) — this command just fixes the destination.
