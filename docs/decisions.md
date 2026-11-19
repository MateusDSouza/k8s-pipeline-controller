# Design decisions

## 1. Pipeline spec is immutable
- **Choice:** CEL rule `self == oldSelf` on `spec`.
- **Why:** changing a running pipeline would make its status meaningless; create a new Pipeline instead.

## 2. PipelineJobTemplate has no controller
- **Choice:** templates are passive data; the Pipeline controller reads them when needed.
- **Why:** nothing has to happen when a template changes. A controller would only add code, RBAC and tests.

## 3. Validation lives in the API, not in the controller
- **Choice:** kubebuilder markers (MinItems, MinLength, Enum, listType=map, CEL rules).
- **Why:** invalid objects are rejected at `kubectl apply`, before the controller sees them, so the
  reconcile logic can trust its input (jobs never empty, step names unique).

## 4. Job name length enforced by the API
- **Problem:** Kubernetes names and label values are limited to 63 characters; the Job name is
  `<pipeline>-<index>-<job>`.
- **Options:** truncate + hash in code / limit it in the API.
- **Choice:** CEL rule on the Pipeline: pipeline name + job name + 4 <= 63, with `MaxItems=50` on jobs
  (so the index is at most 2 digits).
- **Why:** an explicit limit in the API is preferable to a workaround in the code; the user gets a
  clear error at apply time. It also keeps the pipeline label value valid.
- **Trade-off:** long names are rejected instead of shortened.
- **Found by:** a unit test (`TestJobName_Length`) before it ever reached a cluster.

## 5. Each pipeline job is one Kubernetes Job; steps are containers in its pod
- **Options:** one Pod per step / one Job per step / one Job with steps as init containers.
- **Choice:** one Job; all steps but the last are init containers, the last is the main container.
- **Why:** Kubernetes already runs init containers in order and stops at the first failure;
  steps share a `/workspace` emptyDir.
- **Trade-offs:** all steps on one node, in one pod; no per-step resources or retries.

## 6. No automatic retries
- **Choice:** `backoffLimit: 0`, `restartPolicy: Never`.
- **Why:** a failing CI step should fail the job visibly, not be retried silently.

## 7. Deterministic Job names and idempotent creation
- **Why:** after a restart the controller finds the Jobs it already created; `AlreadyExists` on create
  is treated as success, so running reconcile twice is harmless.

## 8. Pipeline owns its Jobs (ownerReferences)
- **Why:** deleting a Pipeline garbage-collects its Jobs and pods (verified on kind), and
  `Owns(&batchv1.Job{})` triggers a reconcile when a Job changes.
- **Ownership check:** the controller only uses Jobs that carry the pipeline label *and* are controlled
  by it (`IsControlledBy`), so a leftover Job from a deleted pipeline with the same name is ignored.

## 9. Tests written alongside the code
- **Choice:** Go unit tests next to the code (`*_test.go`), envtest integration tests with Ginkgo.
- **Note:** envtest has no kubelet, so integration tests check the objects the controller creates,
  not that containers run. In them the test plays the Job controller (it marks Jobs succeeded or
  failed) and calls `Reconcile` step by step: deterministic, no sleeps. Running containers is
  verified on kind (`just demo`, `just burst`).
- **Layers:** unit tests for the pure functions, envtest for the reconciler against a real API server
  (7 scenarios, missing template, stale cache), and kubebuilder's e2e test, which deploys the manager
  to kind and was extended to run two pipelines in-cluster (see decision 26). GitHub Actions runs
  lint, tests, e2e and govulncheck on every push and pull request.

## 10. Reconcile = observe → decide → act → report, with a pure decision function
- **Choice:** `decide()` (in `decide.go`) gets the state of each existing Job and returns ONE action:
  create the next job / wait / fail / succeed. Reconcile observes (lists owned Jobs), calls it,
  acts, and writes the status.
- **Why:** all sequencing and fail-fast logic is a pure function, tested with a table of scenarios
  without any cluster. Reconcile stays short and only talks to the API.
- **Behaviour:** sequential (job N+1 starts only after job N succeeded) and fail-fast (a failed job
  stops the pipeline; later jobs are never created). Each reconcile does one step; Job events
  (`Owns`) trigger the next one.
- **Observed state comes from the Jobs, not from the previous status**, so the status can always be
  recomputed (e.g. after a controller restart).

## 11. Status: phase, per-job status and a Ready condition
- **Choice:** `setPhase()` is the only place where the phase changes (start/completion times,
  `Ready` condition with reason and message). Per-job status is rebuilt from the real Jobs; jobs never
  created because of an earlier failure are `Skipped`. The status is patched on the status
  subresource, and only when it changed.
- **Missing template:** not an error loop. Condition `TemplateNotFound` and a recheck every 15 s
  (`RequeueAfter`, no error, so no exponential backoff and no error logs); the pipeline starts by
  itself once the template exists. Tested in `missing_template_test.go`.
- **Terminal phases:** a Succeeded/Failed pipeline does nothing more, even if events keep arriving.

## 12. Metrics: only what the controller uniquely knows
- **Choice:** three metrics on controller-runtime's existing `/metrics` endpoint:
  `pipelines_started_total`, `pipelines_finished_total{phase}`, `pipeline_duration_seconds{phase}`
  (histogram).
- **Why:** they answer traffic (started), errors (Failed), latency (duration). Job-level metrics
  already come from kube-state-metrics, controller health from controller-runtime's built-in metrics,
  so they are not duplicated (a "jobs created" counter was removed for that reason).
- **Labels:** only `phase` (low cardinality). No pipeline name as a label: it's unbounded and would
  create a new time series per pipeline. Per-pipeline details live in the status and the logs.
- **Counters, not a "running now" gauge:** running = started − finished in PromQL, and counters stay
  correct across restarts.

## 13. Logging: structured, contextual, JSON by default
- **Choice:** logr + zap (kubebuilder default). controller-runtime passes each Reconcile a logger with
  the controller, pipeline name/namespace and a unique `reconcileID`; the controller adds key/value fields.
  JSON by default (`Development: false` in `cmd/main.go`); `--zap-devel=true` for readable local output.
- **What is logged:** events only: `created job` (job, index, template) and `pipeline phase changed`
  (from, to, reason, message). A reconcile where nothing changed logs nothing.
- **Errors:** wrapped with context (`fmt.Errorf("create job %s: %w", ...)`) and returned;
  controller-runtime logs them once and retries. They are not logged a second time.
- **Why JSON:** in production, log systems (Loki, Elasticsearch) filter by field, e.g. `to="Failed"`
  or all lines of one `reconcileID`, without fragile text parsing.

## 14. Transitions are recorded after the status is saved
- **Choice:** `recordTransition()` (in `transitions.go`) logs and updates the metrics only when the
  phase changed, and only after a successful status patch.
- **Why:** Reconcile runs many times per pipeline, but metrics and logs are about events. Comparing the
  stored phase with the new one turns states into events. If the patch fails, the retry still sees
  the old phase, so the transition is recorded on the retry, not twice.
- **Tested by:** `TestRecordTransition_CountsOnlyChanges`, and on kind by `just demo` and `just burst`,
  which compare the metric increases with the pipelines' statuses.

## 15. Status patches use an optimistic lock
- **Problem:** `r.Get` reads from the informer cache, but the status patch goes straight to the API
  server. Creating a Job triggers a new reconcile that can run before the cache has the controller's
  last status write: it saw the old phase and recorded `"" → Running` a second time (seen in the logs with two
  reconcileIDs), overcounting `pipelines_started_total`.
- **Choice:** the status patch carries the `resourceVersion` that was read
  (`client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})`). If the object changed
  since, the API server answers Conflict: nothing is written or recorded, and the reconcile requeues
  after 1 s with fresh data. A Conflict is returned as a requeue, not an error: it's an expected race.
- **Why:** it fixes the cause (acting on stale data) instead of de-duplicating counts afterwards, and it
  protects the status itself, not only the metrics.
- **Tested by:** `stale_cache_test.go`, with a client that returns an old copy of the Pipeline (failed
  before the fix, passes after), and `just burst` on kind: 50 pipelines, each counted exactly once.

## 16. Least-privilege RBAC, generated from the code
- **Choice:** `+kubebuilder:rbac` markers next to the reconciler; `make manifests` generates
  `config/rbac/role.yaml`. Templates are read-only (get, list, watch); Jobs get create plus read;
  the status is written through `pipelines/status`.
- **Why:** permissions sit beside the calls that need them and change with them. RBAC denies
  everything by default, so a missing rule fails loudly in-cluster: the e2e test runs real pipelines
  with the manager's own ServiceAccount, so a missing permission breaks CI, not a user's cluster.

## 17. Deployment defaults kept from kubebuilder
- **Choice:** the manager runs as a Deployment from `config/` (kustomize): non-root, health probes on
  `:8081`, leader election (`--leader-elect`), metrics served over HTTPS with authentication and
  authorization, set explicitly in `config/manager/manager.yaml`.
- **Why:** these are production-ready defaults; changing them would need a reason. Leader election
  makes two replicas safe (only the Lease holder reconciles). The local demo runs the controller with
  `--metrics-secure=false` on `:8080` so `curl` works.

## 18. Demo tooling separated from the operator's build tooling
- **Choice:** kubebuilder's `Makefile` stays untouched and owns everything about the operator
  (build, test, lint, manifests, deploy). A `justfile` at the repo root owns **only the demo**
  (cluster, controller, scenarios, metrics, logs, cleanup). Scripts and scenarios live in `demo/`.
- **Why:** the Makefile is generated by kubebuilder and may be regenerated by upgrades or plugins;
  keeping the demo outside it means no custom code gets overwritten, and a reviewer who knows
  kubebuilder finds the Makefile exactly as expected. The root shows the product; `demo/` shows
  how it is exercised.
- **Boundary:** demo scripts may *call* kubebuilder targets (`make install` for the CRDs,
  `make manifests generate` before running the controller) but never duplicate them.
- **Why just:** clean arguments (`just scenario 04-two-jobs-first-fails`), `just --list` as a
  self-documenting menu, no tab/space pitfalls. The scripts also run directly without just.

## 19. One script per task, shared helpers in one file
- **Choice:** `demo/scripts/` has one small script per task (check tools, cluster up/down, run the
  controller, one scenario, full demo, burst, metrics, logs, reset) and a `lib.sh` with shared
  settings and helpers (wait for a pipeline, read a metric, show a pipeline's status).
- **Why:** each script is short and readable; behaviour is consistent; settings can be overridden
  with environment variables (`TIMEOUT=300 just demo`). `lib.sh` moves to the repo root itself,
  so scripts work from any folder.
- **Portability:** written for macOS's default bash 3.2 (no `mapfile`), checked with shellcheck.

## 20. One-command setup from a fresh clone
- **Choice:** `demo/setup.sh` installs the missing tools with Homebrew (it lists them and asks first),
  checks that Docker is running, creates the kind cluster and installs the CRDs. It needs only bash,
  because `just` may not be installed yet.
- **Why:** a reviewer goes from `git clone` to `just demo-live` with two commands, and nothing is
  installed without their consent.

## 21. Demo scenarios mirror the integration tests
- **Choice:** `demo/scenarios/` has the same 7 scenarios, with the same pipeline names, as
  `pipeline_scenarios_test.go`, plus `templates.yaml` (build, test, deploy succeed; broken fails).
- **Why:** the tests prove the logic with simulated Jobs; the demo shows the same scenarios with real
  containers on kind. Same names, same story, in tests, demo and notes.
- **Images pinned** (`busybox:1.37`) so the demo behaves the same every time.

## 22. The demo verifies itself
- **Choice:** `just demo` starts all scenarios at once, waits for every pipeline, and compares the
  metric increases with what the pipelines' own statuses say (started = number of pipelines,
  Succeeded/Failed = their phases).
- **Why:** the observability requirement is demonstrated.

## 23. A burst test instead of a formal load test
- **Choice:** `just burst [n]` creates n pipelines (default 50) in one `kubectl apply`, shows the work
  queue depth while they run, and checks started = n, Succeeded = n, Failed = 0, plus reconcile errors.
- **Why:** kind on a laptop isn't a benchmark, and the challenge doesn't ask for one. A burst answers
  the questions that matter for a controller: does one worker keep up, does the queue drain, is every
  pipeline counted once under concurrency. Result: 50 pipelines in 34 s, queue peak 50 → 0, 0 errors,
  each counted once. It's also the load that exposed the stale-cache double count (decision 15).

## 24. One-command live demo that cleans up after itself
- **Choice:** `just demo-live` opens one tmux screen with four panes: the controller's log, live
  metrics, the demo, and k9s on the pipelines view (falls back to `kubectl get pipelines -w` without
  k9s). `just burst-live [n]` uses the same screen with the burst in the demo pane. After the run it
  counts down 30 s (`CLOSE_AFTER`), runs the reset and closes the session; Ctrl+C during the countdown
  keeps it open.
- **Safe start:** before starting, `free_ports` checks the controller's ports (8080 metrics, 8081
  health). A controller left running from a previous session (`go run` names it `main`) is stopped;
  any other process holding a port is reported and never killed.
- **Why:** the whole challenge shown in one command, starting from a clean state every time (the
  demo deletes all pipelines first) and leaving nothing behind. A cheat-sheet tool (navi) was
  considered and dropped: `just --list` and the fzf pickers already cover it.

## 25. Lint: kubebuilder's rules, one exception for tests
- **Choice:** golangci-lint with the scaffold configuration, except `goconst` (repeated string
  literals) in `_test.go` files.
- **Why:** in table-driven tests, inline values (`"build"`, `"test"`) are what make each case readable.
  The rule still applies to all production code.

## 26. An end-to-end test that runs pipelines in-cluster
- **Choice:** the scaffold's e2e suite (build the image, deploy the manager to kind, check metrics) was
  extended with one spec that applies the demo templates and two scenarios, `03-two-jobs-ok` and
  `04-two-jobs-first-fails`, then checks the phases, the number of Jobs created, the `Skipped` job and
  the JSON transition logs.
- **Why:** it is the only test where the controller runs as a Deployment with its generated RBAC and
  real pods run the steps. Unit tests and envtest can't catch a missing permission or a pod that never
  starts.
- **Separate namespace:** the manager's namespace enforces the `restricted` Pod Security Standard, and
  the busybox step containers don't meet it (see Known limits), so the pipelines run in their own
  namespace.
- **Reuse:** the spec applies the same YAML files as the demo, so the integration tests, the demo and
  the e2e test describe the same scenarios.

## 27. Releases from tags, not from every merge
- Release check: demo/try-release.sh installs a published release on a throwaway kind cluster and runs a pipeline. It is run by hand; running it automatically before a release is marked as latest is the next step.
- **Build time:** the first release compiled the arm64 image under QEMU emulation and took 57 min.Running the builder stage on the build platform (`FROM --platform=$BUILDPLATFORM`) and letting Go cross-compile with `GOARCH=$TARGETARCH` brought the whole workflow to under 8 min (v0.2.1), with the same images.

## 28. Vulnerability scanning with govulncheck
- **Choice:** `govulncheck` runs on every push and pull request.
- **Why:** it reports only vulnerabilities in code the program actually calls, so a finding is worth
  acting on. Its first run reported standard library issues in Go 1.26.0 and vulnerable versions of
  `golang.org/x/net`, gRPC and OpenTelemetry (indirect dependencies); they were fixed by moving to
  Go 1.26.11 and the patched module versions before the first release.

## Known limits
- **Sequential jobs only:** no parallel jobs or `dependsOn`-style graph (the challenge describes a
  sequence). All steps of a job run in one pod, on one node, with no per-step resources, timeout
  (`activeDeadlineSeconds`) or retries.
- **Releases rebuild the image on the tag** instead of promoting the image tested on `main`.
- **One reconcile worker** (controller-runtime's default): enough for the burst (the queue drains in
  seconds); `MaxConcurrentReconciles` could raise it for many more pipelines, safely, because one
  pipeline is never reconciled twice in parallel.