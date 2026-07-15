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

// Annotations the queue controller stamps on PipelineRuns it acts on. They
// are recorded facts, not selectors: a run the controller never admits or
// cancels carries none of them. All writes ride inside the controller's
// existing admit/cancel spec patches, so they add no API calls.
const (
	// AnnotationQueue names the PipelineRunQueue that admitted or cancelled
	// the run. Stamped on both admission and cancellation.
	AnnotationQueue = "app.edp.epam.com/queue"

	// AnnotationQueueLane records the lane key the run belonged to at the
	// moment the controller acted on it. Stamped on both admission and
	// cancellation.
	AnnotationQueueLane = "app.edp.epam.com/queue-lane"

	// AnnotationQueueAdmittedAt is the RFC3339 time at which the controller
	// cleared the run's pending status. Admission only; subtracting
	// metadata.creationTimestamp gives the run's time in queue.
	AnnotationQueueAdmittedAt = "app.edp.epam.com/queue-admitted-at"

	// AnnotationQueueCancelReason explains why the controller cancelled the
	// run. Cancellation only. Its absence on a cancelled run means the
	// cancellation came from elsewhere (a user, the interceptor, kubectl).
	AnnotationQueueCancelReason = "app.edp.epam.com/queue-cancel-reason"
)

// CancelReasonSuperseded is the only cancel reason the controller emits
// today: a newer arrival in the same lane replaced the run under the
// ReplaceQueued or CancelInProgress strategy.
const CancelReasonSuperseded = "superseded"
