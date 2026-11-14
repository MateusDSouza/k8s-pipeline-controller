#!/usr/bin/env bash
# Run one scenario: (re)create its pipelines, wait until they finish, show status and Jobs.
# No name: pick one interactively with fzf (or list them when fzf is not installed).
#   demo/scripts/scenario.sh 04-two-jobs-first-fails
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"
name="${1:-}"
if [ -z "$name" ]; then
  available=$(for f in "$SCENARIOS"/[0-9]*.yaml; do basename "$f" .yaml; done)
  if command -v fzf >/dev/null && [ -t 0 ] && [ -t 1 ]; then
    # Interactive picker; the right pane previews the scenario's YAML.
    name=$(printf '%s\n' "$available" | fzf --prompt='scenario> ' --height=60% --reverse \
      --preview "cat $SCENARIOS/{}.yaml") || exit 0   # Esc: nothing chosen
  else
    echo "usage: just scenario <scenario>   available:"
    printf '%s\n' "$available" | sed 's/^/  /'
    exit 0
  fi
fi
file="$SCENARIOS/${name}.yaml"
[ -f "$file" ] || die "unknown scenario '$name' (run: just scenario)"
require kubectl curl
use_context
wait_for_controller

kubectl apply -f "$SCENARIOS/templates.yaml" >/dev/null
# Pipeline specs are immutable, so a re-run means delete + create.
kubectl delete -f "$file" --ignore-not-found --wait=true >/dev/null
log "scenario $name"
kubectl apply -f "$file"

pipelines=()
while IFS= read -r p; do pipelines+=("$p"); done < <(pipelines_in "$file")
[ ${#pipelines[@]} -gt 0 ] || die "no Pipeline found in $file"
for p in "${pipelines[@]}"; do wait_finished "$p"; done
for p in "${pipelines[@]}"; do
  echo
  show_pipeline "$p"
  kubectl get jobs -n "$NAMESPACE" -l "$PIPELINE_LABEL=$p" --no-headers 2>/dev/null | sed 's/^/    job: /'
done
echo
log "step logs: just logs ${pipelines[0]}"