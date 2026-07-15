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
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	edpv1alpha1 "github.com/KubeRocketCI/tekton-pipeline-queue/api/v1alpha1"
)

// applyStrategy decides, for a single lane, which queued runs to admit and
// which runs (queued or occupying) to cancel, given strategy and the lane's
// concurrency limit.
//
//   - Queue: never cancels; admits queued FIFO heads while occupying+admitted
//     stays below concurrency.
//   - ReplaceQueued: cancels every queued run except the newest arrival, then
//     admits as Queue.
//   - CancelInProgress: as ReplaceQueued, and additionally cancels every
//     occupying run older than the newest queued run (a superseding arrival
//     preempts in-flight work); if nothing is queued, nothing is cancelled.
//
// Cancelled occupying runs are intentionally still counted against
// concurrency for this pass: the controller derives all state from the live
// PipelineRun set, so the freed slot is only recognized once a later
// reconcile observes the run as no longer occupying (see isOccupying).
func applyStrategy(
	l *lane,
	strategy edpv1alpha1.QueueStrategy,
	concurrency int32,
) (admit, cancel []*tektonv1.PipelineRun) {
	queued := l.queued

	switch strategy {
	case edpv1alpha1.QueueStrategyReplaceQueued, edpv1alpha1.QueueStrategyCancelInProgress:
		queued, cancel = replaceQueued(queued)

		if strategy == edpv1alpha1.QueueStrategyCancelInProgress && len(queued) > 0 {
			cancel = append(cancel, cancelSupersededOccupying(l.occupying, queued[0])...)
		}
	case edpv1alpha1.QueueStrategyQueue:
		// No cancellation; admission below is the same for all strategies.
	}

	admit = admitFIFO(queued, concurrency-int32(len(l.occupying)))

	return admit, cancel
}

// replaceQueued keeps only the newest queued run (if any) and returns the
// rest as cancellation candidates.
func replaceQueued(queued []*tektonv1.PipelineRun) (kept, cancel []*tektonv1.PipelineRun) {
	if len(queued) <= 1 {
		return queued, nil
	}

	newest := queued[len(queued)-1]

	return []*tektonv1.PipelineRun{newest}, append([]*tektonv1.PipelineRun{}, queued[:len(queued)-1]...)
}

// cancelSupersededOccupying returns the occupying runs older than newest.
func cancelSupersededOccupying(
	occupying []*tektonv1.PipelineRun,
	newest *tektonv1.PipelineRun,
) []*tektonv1.PipelineRun {
	var cancel []*tektonv1.PipelineRun

	for _, occ := range occupying {
		if occ.CreationTimestamp.Before(&newest.CreationTimestamp) {
			cancel = append(cancel, occ)
		}
	}

	return cancel
}

// admitFIFO returns up to slots runs from the front of queued (which must
// already be FIFO-ordered). A non-positive slots admits nothing.
func admitFIFO(queued []*tektonv1.PipelineRun, slots int32) []*tektonv1.PipelineRun {
	if slots <= 0 {
		return nil
	}

	if int32(len(queued)) < slots {
		return queued
	}

	return queued[:slots]
}
