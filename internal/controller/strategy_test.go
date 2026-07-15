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
	"testing"
	"time"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	edpv1alpha1 "github.com/KubeRocketCI/tekton-pipeline-queue/api/v1alpha1"
)

func names(runs []*tektonv1.PipelineRun) []string {
	out := make([]string, len(runs))
	for i, r := range runs {
		out[i] = r.Name
	}

	return out
}

func equalNames(t *testing.T, label string, got []*tektonv1.PipelineRun, want []string) {
	t.Helper()

	gotNames := names(got)

	if len(gotNames) != len(want) {
		t.Fatalf("%s = %v, want %v", label, gotNames, want)
	}

	for i := range want {
		if gotNames[i] != want[i] {
			t.Fatalf("%s = %v, want %v", label, gotNames, want)
		}
	}
}

func TestApplyStrategyQueue(t *testing.T) {
	t.Parallel()

	t.Run("admits heads of queued FIFO up to concurrency", func(t *testing.T) {
		t.Parallel()

		l := &lane{
			occupying: nil,
			queued: []*tektonv1.PipelineRun{
				pendingRun("q1", nil, 0),
				pendingRun("q2", nil, time.Second),
				pendingRun("q3", nil, 2*time.Second),
			},
		}

		admit, cancel := applyStrategy(l, edpv1alpha1.QueueStrategyQueue, 2)

		equalNames(t, "admit", admit, []string{"q1", "q2"})
		equalNames(t, "cancel", cancel, nil)
	})

	t.Run("occupied slots reduce admission", func(t *testing.T) {
		t.Parallel()

		l := &lane{
			occupying: []*tektonv1.PipelineRun{runningRun("r1", nil, 0)},
			queued:    []*tektonv1.PipelineRun{pendingRun("q1", nil, 0)},
		}

		admit, cancel := applyStrategy(l, edpv1alpha1.QueueStrategyQueue, 1)

		equalNames(t, "admit", admit, nil)
		equalNames(t, "cancel", cancel, nil)
	})

	t.Run("empty queued admits nothing", func(t *testing.T) {
		t.Parallel()

		l := &lane{}

		admit, cancel := applyStrategy(l, edpv1alpha1.QueueStrategyQueue, 5)

		equalNames(t, "admit", admit, nil)
		equalNames(t, "cancel", cancel, nil)
	})
}

func TestApplyStrategyReplaceQueued(t *testing.T) {
	t.Parallel()

	t.Run("cancels all but the newest queued run", func(t *testing.T) {
		t.Parallel()

		l := &lane{
			queued: []*tektonv1.PipelineRun{
				pendingRun("q1", nil, 0),
				pendingRun("q2", nil, time.Second),
				pendingRun("q3", nil, 2*time.Second),
			},
		}

		admit, cancel := applyStrategy(l, edpv1alpha1.QueueStrategyReplaceQueued, 1)

		equalNames(t, "cancel", cancel, []string{"q1", "q2"})
		equalNames(t, "admit", admit, []string{"q3"})
	})

	t.Run("does not touch occupying runs", func(t *testing.T) {
		t.Parallel()

		l := &lane{
			occupying: []*tektonv1.PipelineRun{runningRun("r1", nil, 0)},
			queued: []*tektonv1.PipelineRun{
				pendingRun("q1", nil, 0),
				pendingRun("q2", nil, time.Second),
			},
		}

		admit, cancel := applyStrategy(l, edpv1alpha1.QueueStrategyReplaceQueued, 1)

		equalNames(t, "cancel", cancel, []string{"q1"})
		equalNames(t, "admit", admit, nil) // concurrency already saturated by r1
	})

	t.Run("empty queued cancels nothing", func(t *testing.T) {
		t.Parallel()

		l := &lane{occupying: []*tektonv1.PipelineRun{runningRun("r1", nil, 0)}}

		admit, cancel := applyStrategy(l, edpv1alpha1.QueueStrategyReplaceQueued, 2)

		equalNames(t, "admit", admit, nil)
		equalNames(t, "cancel", cancel, nil)
	})
}

func TestApplyStrategyCancelInProgress(t *testing.T) {
	t.Parallel()

	t.Run("cancels every occupying run older than the surviving queued run", func(t *testing.T) {
		t.Parallel()

		l := &lane{
			occupying: []*tektonv1.PipelineRun{
				runningRun("old", nil, 0),
				runningRun("newer", nil, 5*time.Second),
			},
			queued: []*tektonv1.PipelineRun{
				pendingRun("q1", nil, 1*time.Second),
				pendingRun("q2", nil, 10*time.Second),
			},
		}

		admit, cancel := applyStrategy(l, edpv1alpha1.QueueStrategyCancelInProgress, 2)

		// Both occupying runs predate the surviving queued run (q2, the
		// newest arrival) and are superseded by it.
		equalNames(t, "cancel", cancel, []string{"q1", "old", "newer"})
		// concurrency (2) - occupying (2) leaves no slots this pass, matching
		// the "state is derived live" design: a cancelled occupying run is
		// only dropped from occupying once a later reconcile observes it
		// actually cancelling.
		equalNames(t, "admit", admit, nil)
	})

	t.Run("same-second occupying run is still superseded via the name tiebreak", func(t *testing.T) {
		t.Parallel()

		// CreationTimestamp has one-second resolution, so a burst (e.g.
		// duplicate webhook deliveries) yields identical timestamps; the
		// occupying run must still be recognized as older via the FIFO
		// name tiebreak, or it escapes cancellation entirely.
		l := &lane{
			occupying: []*tektonv1.PipelineRun{runningRun("burst-1", nil, 0)},
			queued:    []*tektonv1.PipelineRun{pendingRun("burst-4", nil, 0)},
		}

		admit, cancel := applyStrategy(l, edpv1alpha1.QueueStrategyCancelInProgress, 1)

		equalNames(t, "cancel", cancel, []string{"burst-1"})
		equalNames(t, "admit", admit, nil)
	})

	t.Run("empty queued cancels nothing, even with occupying runs", func(t *testing.T) {
		t.Parallel()

		l := &lane{occupying: []*tektonv1.PipelineRun{runningRun("r1", nil, 0)}}

		admit, cancel := applyStrategy(l, edpv1alpha1.QueueStrategyCancelInProgress, 1)

		equalNames(t, "admit", admit, nil)
		equalNames(t, "cancel", cancel, nil)
	})

	t.Run("all occupying cancelled, none admitted until slots free up", func(t *testing.T) {
		t.Parallel()

		l := &lane{
			occupying: []*tektonv1.PipelineRun{runningRun("r1", nil, 0)},
			queued:    []*tektonv1.PipelineRun{pendingRun("q1", nil, 5*time.Second)},
		}

		admit, cancel := applyStrategy(l, edpv1alpha1.QueueStrategyCancelInProgress, 1)

		equalNames(t, "cancel", cancel, []string{"r1"})
		equalNames(t, "admit", admit, nil)
	})
}
