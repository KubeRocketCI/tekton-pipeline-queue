# tekton-pipeline-queue documentation

The operator queues Tekton PipelineRuns: producers create runs paused
(`spec.status: PipelineRunPending`), a `PipelineRunQueue` resource decides
when each one is allowed to start, and everything in between is observable
from cluster state.

## Where to go

| Document | Read it when you want to |
|---|---|
| [use-cases.md](use-cases.md) | configure queues and lanes: recipes for review/build/deploy scenarios, choosing a strategy, sizing concurrency |
| [debugging.md](debugging.md) | understand behavior: why a run is waiting, who admitted or cancelled it, common pitfalls |
| [api.md](api.md) | look up the `PipelineRunQueue` CRD fields (generated reference) |
| [../deploy-templates/README.md](../deploy-templates/README.md) | install and configure the Helm chart (generated values reference) |
| [../config/samples](../config/samples/edp_v1alpha1_pipelinerunqueue.yaml) | copy ready-to-apply queue manifests |

## Concepts in two minutes

**Queue** — a namespaced `PipelineRunQueue` resource. Its `spec.selector`
picks the PipelineRuns it governs; typically one queue per pipeline type
(review, build, deploy) so queues never overlap.

**Lane** — the unit of ordering. Each governed run is assigned to a lane by
joining the values of the labels listed in `spec.queueKey` (empty `queueKey`
= one lane for the whole queue). Lanes are derived, not declared: a lane per
branch or per pull request appears and disappears with its runs, and a run
always belongs to exactly one lane.

**Admission** — per lane, pending runs start in FIFO order (creation time)
while the number of running runs is below `spec.concurrency`. Starting a run
= the operator clears its `spec.status`; from then on Tekton owns it.

**Strategy** — what happens to runs made obsolete by a newer arrival in the
same lane:

| `spec.strategy` | Waiting runs | Running runs |
|---|---|---|
| `Queue` (default) | all kept, FIFO | never touched |
| `ReplaceQueued` | only newest kept | never touched |
| `CancelInProgress` | only newest kept | cancelled when superseded |

**Lifecycle** of a governed run:

```text
created (Pending) ──► admitted (FIFO, slot free) ──► running ──► done
        │                                              │
        └── superseded ──► cancelled (strategy)  ◄─────┘
```

Every action leaves a trace: the operator stamps annotations
(`app.edp.epam.com/queue`, `queue-lane`, `queue-admitted-at`,
`queue-cancel-reason`) on the run, writes an ordered projection into the
queue's `status.lanes`, logs each admission/cancellation, and exports
Prometheus metrics (queue depth, running, admissions, cancellations,
time-in-queue). [debugging.md](debugging.md) shows how to read each of these.

**No stored state** — the live PipelineRun set is the single source of
truth. The operator re-derives every lane on every change, so restarts,
manual cancellations, and out-of-band deletions converge without repair.
