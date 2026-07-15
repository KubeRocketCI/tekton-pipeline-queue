# Use cases

Recipes for configuring queues and lanes. Concepts (queue, lane, admission,
strategy) are defined in the [documentation index](README.md); ready-to-apply
manifests for every recipe live in
[config/samples](../config/samples/edp_v1alpha1_pipelinerunqueue.yaml).

## Recipe overview

| Goal | selector | queueKey | concurrency / strategy |
|---|---|---|---|
| Max N review pipelines cluster-wide | `pipelinetype: review` | *(empty — one lane)* | `N` / `Queue` |
| One pipeline at a time per codebase | `pipelinetype: review` | `codebase` | `1` / `Queue` |
| One build per branch | `pipelinetype: build` | `codebase`, `codebasebranch` | `1` / `Queue` |
| Latest commit wins per pull request | `pipelinetype: review` | `codebase`, `git-change-number` | `1` / `CancelInProgress` |
| One deploy per environment, freshest payload | `pipelinetype: deploy` | `cdpipeline`, `cdstage` | `1` / `ReplaceQueued` |
| One run per Pipeline name | `tekton.dev/pipeline` exists | `tekton.dev/pipeline` | `1` / `Queue` |

(Label keys abbreviated; all are `app.edp.epam.com/...` except
`tekton.dev/pipeline`.)

## Global cap: at most N runs of a type in parallel

An empty `queueKey` makes the whole queue a single lane; `concurrency`
becomes a cluster-wide cap for the selected runs:

```yaml
apiVersion: edp.epam.com/v1alpha1
kind: PipelineRunQueue
metadata:
  name: review-cap
spec:
  selector:
    matchLabels:
      app.edp.epam.com/pipelinetype: review
  concurrency: 2
  strategy: Queue
```

Behavior: the first two review runs execute, every further one waits FIFO;
each completion admits the next. Nothing is ever cancelled.

## Serialize per key: one run per codebase / branch / environment

List the label keys whose values define "the same thing" in `queueKey` — one
lane materializes per distinct combination, each with its own FIFO line and
concurrency budget:

```yaml
apiVersion: edp.epam.com/v1alpha1
kind: PipelineRunQueue
metadata:
  name: build-queue
spec:
  selector:
    matchLabels:
      app.edp.epam.com/pipelinetype: build
  queueKey:
    - app.edp.epam.com/codebase
    - app.edp.epam.com/codebasebranch
  concurrency: 1
  strategy: Queue
```

Behavior: builds of `app-a/main` run one at a time; builds of `app-a/dev`
and `app-b/main` proceed in parallel, unaffected. Scale by adding `queueKey`
labels, never by creating one queue per codebase — lanes are free and
disjoint by construction.

## Latest commit wins: review pipelines per pull request

New pushes make earlier runs of the same PR pointless. Lane per PR plus
`CancelInProgress` keeps exactly one relevant run:

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

Behavior: a push while the previous commit's run is mid-flight gracefully
cancels it (`CancelledRunFinally` — `finally` tasks still report status) and
the new run starts immediately. Queued-but-not-started runs that get
superseded are cancelled without ever starting.

## Freshest payload: deploy pipelines per environment

Deploy runs freeze their parameters (image tags) at creation. `ReplaceQueued`
guarantees the waiting line never holds a stale payload, while an in-flight
deploy is always left to finish (killing a half-applied deploy is worse than
completing it and rolling the newest right after):

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
  strategy: ReplaceQueued
```

Behavior: while a deploy runs, each newer deploy request for the same
environment cancels the previously queued one; when the running deploy
finishes, the newest (freshest-payload) request is admitted.

Caveat — **manually pinned deploys**: a user who queued version `1.2.3`
must not be superseded by an auto-triggered run. Keep such runs out of
replace-style queues (distinguishing label, separate `Queue`-strategy queue).

## Lane per Pipeline name

`queueKey` accepts any label, including Tekton's own `tekton.dev/pipeline`
(stamped from `spec.pipelineRef.name` on Tekton's first reconcile, before the
Pending state is honored):

```yaml
apiVersion: edp.epam.com/v1alpha1
kind: PipelineRunQueue
metadata:
  name: per-pipeline-queue
spec:
  selector:
    matchExpressions:
      - key: tekton.dev/pipeline
        operator: Exists
  queueKey:
    - tekton.dev/pipeline
  concurrency: 1
  strategy: Queue
```

There is a short window between run creation and Tekton's first reconcile
where the label does not exist yet and the run sits in the empty-string lane.
If strict grouping from the first instant matters, have the producer stamp
its own label at creation (e.g. from the TriggerTemplate) and key on that.

## Rules of thumb

- **Lanes within one queue are always disjoint** — a run maps to exactly one
  lane. Prefer more `queueKey` labels over more queue objects.
- **Keep queues disjoint from each other** by selecting on
  `app.edp.epam.com/pipelinetype`: a run has exactly one value, so per-type
  queues can never overlap. Two queues matching the same run would count its
  lane slots independently — avoid that.
- **Choosing a strategy**: every run matters → `Queue`; only the newest
  request matters but in-flight work must finish → `ReplaceQueued`; only the
  newest result matters at all → `CancelInProgress`.
- **Sizing `concurrency`**: watch the
  `tekton_pipeline_queue_time_in_queue_seconds` histogram and
  `tekton_pipeline_queue_depth` gauge; sustained growth means the lane needs
  a higher budget (or the pipeline needs to be faster).
- **Producers must create runs Pending** — a run created without
  `spec.status: PipelineRunPending` starts immediately and merely occupies
  its lane's slots from the queue's point of view; it is never held back.
