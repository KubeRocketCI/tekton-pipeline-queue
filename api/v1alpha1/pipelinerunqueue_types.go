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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Condition types set on PipelineRunQueue.status.conditions.
const (
	// ConditionReady indicates the queue is reconciled and its lane projection
	// reflects the current live PipelineRun set.
	ConditionReady = "Ready"
	// ConditionDegraded indicates the controller could not fully reconcile the
	// queue (e.g. an invalid selector or a failed admission update).
	ConditionDegraded = "Degraded"
)

// QueueStrategy controls how a lane reacts to the arrival of a newer PipelineRun.
type QueueStrategy string

const (
	// QueueStrategyQueue admits runs strictly FIFO, never cancelling anything.
	QueueStrategyQueue QueueStrategy = "Queue"
	// QueueStrategyReplaceQueued cancels all queued (pending) runs in a lane except the newest.
	QueueStrategyReplaceQueued QueueStrategy = "ReplaceQueued"
	// QueueStrategyCancelInProgress additionally cancels running runs superseded by a newer arrival.
	QueueStrategyCancelInProgress QueueStrategy = "CancelInProgress"
)

// PipelineRunQueueSpec defines the desired state of PipelineRunQueue.
type PipelineRunQueueSpec struct {
	// Selector picks the PipelineRuns in this namespace governed by the queue. Required.
	Selector metav1.LabelSelector `json:"selector"`

	// QueueKey lists PipelineRun label keys whose values form the lane identity.
	// Runs whose values differ for any key land in independent lanes.
	// Empty means the queue forms a single lane.
	// +optional
	QueueKey []string `json:"queueKey,omitempty"`

	// Concurrency is the max number of concurrently running PipelineRuns per lane.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	Concurrency int32 `json:"concurrency,omitempty"`

	// Strategy determines how the queue treats already-admitted runs when a newer
	// PipelineRun arrives in the same lane.
	// +kubebuilder:validation:Enum=Queue;ReplaceQueued;CancelInProgress
	// +kubebuilder:default=Queue
	// +optional
	Strategy QueueStrategy `json:"strategy,omitempty"`
}

// LaneStatus reports the derived queue state for a single lane.
type LaneStatus struct {
	// Key is the lane identity, joined values of spec.queueKey labels ("/"-separated; "" for the single lane).
	Key string `json:"key"`

	// Running lists names of PipelineRuns currently occupying lane slots.
	// +optional
	Running []string `json:"running,omitempty"`

	// Queued lists names of pending PipelineRuns in admission (FIFO) order.
	// +optional
	Queued []string `json:"queued,omitempty"`
}

// PipelineRunQueueStatus defines the observed state of PipelineRunQueue.
type PipelineRunQueueStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state of the PipelineRunQueue resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Ready": the queue is reconciled and its lane projection is up to date
	// - "Degraded": the controller failed to reach or maintain the desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Lanes is a projection of the derived queue state for observability;
	// the source of truth is always the live PipelineRun set.
	// +optional
	Lanes []LaneStatus `json:"lanes,omitempty"`

	// QueuedCount is the total number of pending PipelineRuns across all lanes.
	// +optional
	QueuedCount int32 `json:"queuedCount,omitempty"`

	// RunningCount is the total number of running PipelineRuns across all lanes.
	// +optional
	RunningCount int32 `json:"runningCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Queued",type=integer,JSONPath=.status.queuedCount
// +kubebuilder:printcolumn:name="Running",type=integer,JSONPath=.status.runningCount
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=.status.conditions[?(@.type=='Ready')].status
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=.metadata.creationTimestamp

// PipelineRunQueue is the Schema for the pipelinerunqueues API
type PipelineRunQueue struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of PipelineRunQueue
	// +required
	Spec PipelineRunQueueSpec `json:"spec"`

	// status defines the observed state of PipelineRunQueue
	// +optional
	Status PipelineRunQueueStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PipelineRunQueueList contains a list of PipelineRunQueue
type PipelineRunQueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []PipelineRunQueue `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &PipelineRunQueue{}, &PipelineRunQueueList{})
		return nil
	})
}
