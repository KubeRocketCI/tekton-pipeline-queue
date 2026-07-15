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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	edpv1alpha1 "github.com/KubeRocketCI/tekton-pipeline-queue/api/v1alpha1"
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

func TestDeleteStaleLaneMetrics(t *testing.T) {
	queue := &edpv1alpha1.PipelineRunQueue{
		ObjectMeta: metav1.ObjectMeta{Name: "q3", Namespace: "ns3"},
		Status: edpv1alpha1.PipelineRunQueueStatus{
			Lanes: []edpv1alpha1.LaneStatus{{Key: "gone"}, {Key: "kept"}},
		},
	}

	queueDepth.WithLabelValues("q3", "ns3", "gone").Set(2)
	queueDepth.WithLabelValues("q3", "ns3", "kept").Set(1)
	queueRunning.WithLabelValues("q3", "ns3", "gone").Set(1)

	deleteStaleLaneMetrics(queue, map[string]*lane{"kept": {}})

	if got := testutil.CollectAndCount(queueDepth); got != 1 {
		t.Errorf("expected only the kept lane's queueDepth series, got %d", got)
	}

	if got := testutil.CollectAndCount(queueRunning); got != 0 {
		t.Errorf("expected the stale queueRunning series removed, got %d", got)
	}

	deleteQueueMetrics("q3", "ns3")
}
