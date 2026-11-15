#!/usr/bin/env bash
# One-time setup for the demo, from a fresh clone: installs the missing tools (Homebrew, asks
# first), creates the kind cluster and installs the CRDs. Safe to re-run. Needs only bash.
#
#   ./demo/setup.sh
#
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/scripts/lib.sh"

# tool:homebrew-package. tmux, fzf and k9s are only used by the demo screens, but cheap to have.
TOOLS="go:go kind:kind kubectl:kubernetes-cli just:just tmux:tmux fzf:fzf k9s:k9s"

log "checking tools"
missing=""
for pair in $TOOLS; do
  command -v "${pair%%:*}" >/dev/null || missing="$missing ${pair#*:}"
done

if [ -n "$missing" ]; then
  command -v brew >/dev/null || die "missing:$missing - install them (macOS: https://brew.sh) and re-run"
  printf 'Missing:%s\nInstall them with Homebrew now? [y/N] ' "$missing"
  read -r answer
  case "$answer" in y | Y) ;; *) die "nothing installed - run: brew install$missing" ;; esac
  # shellcheck disable=SC2086  # one argument per package
  brew install $missing
fi

require curl make   # preinstalled on macOS (make comes with the Xcode command line tools)
command -v docker >/dev/null || die "Docker is missing: install Docker Desktop (brew install --cask docker) or colima, start it and re-run"
docker info >/dev/null 2>&1 || die "Docker is not running: start Docker Desktop (or: colima start) and re-run"
log "all tools present"

demo/scripts/cluster-up.sh

cat <<'EOF'

Setup done. Run the demo:

  just demo-live          one screen: controller log, live metrics, all 7 scenarios, k9s
                          (cleans up and closes itself 30 s after the summary)

or step by step, in two terminals:

  just controller         terminal 1: the controller (metrics on :8080)
  just demo               terminal 2: all scenarios at once, summary + metrics check
  just scenario           terminal 2: one scenario (pick with fzf), then: just logs

`just` lists everything. Remove the cluster when done: just cluster-down
EOF