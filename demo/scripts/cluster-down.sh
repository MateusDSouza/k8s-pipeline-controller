#!/usr/bin/env bash
# Delete the kind cluster.
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"
require kind
log "deleting kind cluster '$CLUSTER'"
kind delete cluster --name "$CLUSTER"
