# Demo

Everything here runs the controller on a local kind cluster. The commands run from the repo root
through the [`justfile`](../justfile) (`just` lists them); the scripts in `scripts/` also run directly.
What the operator does, the scenarios and the results are described in the [main README](../README.md).

```bash
./demo/setup.sh        # once: tools (asks first), kind cluster, CRDs
just demo-live         # one screen: controller | metrics | the 7 scenarios | k9s
just burst-live        # the same screen, with 50 pipelines at once
```

## Commands

| Command                                  | What it does                                                                                         |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `just cluster-up`                        | Create the kind cluster and install or update the CRDs (safe to re-run)                              |
| `just controller`                        | Run the controller locally: metrics on `:8080`, JSON log also in `.dev/controller.log`               |
| `just scenario [name]`                   | Run one scenario from `scenarios/` (no name: pick one with fzf)                                      |
| `just demo`                              | Run all 7 scenarios at once, print the results, check the metrics against the statuses               |
| `just burst [n]`                         | Create n pipelines at once (default 50), show the work queue draining, check each is counted once    |
| `just demo-live` / `just burst-live [n]` | The demo or the burst on one tmux screen; resets and closes 30 s after the summary (`CLOSE_AFTER=n`) |
| `just metrics` / `just metrics-watch`    | Pipelines started, finished by phase, duration and average                                           |
| `just logs [pipeline]`                   | Every step's output of a pipeline, in order (no name: pick with fzf)                                 |
| `just controller-logs`                   | Follow the controller's JSON log                                                                     |
| `just reset`                             | Delete all pipelines (their Jobs and pods are garbage-collected)                                     |
| `just cluster-down`                      | Delete the kind cluster                                                                              |

## Layout

| Path         | Contents                                                                                                                  |
| ------------ | ------------------------------------------------------------------------------------------------------------------------- |
| `setup.sh`   | One-time setup from a fresh clone                                                                                         |
| `scenarios/` | `templates.yaml` (build, test, deploy succeed; broken exits 1) and the 7 scenario files, named like the integration tests |
| `scripts/`   | One script per command; `lib.sh` holds the shared settings and helpers                                                    |