#!/usr/bin/env bash
# Burst test: create N pipelines at once (default 50), wait until all finish, then check that the
# metrics agree with the statuses (each pipeline started and finished exactly once).
# Needs the controller running (just controller). Usage: burst.sh [N]
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"
require kubectl curl
use_context

N="${1:-50}"
case "$N" in '' | *[!0-9]* | 0) die "N must be a positive number, got '$N'" ;; esac
BURST_LABEL="ci.example.com/burst=true"
BURST_TIMEOUT="${BURST_TIMEOUT:-600}"
wait_for_controller

# metric_sum NAME FILTER: sum of every series of NAME whose labels contain FILTER (0 if none).
metric_sum() {
  curl -sf "$METRICS_URL" | awk -v m="$1{" -v f="$2" \
    'index($1, m) == 1 && index($1, f) {s += $2} END {printf "%g\n", s}'
}
STARTED='pipeline_controller_pipelines_started_total'
OK='pipeline_controller_pipelines_finished_total{phase="Succeeded"}'
FAILED='pipeline_controller_pipelines_finished_total{phase="Failed"}'
DUR_SUM='pipeline_controller_pipeline_duration_seconds_sum{phase="Succeeded"}'
DUR_COUNT='pipeline_controller_pipeline_duration_seconds_count{phase="Succeeded"}'
errors() { metric_sum controller_runtime_reconcile_errors_total 'controller="pipeline"'; }
depth()  { metric_sum workqueue_depth 'name="pipeline"'; }

log "removing pipelines from a previous burst"
kubectl delete pipelines -n "$NAMESPACE" -l "$BURST_LABEL" --ignore-not-found --wait=true >/dev/null
kubectl apply -n "$NAMESPACE" -f "$SCENARIOS/templates.yaml" >/dev/null

# Counters only go up, so compare before/after.
s0=$(metric "$STARTED"); ok0=$(metric "$OK"); f0=$(metric "$FAILED")
ds0=$(metric "$DUR_SUM"); dc0=$(metric "$DUR_COUNT"); e0=$(errors)

log "creating $N pipelines (build -> test) in one apply"
start=$(date +%s)
for i in $(seq 1 "$N"); do
  printf -- '---\napiVersion: ci.example.com/v1alpha1\nkind: Pipeline\nmetadata:\n  name: burst-%03d\n  labels: {ci.example.com/burst: "true"}\nspec:\n  jobs:\n    - {name: build, templateRef: build}\n    - {name: test, templateRef: test}\n' "$i"
done | kubectl apply -n "$NAMESPACE" -f - >/dev/null

max_depth=0
while :; do
  elapsed=$(( $(date +%s) - start ))
  done_count=$(kubectl get pipelines -n "$NAMESPACE" -l "$BURST_LABEL" \
    -o jsonpath='{range .items[*]}{.status.phase}{"\n"}{end}' | grep -cE '^(Succeeded|Failed)$' || true)
  d=$(depth)
  max_depth=$(awk -v a="$max_depth" -v b="$d" 'BEGIN {print (b > a) ? b : a}')
  printf '\r  %4ss  finished %3s/%s  work queue depth %-4s' "$elapsed" "$done_count" "$N" "$d"
  [ "$done_count" -ge "$N" ] && break
  [ "$elapsed" -ge "$BURST_TIMEOUT" ] && { echo; die "only $done_count/$N finished after ${BURST_TIMEOUT}s"; }
  sleep 1
done
echo
sleep 1   # metrics are recorded right after the status patch

delta() { awk -v a="$1" -v b="$2" 'BEGIN {printf "%g", b - a}'; }
started=$(delta "$s0" "$(metric "$STARTED")")
succeeded=$(delta "$ok0" "$(metric "$OK")")
failed=$(delta "$f0" "$(metric "$FAILED")")
errs=$(delta "$e0" "$(errors)")
avg=$(awk -v s="$(delta "$ds0" "$(metric "$DUR_SUM")")" -v c="$(delta "$dc0" "$(metric "$DUR_COUNT")")" \
  'BEGIN {if (c > 0) printf "%.1f", s / c; else print "-"}')

log "burst of $N pipelines finished in ${elapsed}s"
printf '  average pipeline duration  %ss\n' "$avg"
printf '  peak work queue depth      %s\n' "$max_depth"
printf '  reconcile errors           %s\n' "$errs"

FAILED_CHECKS=0
expect() {  # expect LABEL GOT WANT
  if [ "$2" = "$3" ]; then
    printf '  \033[32m✔\033[0m %-26s %s\n' "$1" "$2"
  else
    printf '  \033[31m✘\033[0m %-26s %s (expected %s)\n' "$1" "$2" "$3"
    FAILED_CHECKS=1
  fi
}
log "metrics vs statuses"
expect "pipelines started"   "$started"   "$N"
expect "finished Succeeded"  "$succeeded" "$N"
expect "finished Failed"     "$failed"    0
[ "$FAILED_CHECKS" = 0 ] || die "metrics don't match the pipelines (is another demo running at the same time?)"
log "every pipeline counted exactly once - clean up with: just reset"