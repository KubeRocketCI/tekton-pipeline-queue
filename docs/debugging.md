# Debugging

How to answer "why is my PipelineRun waiting / running / cancelled" from
cluster state alone. Concepts are defined in the
[documentation index](README.md); configuration recipes live in
[use-cases.md](use-cases.md).

## Why is my run Pending?

```console
kubectl get pipelinerunqueue -n <ns>
NAME           QUEUED   RUNNING   READY   AGE
review-queue   3        1         True    2d
```

`status.lanes` shows every lane with its running set and FIFO-ordered waiting
line — your run's position is its index in `queued`:

```console
kubectl get pipelinerunqueue review-queue -n <ns> -o jsonpath='{.status.lanes}' | jq
```

If the run is Pending but appears in **no** queue's lanes, no queue selects it
(label mismatch) — nothing will ever admit it. Compare the run's labels with
the queue's `spec.selector`, or check whether the operator is running at all.

If `READY` is `False`, read the condition — an invalid `spec.selector` stops
the queue entirely until the spec is fixed:

```console
kubectl get pipelinerunqueue review-queue -n <ns> -o jsonpath='{.status.conditions}' | jq
```

## Who started / cancelled my run, and why?

The operator stamps annotations on every run it acts on, inside the same
patch that changes `spec.status`:

| Annotation (`app.edp.epam.com/...`) | Present on | Meaning |
|---|---|---|
| `queue` | admitted and cancelled runs | the `PipelineRunQueue` that acted |
| `queue-lane` | admitted and cancelled runs | lane key at the moment of action |
| `queue-admitted-at` | admitted runs | RFC3339 release time; minus `creationTimestamp` = time spent queued |
| `queue-cancel-reason` | cancelled runs | `superseded` — a newer arrival in the lane replaced it |

```console
kubectl get pipelinerun <name> -n <ns> -o jsonpath='{.metadata.annotations}' | jq
```

Reading the combinations:

- **No queue annotations at all** — the operator never touched this run.
  If it is cancelled anyway, someone else did it (a user, the interceptor's
  cancel-in-progress feature, `kubectl`); check `metadata.managedFields` for
  the field manager that wrote `spec.status`.
- **`queue-admitted-at` only** — normal admission; the run then finished,
  failed, or was cancelled by someone other than the queue.
- **`queue-cancel-reason: superseded`** — the queue cancelled it under
  `ReplaceQueued`/`CancelInProgress`. The successor is the newest run with the
  same `queue-lane` value.
- **Both admit and cancel sets** — admitted first, then superseded while
  running (`CancelInProgress`).

## Operator logs and metrics

Every admission and cancellation is logged with queue, lane, run, and
strategy:

```console
kubectl logs deploy/tekton-pipeline-queue -n <ns> | grep -E 'Admitted|Cancelled'
```

Prometheus metrics (see [README](../README.md#metrics)): alert on
`tekton_pipeline_queue_depth` growth and use
`tekton_pipeline_queue_time_in_queue_seconds` to size `concurrency`.

## Common pitfalls

- **Runs stay Pending forever, no queue exists** — producers create Pending
  runs (e.g. TriggerTemplates were switched on) but the operator or the
  `PipelineRunQueue` was not installed. Either install/create it or clear the
  runs manually: `kubectl patch pipelinerun <name> --type=merge -p '{"spec":{"status":""}}'`.
- **Deleted the queue while runs were waiting** — remaining Pending runs are
  orphaned (nothing admits them); release or delete them manually as above.
- **Lane looks stuck although nothing runs** — check for a run in
  `Terminating` (deletion blocked by a finalizer) or verify the queue's
  `status.observedGeneration` matches `metadata.generation`; a stale
  projection converges within the 2-minute safety requeue at the latest.
- **`tekton.dev/pipeline` lane key and brand-new runs** — Tekton stamps that
  label on its first reconcile; in the brief window before it, the run sits in
  the empty-string lane. Key on a producer-set label if that matters.
- **Operator memory** — the controller caches every PipelineRun in the
  cluster (heavily stripped: only metadata, `spec.status`, and conditions are
  retained), so its footprint scales with the number of runs Tekton's pruner
  keeps around, not with their size. If memory grows unexpectedly, check the
  cluster's completed-run retention before suspecting a leak.
