#!/bin/sh
# Kiro agentSpawn hook: exports OTEL env vars.
# Note: env propagation from agentSpawn to subsequent Kiro hooks is unconfirmed.
export OTEL_EXPORTER_OTLP_ENDPOINT='{{OTEL_ENDPOINT}}'
export OTEL_EXPORTER_OTLP_PROTOCOL='grpc'
export OTEL_SERVICE_NAME='dreamland'
