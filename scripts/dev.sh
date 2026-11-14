#!/usr/bin/env bash
# Local dev loop on kind for the pipeline controller.
#
#   scripts/dev.sh up      create the kind cluster (if missing), switch context, install the CRDs
#   scripts/dev.sh run     run the controller locally against the cluster (blocks; Ctrl+C to stop)
#   scripts/dev.sh demo    (re)create the sample pipeline, wait for it, print each step's logs
#   scripts/dev.sh down    delete the kind cluster
#
# Run from the repo root. Override the cluster name with CLUSTER=... scripts/dev.sh up
set -euo pipefail

CLUSTER="${CLUSTER:-pipeline-dev}"
CONTEXT="kind-${CLUSTER}"
PIPELINE="release-1"            # metadata.name in config/samples/ci_v1alpha1_pipeline.yaml
TIMEOUT="${TIMEOUT:-120}"       # seconds to wait for the demo job

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
  log "running the controller against $CONTEXT (Ctrl+C to stop)"
  exec make run
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

  log "waiting for the Job to finish"
  kubectl wait job -l "ci.example.com/pipeline=$PIPELINE" --for=condition=complete --timeout="${TIMEOUT}s" \
    || log "Job did not complete (failed or timed out) - showing what we have"

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

cmd_down() {
  require kind
  log "deleting kind cluster '$CLUSTER'"
  kind delete cluster --name "$CLUSTER"
}

case "${1:-}" in
  up)   cmd_up ;;
  run)  cmd_run ;;
  demo) cmd_demo ;;
  down) cmd_down ;;
  *)    sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'; exit 1 ;;
esac