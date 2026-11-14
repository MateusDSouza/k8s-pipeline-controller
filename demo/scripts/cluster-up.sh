#!/usr/bin/env bash
# Create the kind cluster (if missing), switch kubectl to it and install/update the CRDs.
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"
require docker kind kubectl make
docker info >/dev/null 2>&1 || die "Docker is not running"

if kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  log "kind cluster '$CLUSTER' already exists"
else
  log "creating kind cluster '$CLUSTER'"
  kind create cluster --name "$CLUSTER"
fi
use_context                     # before make install, so the CRDs go to this cluster
log "context: $(kubectl config current-context)"
log "installing CRDs"
make install
kubectl get crd | grep ci.example.com
log "next: just controller (terminal 1), then just demo or just scenario <name> (terminal 2)"
