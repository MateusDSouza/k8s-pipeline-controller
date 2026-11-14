# Demo of the pipeline controller on a local kind cluster. `just` lists the recipes.
# This file only handles the demo. Building, testing and deploying the operator: kubebuilder's Makefile (`make help`).

set shell := ["bash", "-c"]

default:
    @just --list --unsorted

# --- setup ---

# Check that go, docker, kind, kubectl, kubebuilder and just are installed (Homebrew)
check-tools:
    @demo/scripts/check-tools.sh

# Create the kind cluster and install/update the CRDs (safe to re-run)
cluster-up:
    @demo/scripts/cluster-up.sh

# Run the controller locally: metrics on :8080, JSON log also in .dev/controller.log
controller:
    @demo/scripts/run-controller.sh

# --- run pipelines ---

# Run one scenario, e.g. just scenario 05-three-jobs-second-fails (no name: pick with fzf)
scenario name="":
    @demo/scripts/scenario.sh {{name}}

# Run all 7 scenarios at once, print the summary and check the metrics
demo:
    @demo/scripts/demo.sh

# One screen (tmux): controller | metrics | demo | k9s; resets and closes 30s after the demo (CLOSE_AFTER=n)
demo-live:
    @demo/scripts/demo-live.sh

# --- observe ---

# Show the pipeline metrics once
metrics:
    @demo/scripts/metrics.sh

# Show the pipeline metrics, refreshing every 2 seconds
metrics-watch:
    @demo/scripts/metrics.sh --watch

# Show every step's log of a pipeline, in order, e.g. just logs two-jobs-ok (no name: pick with fzf)
logs pipeline="":
    @demo/scripts/logs.sh {{pipeline}}

# Follow the controller's JSON log (written by just controller)
controller-logs:
    @demo/scripts/logs.sh --controller

# --- clean up ---

# Delete all pipelines (their Jobs and pods are garbage-collected)
reset:
    @demo/scripts/reset.sh

# Delete the kind cluster
cluster-down:
    @demo/scripts/cluster-down.sh
