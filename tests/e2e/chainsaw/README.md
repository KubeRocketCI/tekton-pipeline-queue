# Chainsaw e2e tests

Chainsaw-based end-to-end scenarios exercising the queue controller against
a real Tekton Pipelines install. Run via `make test-e2e` from the repo root
(builds the operator image, loads it into a Kind cluster, installs Tekton
and the Helm chart, then runs Chainsaw).

Each directory under this folder is one scenario:

- `fifo-admission` — strict FIFO admission with no queueKey.
- `concurrency` — concurrency > 1 admits multiple runs per lane.
- `lane-isolation` — queueKey splits runs into independent lanes.
- `completion-promotion` — a queued run is promoted once the occupying run
  actually finishes on the real Tekton controller.
- `replace-queued` — the `ReplaceQueued` strategy cancels superseded
  queued runs, leaving an already-running run untouched.
- `cancel-in-progress` — the `CancelInProgress` strategy also cancels an
  already-running run when a newer arrival supersedes it.
- `invalid-selector` — an unparseable `spec.selector` flips `Ready` to
  `False`/`InvalidSelector` and zeroes the lane projection; fixing it
  recovers the queue.
- `crd-validation` — the CRD's OpenAPI schema rejects structurally invalid
  `PipelineRunQueue` objects (concurrency < 1, unknown strategy, missing
  selector) at admission time.

## Not covered

- **metrics-auth**: the Helm chart does not yet expose the operator's
  `/metrics` endpoint (no Service/ServiceMonitor in `deploy-templates/`), so
  there is nothing to assert against over HTTP. Add a scenario here once the
  chart grows metrics scraping support.
