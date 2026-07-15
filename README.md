# tekton-pipeline-queue

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

Serialize deployments per CD pipeline stage — one deploy at a time, the rest
visibly queued:

```yaml
apiVersion: edp.epam.com/v1alpha1
kind: PipelineRunQueue
metadata:
  name: deploy-queue
spec:
  selector:
    matchLabels:
      app.edp.epam.com/pipelinetype: deploy
  queueKey:
    - app.edp.epam.com/cdpipeline
    - app.edp.epam.com/cdstage
  concurrency: 1
  strategy: Queue
```

```console
$ kubectl get pipelinerunqueue deploy-queue
NAME           QUEUED   RUNNING   READY   AGE
deploy-queue   4        1         True    2d
```

### Strategies

| Strategy | Behavior |
|---|---|
| `Queue` | Strict FIFO admission; nothing is ever cancelled. |
| `ReplaceQueued` | Queued runs superseded by a newer arrival in the same lane are cancelled; only the newest waits. |
| `CancelInProgress` | As `ReplaceQueued`, plus running runs older than the newest arrival are gracefully cancelled. |

Typical lane keys: `codebase + branch` for build pipelines,
`codebase + change number` for review pipelines (with `ReplaceQueued` or
`CancelInProgress`), `cdpipeline + cdstage` for deployments.

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
make lint             # golangci-lint
make manifests        # regenerate CRDs (config/crd/bases + deploy-templates/crds) and docs/api.md
make helm-docs        # regenerate deploy-templates/README.md
```

API reference: [docs/api.md](docs/api.md). Chart values:
[deploy-templates/README.md](deploy-templates/README.md).

## License

Apache-2.0 — see [LICENSE.txt](LICENSE.txt).
