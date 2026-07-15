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
	"sort"
	"strings"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// lane groups the PipelineRuns sharing a single lane identity (see laneKey)
// into the two buckets the admission logic cares about. Everything else
// (done, cancelled, gracefully-cancelling runs) is dropped during bucketing
// since it neither occupies a concurrency slot nor waits for one.
type lane struct {
	// occupying holds runs that currently consume a concurrency slot.
	occupying []*tektonv1.PipelineRun
	// queued holds pending runs in FIFO admission order (oldest first,
	// CreationTimestamp then Name as a tiebreak).
	queued []*tektonv1.PipelineRun
}

// laneKey returns the lane identity for pr: the values of the queueKey
// labels, joined in order with "/". A label missing on pr contributes an
// empty segment (rather than collapsing lanes together), and an empty
// queueKey always yields the single "" lane.
func laneKey(pr *tektonv1.PipelineRun, queueKey []string) string {
	if len(queueKey) == 0 {
		return ""
	}

	segments := make([]string, len(queueKey))
	for i, key := range queueKey {
		segments[i] = pr.Labels[key]
	}

	return strings.Join(segments, "/")
}

// isQueued reports whether pr is waiting for admission.
func isQueued(pr *tektonv1.PipelineRun) bool {
	return pr.IsPending()
}

// isCancelling reports whether pr has been asked to stop, whether or not
// Tekton has yet reflected that in a terminal condition. A run in this state
// must free its lane slot immediately rather than waiting for IsDone(),
// otherwise the queue would stall behind a run it just told to cancel.
func isCancelling(pr *tektonv1.PipelineRun) bool {
	return pr.IsCancelled() || pr.IsGracefullyCancelled() || pr.IsGracefullyStopped()
}

// isOccupying reports whether pr currently holds a lane concurrency slot:
// it is neither pending admission, nor being cancelled, nor already done.
func isOccupying(pr *tektonv1.PipelineRun) bool {
	return !isQueued(pr) && !isCancelling(pr) && !pr.IsDone()
}

// bucketLanes partitions runs into per-lane occupying/queued buckets keyed by
// laneKey(run, queueKey). Runs that are neither occupying nor queued (done,
// cancelled, cancelling) are ignored. The queued bucket of each lane is
// sorted FIFO.
func bucketLanes(runs []tektonv1.PipelineRun, queueKey []string) map[string]*lane {
	lanes := make(map[string]*lane)

	for i := range runs {
		pr := &runs[i]

		occupying, queued := isOccupying(pr), isQueued(pr)
		if !occupying && !queued {
			// Done, cancelled, or cancelling: irrelevant to admission and must
			// not create an empty lane entry of its own.
			continue
		}

		key := laneKey(pr, queueKey)

		l, ok := lanes[key]
		if !ok {
			l = &lane{}
			lanes[key] = l
		}

		if occupying {
			l.occupying = append(l.occupying, pr)
		} else {
			l.queued = append(l.queued, pr)
		}
	}

	for _, l := range lanes {
		sortFIFO(l.queued)
	}

	return lanes
}

// sortFIFO orders runs oldest-first by CreationTimestamp, breaking ties by
// Name for a deterministic order when timestamps collide.
func sortFIFO(runs []*tektonv1.PipelineRun) {
	sort.Slice(runs, func(i, j int) bool {
		return fifoLess(runs[i], runs[j])
	})
}

// fifoLess is the single total order used everywhere runs are compared for
// age: CreationTimestamp first, Name as the tiebreak. CreationTimestamp has
// one-second resolution, so runs created in the same second (e.g. duplicate
// webhook deliveries) are otherwise incomparable — every ordering decision
// must go through this comparator or same-second arrivals get inconsistent
// treatment between admission and cancellation.
func fifoLess(a, b *tektonv1.PipelineRun) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}

	return a.Name < b.Name
}
