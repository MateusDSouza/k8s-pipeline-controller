#!/usr/bin/env bash
# Show logs.
#   demo/scripts/logs.sh <pipeline>      every step's log, in order, for each job of the pipeline
#   demo/scripts/logs.sh --controller    follow the controller log written by just controller
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"

if [ "${1:-}" = "--controller" ]; then
  [ -f "$CONTROLLER_LOG" ] || die "no $CONTROLLER_LOG yet - start the controller with: just controller"
  exec tail -n 50 -f "$CONTROLLER_LOG"
fi

require kubectl
use_context
pipeline="${1:-}"
if [ -z "$pipeline" ]; then
  available=$(kubectl get pipelines -n "$NAMESPACE" -o name 2>/dev/null | sed 's|.*/||')
  [ -n "$available" ] || die "no pipelines in the cluster - run a scenario first (just scenario)"
  if command -v fzf >/dev/null && [ -t 0 ] && [ -t 1 ]; then
    # Interactive picker; the right pane previews the pipeline's phase and jobs.
    pipeline=$(printf '%s\n' "$available" | fzf --prompt='pipeline> ' --height=60% --reverse \
      --preview "kubectl get pipeline {} -n $NAMESPACE -o jsonpath='{.status.phase}{\"\\n\\n\"}{range .status.jobs[*]}{.name}{\"\\t\"}{.phase}{\"\\n\"}{end}'") || exit 0
  else
    echo "usage: just logs <pipeline>   pipelines in the cluster:"
    printf '%s\n' "$available" | sed 's/^/  /'
    exit 0
  fi
fi
jobs=$(kubectl get pipeline "$pipeline" -n "$NAMESPACE" -o jsonpath='{range .status.jobs[*]}{.jobName}{" "}{end}')
[ -n "${jobs// /}" ] || die "pipeline '$pipeline' has no Jobs yet"
for job in $jobs; do   # in pipeline order; skipped jobs have no Job and are not listed
  pod=$(kubectl get pods -n "$NAMESPACE" -l "batch.kubernetes.io/job-name=$job" -o name | head -1)
  [ -n "$pod" ] || { warn "no pod for job $job"; continue; }
  # init containers = steps 1..n-1, main container = last step
  for c in $(kubectl get "$pod" -n "$NAMESPACE" -o jsonpath='{.spec.initContainers[*].name} {.spec.containers[*].name}'); do
    log "${pod#pod/} / step '$c'"
    kubectl logs "$pod" -n "$NAMESPACE" -c "$c" 2>&1 || true
  done
done