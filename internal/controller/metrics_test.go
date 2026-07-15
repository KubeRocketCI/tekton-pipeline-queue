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

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestDeleteQueueMetrics(t *testing.T) {
	queueDepth.WithLabelValues("q1", "ns1", "laneA").Set(3)
	queueDepth.WithLabelValues("q1", "ns1", "laneB").Set(1)
	queueRunning.WithLabelValues("q1", "ns1", "laneA").Set(1)
	// A different queue in the same namespace must survive the cleanup.
	queueDepth.WithLabelValues("q2", "ns1", "laneA").Set(2)

	deleteQueueMetrics("q1", "ns1")

	if got := testutil.CollectAndCount(queueDepth); got != 1 {
		t.Errorf("expected 1 remaining queueDepth series, got %d", got)
	}

	if got := testutil.CollectAndCount(queueRunning); got != 0 {
		t.Errorf("expected 0 remaining queueRunning series, got %d", got)
	}

	deleteQueueMetrics("q2", "ns1")

	if got := testutil.CollectAndCount(queueDepth); got != 0 {
		t.Errorf("expected 0 remaining queueDepth series, got %d", got)
	}
}
