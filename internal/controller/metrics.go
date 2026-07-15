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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	edpv1alpha1 "github.com/KubeRocketCI/tekton-pipeline-queue/api/v1alpha1"
)

// Metric names are exported as constants so the metric identity below stays
// in one place and can be referenced by tests.
const (
	metricQueueDepth         = "tekton_pipeline_queue_depth"
	metricQueueRunning       = "tekton_pipeline_queue_running"
	metricAdmissionsTotal    = "tekton_pipeline_queue_admissions_total"
	metricCancellationsTotal = "tekton_pipeline_queue_cancellations_total"
	metricTimeInQueue        = "tekton_pipeline_queue_time_in_queue_seconds"
)

// Prometheus label names shared across the metrics below.
const (
	metricLabelQueue     = "queue"
	metricLabelNamespace = "namespace"
	metricLabelLane      = "lane"
	metricLabelStrategy  = "strategy"
)

var (
	queueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricQueueDepth,
		Help: "Number of PipelineRuns currently queued (pending admission) in a lane.",
	}, []string{metricLabelQueue, metricLabelNamespace, metricLabelLane})

	queueRunning = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricQueueRunning,
		Help: "Number of PipelineRuns currently occupying a lane's concurrency slots.",
	}, []string{metricLabelQueue, metricLabelNamespace, metricLabelLane})

	admissionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: metricAdmissionsTotal,
		Help: "Total number of PipelineRuns admitted (spec.status cleared) by the queue controller.",
	}, []string{metricLabelQueue, metricLabelNamespace})

	cancellationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: metricCancellationsTotal,
		Help: "Total number of PipelineRuns cancelled by the queue controller, by strategy.",
	}, []string{metricLabelQueue, metricLabelNamespace, metricLabelStrategy})

	timeInQueue = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: metricTimeInQueue,
		Help: "Time a PipelineRun spent queued before admission, observed at admission time.",
		// 1s .. ~1h.
		Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 1200, 1800, 3600},
	}, []string{metricLabelQueue, metricLabelNamespace})
)

func init() {
	metrics.Registry.MustRegister(
		queueDepth,
		queueRunning,
		admissionsTotal,
		cancellationsTotal,
		timeInQueue,
	)
}

// resetLaneMetrics drops every depth/running gauge series previously
// exported for queue. The caller re-Sets the current lanes right after,
// which keeps stale (disappeared) lanes from lingering in exported metrics
// without having to diff against the previous reconcile's lane set.
func resetLaneMetrics(queue *edpv1alpha1.PipelineRunQueue) {
	deleteQueueMetrics(queue.Name, queue.Namespace)
}

// deleteQueueMetrics removes all depth/running gauge series for a queue.
// Called when a queue is deleted or stops being reconcilable (invalid
// selector), so dashboards don't keep reporting the last-observed values
// for a queue that no longer produces them. Counters/histograms are kept:
// totals remain meaningful after deletion and are cheap to retain.
func deleteQueueMetrics(name, namespace string) {
	matchQueue := prometheus.Labels{metricLabelQueue: name, metricLabelNamespace: namespace}

	queueDepth.DeletePartialMatch(matchQueue)
	queueRunning.DeletePartialMatch(matchQueue)
}
