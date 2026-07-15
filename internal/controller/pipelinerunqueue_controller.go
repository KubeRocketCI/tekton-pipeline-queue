/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"time"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	edpv1alpha1 "github.com/KubeRocketCI/tekton-pipeline-queue/api/v1alpha1"
)

// requeueInterval is a safety net against missed watch events: even if a
// PipelineRun change is somehow not observed, the queue re-derives its state
// from the live PipelineRun set at least this often.
const requeueInterval = 2 * time.Minute

// reasonInvalidSelector/reasonReconciled are the Ready condition reasons set
// by Reconcile.
const (
	reasonInvalidSelector = "InvalidSelector"
	reasonReconciled      = "Reconciled"
)

// PipelineRunQueueReconciler reconciles a PipelineRunQueue object
type PipelineRunQueueReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=edp.epam.com,resources=pipelinerunqueues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=edp.epam.com,resources=pipelinerunqueues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=edp.epam.com,resources=pipelinerunqueues/finalizers,verbs=update
// +kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns,verbs=get;list;watch;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// All queue state is derived from the live PipelineRun set on every call:
// the controller keeps no in-memory bookkeeping between reconciles.
func (r *PipelineRunQueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	queue := &edpv1alpha1.PipelineRunQueue{}
	if err := r.Get(ctx, req.NamespacedName, queue); err != nil {
		if apierrors.IsNotFound(err) {
			// The queue is gone; drop its gauge series so dashboards don't
			// keep reporting the last-observed depth/running values forever.
			deleteQueueMetrics(req.Name, req.Namespace)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get PipelineRunQueue: %w", err)
	}

	selector, err := metav1.LabelSelectorAsSelector(&queue.Spec.Selector)
	if err != nil {
		log.Error(err, "Invalid PipelineRunQueue selector", "queue", queue.Name, "namespace", queue.Namespace)

		// Lanes derived under the old selector are no longer being observed;
		// drop their gauge series rather than freezing stale values.
		deleteQueueMetrics(queue.Name, queue.Namespace)

		if updateErr := r.markInvalidSelector(ctx, queue, err); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		// The selector can only change via a spec edit; nothing to retry
		// until the user fixes it.
		return ctrl.Result{}, nil
	}

	runList := &tektonv1.PipelineRunList{}
	if err := r.List(ctx, runList,
		client.InNamespace(queue.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list PipelineRuns: %w", err)
	}

	concurrency := queue.Spec.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	lanes := bucketLanes(runList.Items, queue.Spec.QueueKey)

	statusLanes, queuedTotal, runningTotal := r.applyLanes(ctx, queue, lanes, concurrency)

	if err := r.updateStatus(ctx, req.NamespacedName, statusLanes, queuedTotal, runningTotal); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

// applyLanes runs the configured strategy for every lane, best-effort
// patching cancellations and admissions (a failure to patch one run is
// logged and does not block the others; the next watch event will retry),
// records metrics, and returns the effective post-action status projection.
func (r *PipelineRunQueueReconciler) applyLanes(
	ctx context.Context,
	queue *edpv1alpha1.PipelineRunQueue,
	lanes map[string]*lane,
	concurrency int32,
) (statusLanes []edpv1alpha1.LaneStatus, queuedTotal, runningTotal int32) {
	log := logf.FromContext(ctx)

	resetLaneMetrics(queue)

	keys := make([]string, 0, len(lanes))
	for key := range lanes {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	statusLanes = make([]edpv1alpha1.LaneStatus, 0, len(keys))

	for _, key := range keys {
		l := lanes[key]

		admit, cancel := applyStrategy(l, queue.Spec.Strategy, concurrency)

		handled := make(map[string]bool, len(admit)+len(cancel))

		for _, pr := range cancel {
			if err := r.cancelRun(ctx, pr); err != nil {
				log.Error(err, "Failed to cancel PipelineRun", "lane", key, "pipelineRun", pr.Name)
				continue
			}

			handled[pr.Name] = true

			cancellationsTotal.WithLabelValues(queue.Name, queue.Namespace, string(queue.Spec.Strategy)).Inc()
			log.Info("Cancelled PipelineRun", "lane", key, "pipelineRun", pr.Name, "strategy", queue.Spec.Strategy)
		}

		for _, pr := range admit {
			if err := r.admitRun(ctx, pr); err != nil {
				log.Error(err, "Failed to admit PipelineRun", "lane", key, "pipelineRun", pr.Name)
				continue
			}

			handled[pr.Name] = true

			admissionsTotal.WithLabelValues(queue.Name, queue.Namespace).Inc()
			timeInQueue.WithLabelValues(queue.Name, queue.Namespace).
				Observe(time.Since(pr.CreationTimestamp.Time).Seconds())
			log.Info("Admitted PipelineRun", "lane", key, "pipelineRun", pr.Name)
		}

		running := runningNames(l.occupying)
		queued := effectiveQueuedNames(l.queued, handled)

		queueDepth.WithLabelValues(queue.Name, queue.Namespace, key).Set(float64(len(queued)))
		queueRunning.WithLabelValues(queue.Name, queue.Namespace, key).Set(float64(len(running)))

		queuedTotal += int32(len(queued))
		runningTotal += int32(len(running))

		statusLanes = append(statusLanes, edpv1alpha1.LaneStatus{
			Key:     key,
			Running: running,
			Queued:  queued,
		})
	}

	return statusLanes, queuedTotal, runningTotal
}

// runningNames returns the sorted names of the runs currently occupying a
// lane's concurrency slots.
func runningNames(occupying []*tektonv1.PipelineRun) []string {
	names := make([]string, 0, len(occupying))
	for _, pr := range occupying {
		names = append(names, pr.Name)
	}

	sort.Strings(names)

	return names
}

// effectiveQueuedNames returns the FIFO-ordered names of queued runs that
// were neither admitted nor cancelled during this reconcile, i.e. the queue
// projection as it will look once our patches are observed.
func effectiveQueuedNames(queued []*tektonv1.PipelineRun, handled map[string]bool) []string {
	names := make([]string, 0, len(queued))

	for _, pr := range queued {
		if handled[pr.Name] {
			continue
		}

		names = append(names, pr.Name)
	}

	return names
}

// cancelRun asks pr to gracefully cancel by patching spec.status. Runs that
// are already done or already cancelling are skipped; bucketLanes never
// classifies such runs as occupying/queued in the first place, so this is a
// defensive no-op rather than the common path.
func (r *PipelineRunQueueReconciler) cancelRun(ctx context.Context, pr *tektonv1.PipelineRun) error {
	if pr.IsDone() || isCancelling(pr) {
		return nil
	}

	patch := client.MergeFrom(pr.DeepCopy())
	pr.Spec.Status = tektonv1.PipelineRunSpecStatusCancelledRunFinally

	if err := r.Patch(ctx, pr, patch); err != nil {
		return fmt.Errorf("failed to patch PipelineRun %s/%s to CancelledRunFinally: %w", pr.Namespace, pr.Name, err)
	}

	return nil
}

// admitRun clears spec.status on pr so Tekton starts running it.
func (r *PipelineRunQueueReconciler) admitRun(ctx context.Context, pr *tektonv1.PipelineRun) error {
	patch := client.MergeFrom(pr.DeepCopy())
	pr.Spec.Status = ""

	if err := r.Patch(ctx, pr, patch); err != nil {
		return fmt.Errorf("failed to patch PipelineRun %s/%s to clear pending status: %w", pr.Namespace, pr.Name, err)
	}

	return nil
}

// markInvalidSelector sets a False Ready condition on queue explaining why
// its spec.selector could not be parsed, and persists it. Merge patch for
// the same cache-staleness reason as updateStatus.
func (r *PipelineRunQueueReconciler) markInvalidSelector(
	ctx context.Context,
	queue *edpv1alpha1.PipelineRunQueue,
	cause error,
) error {
	base := queue.DeepCopy()

	apimeta.SetStatusCondition(&queue.Status.Conditions, metav1.Condition{
		Type:               edpv1alpha1.ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             reasonInvalidSelector,
		Message:            cause.Error(),
		ObservedGeneration: queue.Generation,
	})
	queue.Status.ObservedGeneration = queue.Generation

	if err := r.Status().Patch(ctx, queue, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("failed to patch PipelineRunQueue status for invalid selector: %w", err)
	}

	return nil
}

// updateStatus re-fetches queue, applies the freshly computed lane
// projection, and persists it only if it actually changed, so our own
// status write does not force perpetual re-reconciliation.
//
// The write is a merge patch without optimistic locking rather than an
// Update: the re-Get is served from the (possibly stale) cache, so right
// after our own previous status write an Update would routinely fail with
// a resourceVersion conflict during event bursts. Last-writer-wins is safe
// here because every reconcile recomputes the full projection from the
// live PipelineRun set.
func (r *PipelineRunQueueReconciler) updateStatus(
	ctx context.Context,
	key client.ObjectKey,
	statusLanes []edpv1alpha1.LaneStatus,
	queuedTotal, runningTotal int32,
) error {
	queue := &edpv1alpha1.PipelineRunQueue{}
	if err := r.Get(ctx, key, queue); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to re-get PipelineRunQueue before status update: %w", err)
	}

	base := queue.DeepCopy()

	apimeta.SetStatusCondition(&queue.Status.Conditions, metav1.Condition{
		Type:               edpv1alpha1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             reasonReconciled,
		Message:            "Lane projection reflects the current live PipelineRun set.",
		ObservedGeneration: queue.Generation,
	})
	queue.Status.Lanes = statusLanes
	queue.Status.QueuedCount = queuedTotal
	queue.Status.RunningCount = runningTotal
	queue.Status.ObservedGeneration = queue.Generation

	if apiequality.Semantic.DeepEqual(&base.Status, &queue.Status) {
		return nil
	}

	if err := r.Status().Patch(ctx, queue, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("failed to patch PipelineRunQueue status: %w", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineRunQueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&edpv1alpha1.PipelineRunQueue{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&tektonv1.PipelineRun{},
			handler.EnqueueRequestsFromMapFunc(r.mapPipelineRunToQueues),
			builder.WithPredicates(pipelineRunPredicate),
		).
		Named("pipelinerunqueue").
		Complete(r)
}

// pipelineRunPredicate filters PipelineRun watch events down to the ones
// that can actually change a queue's admission decision: creation, deletion,
// or an update that changed spec.status, done-ness, or labels (which affect
// selector matching and lane identity). Everything else (e.g. status
// message churn while running) is dropped to avoid needless reconciles.
var pipelineRunPredicate = predicate.Funcs{
	CreateFunc:  func(event.CreateEvent) bool { return true },
	DeleteFunc:  func(event.DeleteEvent) bool { return true },
	GenericFunc: func(event.GenericEvent) bool { return true },
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldPR, oldOK := e.ObjectOld.(*tektonv1.PipelineRun)
		newPR, newOK := e.ObjectNew.(*tektonv1.PipelineRun)

		if !oldOK || !newOK {
			// Cast failure should never happen for a typed watch, but fail
			// open (reconcile) rather than silently drop the event.
			return true
		}

		if oldPR.Spec.Status != newPR.Spec.Status {
			return true
		}

		if oldPR.IsDone() != newPR.IsDone() {
			return true
		}

		return !maps.Equal(oldPR.Labels, newPR.Labels)
	},
}

// mapPipelineRunToQueues returns a reconcile request for every PipelineRunQueue in
// obj's namespace whose spec.selector matches obj's labels, so that a PipelineRun
// change re-triggers reconciliation of the queues that govern it.
func (r *PipelineRunQueueReconciler) mapPipelineRunToQueues(
	ctx context.Context,
	obj client.Object,
) []reconcile.Request {
	log := logf.FromContext(ctx)

	queueList := &edpv1alpha1.PipelineRunQueueList{}
	if err := r.List(ctx, queueList, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Failed to list PipelineRunQueues", "namespace", obj.GetNamespace())
		return nil
	}

	var requests []reconcile.Request

	for i := range queueList.Items {
		queue := &queueList.Items[i]

		selector, err := metav1.LabelSelectorAsSelector(&queue.Spec.Selector)
		if err != nil {
			log.Error(err, "Skipping PipelineRunQueue with invalid selector",
				"queue", queue.Name, "namespace", queue.Namespace)

			continue
		}

		if selector.Matches(labels.Set(obj.GetLabels())) {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(queue),
			})
		}
	}

	return requests
}
