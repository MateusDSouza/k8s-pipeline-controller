#!/usr/bin/env bash
# One screen, four panes (tmux):
#   ┌──────────────────────┬──────────────────────┐
#   │ controller (JSON log) │ live metrics         │
#   ├──────────────────────┼──────────────────────┤
#   │ the demo              │ k9s on pipelines     │
#   └──────────────────────┴──────────────────────┘
# Before: a controller left running is stopped, and the demo itself starts by deleting all pipelines.
# When the demo finishes it counts down CLOSE_AFTER seconds (default 30), runs the reset and
# closes the session. Ctrl+C during the countdown keeps everything open.
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"
SESSION="pipeline-demo"
CLOSE_AFTER="${CLOSE_AFTER:-30}"

# Internal: what runs in the demo pane.
if [ "${1:-}" = "--demo-pane" ]; then
  demo/scripts/demo.sh || warn "demo finished with errors"
  echo
  for ((i = CLOSE_AFTER; i > 0; i--)); do
    printf '\r\033[1;34m==>\033[0m reset and close in %2ds (Ctrl+C to keep everything open) ' "$i"
    sleep 1
  done
  echo
  demo/scripts/reset.sh
  tmux kill-session -t "$SESSION"
  exit 0
fi

require tmux kubectl
[ -z "${TMUX:-}" ] || die "already inside tmux - run it from a normal terminal"
use_context
free_ports   # stops a controller left running from a previous session

if command -v k9s >/dev/null; then
  watcher="k9s --context $CONTEXT -n $NAMESPACE -c pipelines"
else
  warn "k9s not installed (brew install k9s) - showing kubectl get pipelines -w instead"
  watcher="kubectl get pipelines -n $NAMESPACE -w"
fi

tmux kill-session -t "$SESSION" 2>/dev/null || true
# Pane ids (not indexes) so the layout doesn't depend on split order.
ctrl=$(tmux new-session -d -s "$SESSION" -c "$PWD" -x 240 -y 60 -P -F '#{pane_id}' \
  "demo/scripts/run-controller.sh; read -r -p 'controller stopped, press Enter'")
metrics=$(tmux split-window -h -t "$ctrl" -c "$PWD" -P -F '#{pane_id}' "demo/scripts/metrics.sh --watch")
tmux split-window -v -t "$metrics" -c "$PWD" "$watcher"
demo=$(tmux split-window -v -t "$ctrl" -c "$PWD" -P -F '#{pane_id}' "demo/scripts/demo-live.sh --demo-pane; exec \$SHELL")
tmux select-pane -t "$demo"

log "attaching to tmux session '$SESSION' (detach: Ctrl+b d, close: tmux kill-session -t $SESSION)"
tmux attach -t "$SESSION"