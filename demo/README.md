# Demo

Everything here exercises the controller on a local kind cluster. All commands run from the
repo root through the [`justfile`](../justfile) (`just` lists them); the scripts in
`demo/scripts/` also run directly.

```bash
just check-tools      # go, docker, kind, kubectl, kubebuilder, just
just cluster-up       # kind cluster + CRDs
just controller       # terminal 1: the controller (metrics on :8080)
just demo             # terminal 2: all scenarios at once, summary + metrics check
```

## Scenarios

`scenarios/templates.yaml` defines four reusable job templates: `build`, `test` and `deploy`
succeed, `broken` exits with code 1. Each scenario file uses them. The names match the
integration tests in `internal/controller/pipeline_scenarios_test.go`.

| File | Pipeline(s) | Expected |
| ---- | ----------- | -------- |
| `01-one-job-ok` | one-job-ok | Succeeded |
| `02-one-job-fail` | one-job-fail | Failed |
| `03-two-jobs-ok` | two-jobs-ok | Succeeded |
| `04-two-jobs-first-fails` | two-jobs-first-fails | Failed; deploy Skipped (never created) |
| `05-three-jobs-second-fails` | three-jobs-second-fails | Failed; deploy Skipped |
| `06-three-jobs-ok` | three-jobs-ok | Succeeded |
| `07-parallel` | parallel-a, parallel-b | a Succeeded, b Failed in its second job |

Run one with `just scenario <file name without .yaml>`, then `just logs <pipeline>` to see
each step's output in order.

## Observing

- `just metrics` / `just metrics-watch`: started, finished by phase, duration (and average).
- `just controller-logs`: the controller's JSON log; one `pipeline phase changed` line per transition.
- `just demo-live`: controller log, live metrics and the demo side by side (needs `tmux`).
