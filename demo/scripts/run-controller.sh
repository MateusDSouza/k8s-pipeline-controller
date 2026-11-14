#!/usr/bin/env bash
# Run the controller locally against the kind cluster: metrics on :8080, JSON logs on screen
# and in .dev/controller.log (used by just controller-logs). Blocks; Ctrl+C to stop.
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"
require go kubectl make
use_context
free_ports   # stops a controller left running from a previous session
mkdir -p "$LOG_DIR"
make manifests generate fmt vet
log "controller running against $CONTEXT, metrics on $METRICS_URL (Ctrl+C to stop)"
go run ./cmd/main.go --metrics-bind-address=":${METRICS_PORT}" --health-probe-bind-address=":${HEALTH_PORT}" --metrics-secure=false 2>&1 | tee "$CONTROLLER_LOG"