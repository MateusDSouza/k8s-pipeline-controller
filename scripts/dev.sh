#!/usr/bin/env bash
# Local dev loop on kind for the pipeline controller.
#
#   scripts/dev.sh up      create the kind cluster (if missing), switch context, install the CRDs
#   scripts/dev.sh run     run the controller locally, metrics on :8080 (blocks; Ctrl+C to stop)
#   scripts/dev.sh demo    (re)create the sample pipeline, wait for it, print each step's logs
#   scripts/dev.sh reset   delete all pipelines (their Jobs and pods are garbage-collected)
#   scripts/dev.sh metrics run one passing + one failing pipeline and check the metrics moved
#   scripts/dev.sh down    delete the kind cluster
#
# Run from the repo root. Override the cluster name with CLUSTER=... scripts/dev.sh up
set -euo pipefail

CLUSTER="${CLUSTER:-pipeline-dev}"
CONTEXT="kind-${CLUSTER}"
PIPELINE="release-1"            # metadata.name in config/samples/ci_v1alpha1_pipeline.yaml
TIMEOUT="${TIMEOUT:-120}"       # seconds to wait for the demo job
METRICS_PORT="${METRICS_PORT:-8080}"
METRICS_URL="http://localhost:${METRICS_PORT}/metrics"

log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

require() {
  for tool in "$@"; do
    command -v "$tool" >/dev/null || die "'$tool' is not installed"
  done
}

use_context() {
  kubectl config use-context "$CONTEXT" >/dev/null || die "context $CONTEXT not found - run: scripts/dev.sh up"
}

cmd_up() {
  require docker kind kubectl make
  docker info >/dev/null 2>&1 || die "Docker is not running"

  if kind get clusters | grep -qx "$CLUSTER"; then
    log "kind cluster '$CLUSTER' already exists"
  else
    log "creating kind cluster '$CLUSTER'"
    kind create cluster --name "$CLUSTER"
  fi

  use_context   # before make install, so the CRDs go to this cluster
  log "context: $(kubectl config current-context)"

  log "installing CRDs"
  make install
  kubectl get crd | grep ci.example.com
  log "ready - next: scripts/dev.sh run (terminal 1) and scripts/dev.sh demo (terminal 2)"
}

cmd_run() {
  require kubectl make
  use_context
  log "running the controller against $CONTEXT, metrics on $METRICS_URL (Ctrl+C to stop)"
  make manifests generate fmt vet
  exec go run ./cmd/main.go --metrics-bind-address=":${METRICS_PORT}" --metrics-secure=false
}

cmd_demo() {
  require kubectl
  use_context

  # The Pipeline spec is immutable, so a re-run means delete + create.
  log "resetting pipeline '$PIPELINE'"
  kubectl delete pipeline "$PIPELINE" --ignore-not-found --wait=true
  kubectl apply -k config/samples

  log "waiting for the controller to create the Job (is 'scripts/dev.sh run' running?)"
  for ((i = 0; i < TIMEOUT; i++)); do
    kubectl get jobs -l "ci.example.com/pipeline=$PIPELINE" -o name 2>/dev/null | grep -q . && break
    sleep 1
  done
  kubectl get jobs -l "ci.example.com/pipeline=$PIPELINE" -o name | grep -q . \
    || die "no Job created after ${TIMEOUT}s - check the controller output"

  log "waiting for the pipeline to finish"
  kubectl wait pipeline "$PIPELINE" --for=jsonpath='{.status.phase}'=Succeeded --timeout="${TIMEOUT}s" \
    || log "pipeline did not succeed - showing what we have"

  kubectl get pipeline "$PIPELINE"
  kubectl get jobs,pods -l "ci.example.com/pipeline=$PIPELINE"

  # Print every container's log in order: init containers (steps 1..n-1), then the main one (last step).
  for pod in $(kubectl get pods -l "ci.example.com/pipeline=$PIPELINE" -o name); do
    for c in $(kubectl get "$pod" -o jsonpath='{.spec.initContainers[*].name} {.spec.containers[*].name}'); do
      log "$pod / step '$c'"
      kubectl logs "$pod" -c "$c" || true
    done
  done
}

cmd_reset() {
  require kubectl
  use_context
  log "deleting all pipelines (their Jobs and pods are garbage-collected)"
  kubectl delete pipelines --all --wait=true
  kubectl wait jobs -l ci.example.com/pipeline --for=delete --timeout=60s >/dev/null 2>&1 || true
  kubectl get pipelines,jobs,pods
}

# metric NAME -> current value from the controller's /metrics (0 if not exposed yet)
metric() {
  curl -sf "$METRICS_URL" | awk -v m="$1" '$1 == m {print $2; found=1} END {if (!found) print 0}'
}

FAILED=0
# check LABEL BEFORE AFTER EXPECTED_INCREASE
check() {
  local got
  got=$(awk -v a="$3" -v b="$2" 'BEGIN {print a - b}')
  if [ "$got" = "$4" ]; then
    log "OK    $1 +$got"
  else
    printf '\033[1;31mFAIL\033[0m  %s +%s (expected +%s)\n' "$1" "$got" "$4"
    FAILED=1
  fi
}

cmd_metrics() {
  require kubectl curl
  use_context
  curl -sf "$METRICS_URL" >/dev/null || die "no metrics at $METRICS_URL - is 'scripts/dev.sh run' running?"
  cmd_reset

  # Counters only go up and survive between runs, so compare before/after instead of absolute values.
  local started_metric='pipeline_controller_pipelines_started_total'
  local ok_metric='pipeline_controller_pipelines_finished_total{phase="Succeeded"}'
  local failed_metric='pipeline_controller_pipelines_finished_total{phase="Failed"}'
  local dur_metric='pipeline_controller_pipeline_duration_seconds_count{phase="Succeeded"}'
  local s0 ok0 f0 d0
  s0=$(metric "$started_metric"); ok0=$(metric "$ok_metric"); f0=$(metric "$failed_metric"); d0=$(metric "$dur_metric")

  log "running one succeeding and one failing pipeline"
  kubectl apply -k config/samples
  kubectl apply -f examples/fail-fast.yaml
  kubectl wait pipeline "$PIPELINE" --for=jsonpath='{.status.phase}'=Succeeded --timeout="${TIMEOUT}s"
  kubectl wait pipeline fail-fast --for=jsonpath='{.status.phase}'=Failed --timeout="${TIMEOUT}s"
  sleep 1   # metrics are recorded right after the status write

  check "pipelines started"           "$s0"  "$(metric "$started_metric")" 2
  check "pipelines finished Succeeded" "$ok0" "$(metric "$ok_metric")"      1
  check "pipelines finished Failed"    "$f0"  "$(metric "$failed_metric")"  1
  check "duration observed Succeeded"  "$d0"  "$(metric "$dur_metric")"     1

  kubectl get jobs -l ci.example.com/pipeline=fail-fast   # fail-fast: only the first job may exist
  [ "$FAILED" = 0 ] || die "metrics check failed"
  log "metrics check passed"
}

cmd_down() {
  require kind
  log "deleting kind cluster '$CLUSTER'"
  kind delete cluster --name "$CLUSTER"
}

case "${1:-}" in
  up)   cmd_up ;;
  run)  cmd_run ;;
  demo) cmd_demo ;;
  reset) cmd_reset ;;
  metrics) cmd_metrics ;;
  down) cmd_down ;;
  *)    sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'; exit 1 ;;
esac