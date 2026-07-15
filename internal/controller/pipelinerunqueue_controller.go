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

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	edpv1alpha1 "github.com/KubeRocketCI/tekton-pipeline-queue/api/v1alpha1"
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
// TODO(phase-2): implement lane grouping and FIFO admission logic; this phase
// only wires up the API types, RBAC, and the PipelineRun watch.
func (r *PipelineRunQueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineRunQueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&edpv1alpha1.PipelineRunQueue{}).
		Watches(
			&tektonv1.PipelineRun{},
			handler.EnqueueRequestsFromMapFunc(r.mapPipelineRunToQueues),
		).
		Named("pipelinerunqueue").
		Complete(r)
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
