# Chainsaw e2e tests

Chainsaw-based end-to-end scenarios exercising the queue controller against
a real Tekton Pipelines install. The suite is self-provisioning and runs as
ONE unit: the `00-install` test executes first (lexical order, `parallel:
1`) and provisions the current kubeconfig context — Tekton Pipelines
(idempotent, skipped if the CRDs already exist) and the operator Helm chart
into the fixed `tpq-e2e` namespace — then every scenario runs against that
install. The operator image is resolved from the standard KRCI e2e env
contract: `CONTAINER_REGISTRY_URL` / `CONTAINER_REGISTRY_SPACE` /
`E2E_IMAGE_REPOSITORY` / `E2E_IMAGE_TAG` (same contract as
cd-pipeline-operator and the `e2e-chainsaw` Tekton task on the core
cluster, which runs this suite inside a vcluster). Always run the suite
whole — individual scenarios assume `00-install` has run.

Run locally via `make start-kind && make e2e` from the repo root (creates
the Kind cluster, builds the operator image, loads it into Kind, exports
the env contract, runs Chainsaw). `make delete-kind` tears the cluster
down.

Each directory under this folder is one scenario:

- `00-install` — suite provisioning (Tekton Pipelines + operator chart),
  not a behavior test; must sort first.

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
