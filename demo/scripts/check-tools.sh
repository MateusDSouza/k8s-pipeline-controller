#!/usr/bin/env bash
# Check that every tool the lab needs is installed (macOS / Homebrew).
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"

missing=""
check() {  # check TOOL BREW_PACKAGE VERSION_COMMAND...
  local tool=$1 pkg=$2; shift 2
  if command -v "$tool" >/dev/null; then
    printf '  \033[32m✔\033[0m %-12s %s\n' "$tool" "$("$@" 2>&1 | head -1)"
  else
    printf '  \033[31m✘\033[0m %-12s missing  (brew install %s)\n' "$tool" "$pkg"
    missing="$missing $pkg"
  fi
}

log "required"
check go          go              go version
check docker      "--cask docker" docker --version
check kind        kind            kind version
check kubectl     kubernetes-cli  kubectl version --client
check kubebuilder kubebuilder     kubebuilder version
check curl        curl            curl --version
check just        just            just --version
log "optional"
if command -v tmux >/dev/null; then check tmux tmux tmux -V; else printf '  - %-12s not installed: only needed for just demo-live (brew install tmux)\n' tmux; fi
if command -v k9s >/dev/null; then check k9s k9s k9s version --short; else printf '  - %-12s not installed: pipelines pane in just demo-live (brew install k9s)\n' k9s; fi
if command -v fzf >/dev/null; then check fzf fzf fzf --version; else printf '  - %-12s not installed: interactive pickers for just scenario / just logs (brew install fzf)\n' fzf; fi

if command -v docker >/dev/null && ! docker info >/dev/null 2>&1; then
  die "Docker is installed but not running: start Docker Desktop (or colima start)"
fi
[ -z "$missing" ] || die "missing tools - run: brew install${missing}"
log "all required tools are installed"