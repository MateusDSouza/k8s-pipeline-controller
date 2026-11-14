#!/usr/bin/env bash
# Shared settings and helpers, sourced by the other scripts. Run everything from the repo root.
# shellcheck disable=SC2034  # variables are used by the scripts that source this file
set -euo pipefail

# Always run from the repo root, wherever the script was called from.
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
SCENARIOS="demo/scenarios"

CLUSTER="${CLUSTER:-pipeline-dev}"
CONTEXT="kind-${CLUSTER}"
NAMESPACE="${NAMESPACE:-default}"
TIMEOUT="${TIMEOUT:-180}"                     # seconds to wait for pipelines / the controller
METRICS_PORT="${METRICS_PORT:-8080}"
METRICS_URL="http://localhost:${METRICS_PORT}/metrics"
HEALTH_PORT="${HEALTH_PORT:-8081}"           # controller health probe (default in cmd/main.go)
LOG_DIR=".dev"                                # local run artefacts (gitignored)
CONTROLLER_LOG="${LOG_DIR}/controller.log"
PIPELINE_LABEL="ci.example.com/pipeline"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mWARN:\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

require() {
  for tool in "$@"; do
    command -v "$tool" >/dev/null || die "'$tool' is not installed (run: just check-tools)"
  done
}

use_context() {
  kubectl config use-context "$CONTEXT" >/dev/null 2>&1 || die "context $CONTEXT not found (run: just cluster-up)"
}

controller_up() { curl -sf "$METRICS_URL" >/dev/null 2>&1; }

# free_ports: make sure the controller's ports are free. A previous local controller (go run builds
# a binary called "main") is stopped; anything else holding a port is reported, never killed.
free_ports() {
  local pids pid name
  pids=$(lsof -nP -t -iTCP:"$METRICS_PORT" -iTCP:"$HEALTH_PORT" -sTCP:LISTEN 2>/dev/null | sort -u || true)
  [ -n "$pids" ] || return 0
  for pid in $pids; do
    name=$(basename "$(ps -p "$pid" -o comm= 2>/dev/null)")
    if [ "$name" = "main" ]; then
      log "stopping the controller that is already running (pid $pid)"
      kill "$pid"
    else
      die "port $METRICS_PORT or $HEALTH_PORT is used by '$name' (pid $pid) - stop it, or use METRICS_PORT=..."
    fi
  done
  for ((i = 0; i < 10; i++)); do
    lsof -nP -t -iTCP:"$METRICS_PORT" -iTCP:"$HEALTH_PORT" -sTCP:LISTEN >/dev/null 2>&1 || return 0
    sleep 1
  done
  die "ports $METRICS_PORT/$HEALTH_PORT still in use after 10s"
}

# wait_for_controller: block until the local controller answers on its metrics endpoint.
wait_for_controller() {
  controller_up && return 0
  log "waiting for the controller on $METRICS_URL (start it with: just controller)"
  for ((i = 0; i < TIMEOUT; i++)); do
    controller_up && return 0
    sleep 1
  done
  die "controller not reachable after ${TIMEOUT}s"
}

# metric NAME -> current value from /metrics (0 if not exposed yet)
metric() {
  curl -sf "$METRICS_URL" | awk -v m="$1" '$1 == m {print $2; found=1} END {if (!found) print 0}'
}

# wait_finished NAME: wait until a pipeline is Succeeded or Failed.
wait_finished() {
  local phase=""
  for ((i = 0; i < TIMEOUT; i++)); do
    phase=$(kubectl get pipeline "$1" -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || true)
    case "$phase" in Succeeded | Failed) return 0 ;; esac
    sleep 1
  done
  die "pipeline $1 not finished after ${TIMEOUT}s (phase: '${phase}')"
}

# show_pipeline NAME: phase, Ready reason/message and the phase of every job.
show_pipeline() {
  kubectl get pipeline "$1" -n "$NAMESPACE" -o jsonpath='{.metadata.name}{": "}{.status.phase}{"  ("}{.status.conditions[?(@.type=="Ready")].reason}{": "}{.status.conditions[?(@.type=="Ready")].message}{")\n"}{range .status.jobs[*]}{"    "}{.name}{"\t"}{.phase}{"\n"}{end}'
}

# pipelines_in FILE: names of the Pipelines defined in a YAML file.
pipelines_in() {
  kubectl apply --dry-run=client -f "$1" -o name | awk -F/ '$1 ~ /^pipeline\./ {print $2}'
}