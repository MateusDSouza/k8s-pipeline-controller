#!/usr/bin/env bash
# Delete all pipelines; their Jobs and pods are garbage-collected through ownerReferences.
# shellcheck source=demo/scripts/lib.sh
source "$(dirname "$0")/lib.sh"
require kubectl
use_context
log "deleting all pipelines"
kubectl delete pipelines --all -n "$NAMESPACE" --wait=true
kubectl wait jobs -n "$NAMESPACE" -l "$PIPELINE_LABEL" --for=delete --timeout=60s >/dev/null 2>&1 || true
kubectl get pipelines,jobs,pods -n "$NAMESPACE"
