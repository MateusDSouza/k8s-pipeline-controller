#!/usr/bin/env bash
# Full demo: start all scenarios at the same time, wait for every pipeline, print a summary and
# check that the metrics moved by exactly what the pipelines' statuses say.
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"
require kubectl curl
use_context
wait_for_controller

log "clean slate: deleting existing pipelines"
kubectl delete pipelines --all -n "$NAMESPACE" --wait=true >/dev/null

started_m='pipeline_controller_pipelines_started_total'
ok_m='pipeline_controller_pipelines_finished_total{phase="Succeeded"}'
failed_m='pipeline_controller_pipelines_finished_total{phase="Failed"}'
s0=$(metric "$started_m"); ok0=$(metric "$ok_m"); f0=$(metric "$failed_m")

log "templates"
kubectl apply -f "$SCENARIOS/templates.yaml"
log "starting all scenarios at once"
pipelines=()
for f in "$SCENARIOS"/[0-9]*.yaml; do
  kubectl apply -f "$f"
  while IFS= read -r p; do pipelines+=("$p"); done < <(pipelines_in "$f")
done

[ ${#pipelines[@]} -gt 0 ] || die "no pipelines found in $SCENARIOS/"
log "waiting for ${#pipelines[@]} pipelines"
for p in "${pipelines[@]}"; do wait_finished "$p"; done
sleep 1   # metrics are recorded right after the status write

echo
kubectl get pipelines -n "$NAMESPACE"
echo
for p in "${pipelines[@]}"; do show_pipeline "$p"; done

# Expected metric increases, computed from the pipelines' own statuses.
want_ok=0; want_failed=0
for p in "${pipelines[@]}"; do
  case "$(kubectl get pipeline "$p" -n "$NAMESPACE" -o jsonpath='{.status.phase}')" in
    Succeeded) want_ok=$((want_ok + 1)) ;;
    Failed) want_failed=$((want_failed + 1)) ;;
  esac
done
echo
log "metrics (increase during this demo)"
report() {  # report LABEL BEFORE NOW EXPECTED
  local got; got=$(awk -v a="$3" -v b="$2" 'BEGIN {print a - b}')
  if [ "$got" = "$4" ]; then printf '  \033[32m✔\033[0m %-22s +%s\n' "$1" "$got"
  else printf '  \033[33m!\033[0m %-22s +%s (expected +%s)\n' "$1" "$got" "$4"; fi
}
report "pipelines started"   "$s0"  "$(metric "$started_m")" "${#pipelines[@]}"
report "finished Succeeded"  "$ok0" "$(metric "$ok_m")"      "$want_ok"
report "finished Failed"     "$f0"  "$(metric "$failed_m")"  "$want_failed"
echo
log "details: just metrics | just logs <name> | just controller-logs"
