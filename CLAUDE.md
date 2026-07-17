# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A kubebuilder v4 operator that queues Tekton PipelineRuns. Producers create runs paused with `spec.status: PipelineRunPending`; a `PipelineRunQueue` CR (group `edp.epam.com/v1alpha1`) selects them by label selector, derives a *lane* per run from the values of its `queueKey` labels, and the controller admits runs FIFO per lane (by clearing `spec.status`) while running count < `concurrency`, or cancels superseded ones depending on `strategy` (`Queue` / `ReplaceQueued` / `CancelInProgress`).

**Read `AGENTS.md` first** — it covers project structure, auto-generated files you must never edit, the regeneration workflow (`make manifests generate`), and design rules. This file adds commands and the big-picture control flow.

## Commands

```bash
make build             # Linux binary → dist/manager-$(GOARCH); the Dockerfile only
                       # packages it (GOARCH=amd64/arm64 overridable for multi-arch)
make test              # Unit tests via envtest (downloads K8s API+etcd binaries into bin/)
make lint-fix          # golangci-lint with auto-fix (custom build via .custom-gcl.yml)
make manifests generate # After changing api/ types or kubebuilder markers
make helm-docs         # After changing deploy-templates/values.yaml or README.md.gotmpl
make validate-docs     # CI gate: fails if docs/api.md or deploy-templates/README.md stale
make run               # Run the manager locally against current kubeconfig
make start-kind        # Optional: create the dedicated e2e Kind cluster (switches context)
make e2e               # Full Chainsaw suite against the CURRENT kubeconfig context;
                       # `make delete-kind` tears the Kind cluster down.
                       # The 00-install test provisions Tekton + the chart first, so
                       # any disposable cluster works (Kind locally, vcluster in the
                       # core-cluster pipeline). LOAD_KIND_IMAGE=false skips the
                       # docker-build/kind-load for non-Kind clusters.
```

Run a single unit test (pure-logic tests in `lane_test.go`, `strategy_test.go`, `metrics_test.go` need no cluster binaries):

```bash
go test ./internal/controller/ -run TestName -v
```

The envtest-backed controller suite (`suite_test.go`, Ginkgo) needs `KUBEBUILDER_ASSETS`; easiest is `make test`, or export it via `bin/setup-envtest use <version> --bin-dir bin -p path`.

Always run the e2e suite whole (`make e2e`) — scenarios are not meant to be
cherry-picked; each one self-provisions, so the full run is the supported path.

## Architecture

The controller is deliberately **stateless**: there is no stored queue. Every reconcile recomputes decisions from the live PipelineRun set, so restarts, out-of-band deletions, and manual cancellations converge automatically. `status.lanes` is a read-only projection for observability.

Reconcile flow, spread across `internal/controller/`:

1. `pipelinerunqueue_controller.go` — entry point. Watches `PipelineRunQueue` (generation changes only) and Tekton `PipelineRun`s, mapping each run back to the queues whose selector matches it (`mapPipelineRunToQueues`). `pipelineRunPredicate` drops watch events that can't change an admission decision (only bucket transitions and label changes pass) — if you add a field the decision depends on, update this predicate or the controller won't see the change.
2. `cache.go` — configures the manager cache; the PipelineRun informer is label-filtered so the operator doesn't cache every run in the cluster.
3. `lane.go` — classifies each matched run into a bucket (queued / occupying / ignored) via `bucketOf`, and groups runs into lanes keyed by their `queueKey` label values (`bucketLanes`). This bucket classification is shared with the watch predicate.
4. `strategy.go` — given a lane's buckets, decides which runs to admit and which to cancel per the queue's strategy.
5. Back in the controller: admission is a patch clearing `spec.status`; cancellation sets `spec.status: CancelledRunFinally`. Both patches stamp the `app.edp.epam.com/queue*` annotations (defined in `api/v1alpha1/annotations.go`) in the same request, so a run's queue history is always self-describing.
6. `metrics.go` — Prometheus gauges/counters (`tekton_pipeline_queue_*`) updated from lane state; keep in sync when changing lane/queue behavior.

RBAC is minimal by design: the operator only reads its own CRD (patching just its status) and only gets/lists/watches/patches PipelineRuns. It never installs or owns the Tekton CRDs (external type, pinned in `PROJECT`/go.mod).

## Generated-file gotchas

`make manifests` does more here than stock kubebuilder: it also copies CRDs into `deploy-templates/crds/` (the Helm chart, which is the shipped artifact) and regenerates `docs/api.md` via crdoc. `deploy-templates/README.md` is generated from `README.md.gotmpl` by helm-docs. CI enforces freshness with `make validate-docs` — never hand-edit those four outputs.
