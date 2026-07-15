# Tekton Pipeline Queue

[![CI](https://github.com/KubeRocketCI/tekton-pipeline-queue/actions/workflows/pr.yaml/badge.svg)](https://github.com/KubeRocketCI/tekton-pipeline-queue/actions/workflows/pr.yaml)
[![E2E](https://github.com/KubeRocketCI/tekton-pipeline-queue/actions/workflows/e2e.yaml/badge.svg)](https://github.com/KubeRocketCI/tekton-pipeline-queue/actions/workflows/e2e.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/KubeRocketCI/tekton-pipeline-queue.svg)](https://pkg.go.dev/github.com/KubeRocketCI/tekton-pipeline-queue)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A Kubernetes operator that queues [Tekton](https://tekton.dev) PipelineRuns.
It serializes or limits concurrent PipelineRuns per *lane* — an arbitrary
combination of label values such as codebase, branch, pull request, or
CD pipeline + stage — instead of letting every run start at once and compete
for cluster resources.

Part of the [KubeRocketCI](https://docs.kuberocketci.io) platform, usable with
any Tekton installation.

## How it works

The operator builds on Tekton's native pause primitive, `spec.status: PipelineRunPending`:

1. **Producers** (Tekton TriggerTemplates, operators, UI) create PipelineRuns
   with `spec.status: PipelineRunPending` and identifying labels. Tekton
   accepts the run but starts nothing — no TaskRuns, no pods.
2. A **`PipelineRunQueue`** resource selects the runs it governs (label
   selector), derives a *lane* for each run from the values of the
   `queueKey` labels, and defines per-lane `concurrency` and a `strategy`.
3. The **controller** watches PipelineRuns and, on every change, re-derives
   each lane from the live cluster state: pending runs are admitted FIFO
   (by creation time) by clearing `spec.status` while the number of running
   runs is below `concurrency`; superseded runs are gracefully cancelled
   (`CancelledRunFinally`) depending on the strategy.

There is no stored queue: the live PipelineRun set is the single source of
truth, so controller restarts, out-of-band deletions, and manual cancellations
converge automatically. `status.lanes` is a read-only projection for
observability (portal, `kubectl`).

## Example

Review pipelines, one lane per pull request: each new push to a PR
supersedes the previous commit's run — queued predecessors are cancelled
and a still-running one is gracefully stopped — so CI spends resources
only on the newest commit, the one that can actually merge:

```yaml
apiVersion: edp.epam.com/v1alpha1
kind: PipelineRunQueue
metadata:
  name: review-queue
spec:
  selector:
    matchLabels:
      app.edp.epam.com/pipelinetype: review
  queueKey:
    - app.edp.epam.com/codebase
    - app.edp.epam.com/git-change-number
  concurrency: 1
  strategy: CancelInProgress
```

```console
$ kubectl get pipelinerunqueue review-queue
NAME           QUEUED   RUNNING   READY   AGE
review-queue   2        3         True    2d
```

Different pull requests run in parallel (each is its own lane); pushes to
the same pull request replace each other.

### Strategies

| Strategy | Behavior |
|---|---|
| `Queue` | Strict FIFO admission; nothing is ever cancelled. |
| `ReplaceQueued` | Queued runs superseded by a newer arrival in the same lane are cancelled; only the newest waits. |
| `CancelInProgress` | As `ReplaceQueued`, plus running runs older than the newest arrival are gracefully cancelled. |

Typical lane keys: `codebase + branch` for build pipelines,
`codebase + change number` for review pipelines (with `ReplaceQueued` or
`CancelInProgress`), `cdpipeline + cdstage` for deployments.

Full recipes — global caps, per-codebase serialization, latest-commit-wins
reviews, freshest-payload deploys, lane per Pipeline name, and how to keep
queues disjoint: [docs/use-cases.md](docs/use-cases.md). Ready-to-apply
manifests: [config/samples](config/samples/edp_v1alpha1_pipelinerunqueue.yaml).

## Installation

Requires an existing Tekton Pipelines installation (the operator watches
`tekton.dev/v1` PipelineRuns and never installs or owns that CRD).

Helm chart (includes CRDs):

```sh
helm install tekton-pipeline-queue ./deploy-templates -n krci
```

Or with kustomize during development:

```sh
make install          # CRDs
make deploy IMG=docker.io/epamedp/tekton-pipeline-queue:<tag>
```

## Annotations

The operator stamps every PipelineRun it acts on, in the same patch that
changes `spec.status`: `app.edp.epam.com/queue`, `.../queue-lane`, and
`.../queue-admitted-at` on admission; `.../queue`, `.../queue-lane`, and
`.../queue-cancel-reason: superseded` on cancellation. A run without these
annotations was never touched by the queue. How to read them and diagnose
queue behavior: [docs/debugging.md](docs/debugging.md).

## Metrics

Prometheus metrics exposed by the controller:

| Metric | Type | Labels |
|---|---|---|
| `tekton_pipeline_queue_depth` | gauge | queue, namespace, lane |
| `tekton_pipeline_queue_running` | gauge | queue, namespace, lane |
| `tekton_pipeline_queue_admissions_total` | counter | queue, namespace |
| `tekton_pipeline_queue_cancellations_total` | counter | queue, namespace, strategy |
| `tekton_pipeline_queue_time_in_queue_seconds` | histogram | queue, namespace |

## Development

```sh
make build            # compile the manager
make test             # unit + envtest integration tests
make test-e2e         # Chainsaw e2e tests against a real Tekton install on Kind
make lint             # golangci-lint
make manifests        # regenerate CRDs (config/crd/bases + deploy-templates/crds) and docs/api.md
make helm-docs        # regenerate deploy-templates/README.md
```

Documentation index: [docs/README.md](docs/README.md) — use cases,
debugging, API reference. Chart values:
[deploy-templates/README.md](deploy-templates/README.md).

## License

Apache-2.0 — see [LICENSE.txt](LICENSE.txt).
