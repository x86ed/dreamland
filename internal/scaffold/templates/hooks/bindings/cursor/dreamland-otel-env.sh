#!/bin/sh
# Cursor sessionStart hook: outputs JSON env object so Cursor propagates vars to all hooks.
printf '{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"%s","OTEL_EXPORTER_OTLP_PROTOCOL":"grpc","OTEL_SERVICE_NAME":"dreamland"}}' '{{OTEL_ENDPOINT}}'
