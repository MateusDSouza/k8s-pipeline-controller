#!/usr/bin/env bash
# Try a released version of the operator end to end, leaving nothing behind:
#   check tools -> temp folder -> kind cluster -> install the release -> run a pipeline
#   -> wait for it -> show the result -> delete the cluster and the folder.
#
#   ./demo/try-release.sh                 # latest release
#   VERSION=v0.2.0 ./demo/try-release.sh  # a specific release
#   KEEP=1 ./demo/try-release.sh          # keep the cluster to look around (prints how to delete it)
#
# Needs only docker, kind, kubectl and curl. Your kubeconfig is not touched: the cluster's
# credentials live in the temp folder.
set -euo pipefail

REPO="MateusDSouza/k8s-pipeline-controller"
VERSION="${VERSION:-latest}"
CLUSTER="pipeline-try-$$"
TIMEOUT="${TIMEOUT:-300}"
NS_OPERATOR="k8s-pipeline-controller-system"
if [ "$VERSION" = latest ]; then
  INSTALL_URL="https://github.com/$REPO/releases/latest/download/install.yaml"
else
  INSTALL_URL="https://github.com/$REPO/releases/download/$VERSION/install.yaml"
fi

log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

# 1. Tools
log "checking tools"
for tool in docker kind kubectl curl; do
  command -v "$tool" >/dev/null || die "'$tool' is not installed"
done
docker info >/dev/null 2>&1 || die "Docker is not running"

# 2. Temp folder, removed on exit together with the cluster (also on Ctrl+C or an error)
WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/pipeline-try.XXXXXX")"
export KUBECONFIG="$WORKDIR/kubeconfig"
cleanup() {
  status=$?
  if [ "${KEEP:-}" = 1 ]; then
    log "KEEP=1: cluster '$CLUSTER' kept. Use it with: export KUBECONFIG=$KUBECONFIG"
    log "delete it later with: kind delete cluster --name $CLUSTER && rm -rf $WORKDIR"
  else
    log "deleting cluster '$CLUSTER' and $WORKDIR"
    kind delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true
    rm -rf "$WORKDIR"
  fi
  exit "$status"
}
trap cleanup EXIT
log "working in $WORKDIR"

# 3. Cluster
log "creating kind cluster '$CLUSTER'"
kind create cluster --name "$CLUSTER" --kubeconfig "$KUBECONFIG" --wait 120s

# 4. Operator
log "installing the operator ($VERSION) from $INSTALL_URL"
curl -fsSL "$INSTALL_URL" -o "$WORKDIR/install.yaml" || die "could not download $INSTALL_URL"
kubectl apply -f "$WORKDIR/install.yaml" >/dev/null
kubectl wait deployment --all -n "$NS_OPERATOR" --for=condition=Available --timeout="${TIMEOUT}s"

# 5. A pipeline: two jobs, the first passes a file to the second through /workspace
log "creating a template and a two-job pipeline"
cat > "$WORKDIR/pipeline.yaml" <<'EOF'
apiVersion: ci.example.com/v1alpha1
kind: PipelineJobTemplate
metadata:
  name: compile-and-package
spec:
  steps:
    - name: compile
      image: busybox:1.37
      command: ["sh", "-c", "echo binary > /workspace/app && echo compiled"]
    - name: package
      image: busybox:1.37
      command: ["sh", "-c", "ls /workspace && echo packaged"]
---
apiVersion: ci.example.com/v1alpha1
kind: PipelineJobTemplate
metadata:
  name: ship
spec:
  steps:
    - name: ship
      image: busybox:1.37
      command: ["sh", "-c", "echo shipped"]
---
apiVersion: ci.example.com/v1alpha1
kind: Pipeline
metadata:
  name: hello-pipeline
spec:
  jobs:
    - {name: build, templateRef: compile-and-package}
    - {name: deploy, templateRef: ship}
EOF
kubectl apply -f "$WORKDIR/pipeline.yaml"

# 6. Wait until it finishes (Succeeded or Failed)
log "waiting for the pipeline to finish (up to ${TIMEOUT}s)"
phase=""
for ((i = 0; i < TIMEOUT; i += 2)); do
  phase=$(kubectl get pipeline hello-pipeline -o jsonpath='{.status.phase}' 2>/dev/null || true)
  printf '\r  %3ss  phase: %-10s' "$i" "${phase:-Pending}"
  case "$phase" in Succeeded | Failed) break ;; esac
  sleep 2
done
echo

# 7. Result
kubectl get pipeline hello-pipeline
kubectl get pipeline hello-pipeline -o jsonpath='{range .status.jobs[*]}  {.name}{"\t"}{.phase}{"\t"}{.jobName}{"\n"}{end}'
for pod in $(kubectl get pods -l ci.example.com/pipeline=hello-pipeline -o name); do
  for c in $(kubectl get "$pod" -o jsonpath='{.spec.initContainers[*].name} {.spec.containers[*].name}'); do
    printf '\033[1;34m--\033[0m %s / %s: ' "${pod#pod/}" "$c"
    kubectl logs "$pod" -c "$c" | tr '\n' ' '
    echo
  done
done

[ "$phase" = Succeeded ] || die "pipeline did not succeed (phase: '${phase:-none}')"
log "pipeline succeeded"