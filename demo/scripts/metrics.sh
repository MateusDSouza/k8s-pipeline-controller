#!/usr/bin/env bash
# Show the controller's pipeline metrics. --watch refreshes every 2 seconds.
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"
require curl

show() {
  local out
  out=$(curl -sf "$METRICS_URL") || { echo "controller not reachable on $METRICS_URL (just controller)"; return; }
  echo "$out" | grep -E '^pipeline_controller_pipelines_(started|finished)_total' || echo "(no pipelines yet)"
  echo
  echo "$out" | grep -E '^pipeline_controller_pipeline_duration_seconds_(sum|count)' | sort || true
  echo "$out" | awk -F'[{} ]' '/^pipeline_controller_pipeline_duration_seconds_(sum|count)\{/ {
      split($2, l, "\""); v[l[2], $1 ~ /_sum$/ ? "sum" : "count"] = $NF; p[l[2]] = 1 }
    END { for (k in p) if (v[k, "count"] > 0) printf "average duration %s: %.1fs\n", k, v[k, "sum"] / v[k, "count"] }'
}

if [ "${1:-}" = "--watch" ]; then
  while true; do
    clear
    log "pipeline metrics ($METRICS_URL) - $(date +%T)"
    show
    sleep 2
  done
else
  show
fi
