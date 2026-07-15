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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	edpv1alpha1 "github.com/KubeRocketCI/tekton-pipeline-queue/api/v1alpha1"
)

// There is no Tekton controller running in envtest, so "running"/"occupying"
// PipelineRuns in these tests are simulated directly: a run whose
// spec.status the queue controller cleared (or that we created already
// clear) and that carries no terminal Succeeded condition. Completion is
// simulated by patching status.conditions; deletion by deleting the object.
const (
	testTimeout  = 10 * time.Second
	testInterval = 100 * time.Millisecond

	// creationGap is slept between sequential creates that must land in a
	// deterministic FIFO order. bucketLanes ties break on Name, which by
	// itself would already give us a deterministic order for these tests,
	// but sleeping keeps CreationTimestamp itself monotonic too, matching
	// real-world FIFO semantics rather than relying solely on the tiebreak.
	creationGap = 1100 * time.Millisecond

	testLabelLane  = "lane"
	testLabelQueue = "queue"
)

// laneLabels builds the single-label selector/queueKey map used by most
// scenarios below, keyed on testLabelLane.
func laneLabels(value string) map[string]string {
	return map[string]string{testLabelLane: value}
}

var _ = Describe("PipelineRunQueue admission controller", func() {
	var namespace string

	BeforeEach(func() {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "prq-test-"}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		namespace = ns.Name
	})

	It("admits the oldest pending run first (FIFO) and keeps the rest queued", func() {
		Expect(k8sClient.Create(ctx, newQueue("fifo", namespace, laneLabels("fifo"), nil, 1,
			edpv1alpha1.QueueStrategyQueue))).To(Succeed())

		runs := createSequentially(namespace, laneLabels("fifo"), 3)

		expectSpecStatus(namespace, runs[0].Name, "")
		expectSpecStatus(namespace, runs[1].Name, tektonv1.PipelineRunSpecStatusPending)
		expectSpecStatus(namespace, runs[2].Name, tektonv1.PipelineRunSpecStatusPending)

		Eventually(func() *edpv1alpha1.LaneStatus {
			return laneStatus(namespace, "fifo", "")
		}, testTimeout, testInterval).Should(And(
			Not(BeNil()),
			WithTransform(func(l *edpv1alpha1.LaneStatus) []string { return l.Running }, ConsistOf(runs[0].Name)),
			WithTransform(func(l *edpv1alpha1.LaneStatus) []string { return l.Queued }, Equal([]string{runs[1].Name, runs[2].Name})),
		))
	})

	It("promotes the next queued run once the admitted run completes", func() {
		Expect(k8sClient.Create(ctx, newQueue("completion", namespace, laneLabels("completion"), nil, 1,
			edpv1alpha1.QueueStrategyQueue))).To(Succeed())

		runs := createSequentially(namespace, laneLabels("completion"), 2)

		expectSpecStatus(namespace, runs[0].Name, "")
		expectSpecStatus(namespace, runs[1].Name, tektonv1.PipelineRunSpecStatusPending)

		markSucceeded(namespace, runs[0].Name)

		expectSpecStatus(namespace, runs[1].Name, "")
	})

	It("promotes the next queued run once the admitted run is deleted", func() {
		Expect(k8sClient.Create(ctx, newQueue("deletion", namespace, laneLabels("deletion"), nil, 1,
			edpv1alpha1.QueueStrategyQueue))).To(Succeed())

		runs := createSequentially(namespace, laneLabels("deletion"), 2)

		expectSpecStatus(namespace, runs[0].Name, "")
		expectSpecStatus(namespace, runs[1].Name, tektonv1.PipelineRunSpecStatusPending)

		Expect(k8sClient.Delete(ctx, runs[0])).To(Succeed())

		expectSpecStatus(namespace, runs[1].Name, "")
	})

	It("isolates lanes: independent branches admit concurrently", func() {
		const queueName = "lanes"

		Expect(k8sClient.Create(ctx, newQueue(queueName, namespace, map[string]string{testLabelQueue: queueName}, []string{testLabelBranch}, 1,
			edpv1alpha1.QueueStrategyQueue))).To(Succeed())

		runA := newPipelineRun("run-a", namespace, map[string]string{testLabelQueue: queueName, testLabelBranch: "a"}, true)
		Expect(k8sClient.Create(ctx, runA)).To(Succeed())

		runB := newPipelineRun("run-b", namespace, map[string]string{testLabelQueue: queueName, testLabelBranch: "b"}, true)
		Expect(k8sClient.Create(ctx, runB)).To(Succeed())

		expectSpecStatus(namespace, runA.Name, "")
		expectSpecStatus(namespace, runB.Name, "")
	})

	It("ReplaceQueued cancels all but the newest queued run and leaves the running run alone", func() {
		const laneName = "replace"

		Expect(k8sClient.Create(ctx, newQueue(laneName, namespace, laneLabels(laneName), nil, 1,
			edpv1alpha1.QueueStrategyReplaceQueued))).To(Succeed())

		running := newPipelineRun("running", namespace, laneLabels(laneName), false)
		Expect(k8sClient.Create(ctx, running)).To(Succeed())

		queuedRuns := createSequentially(namespace, laneLabels(laneName), 3)

		expectSpecStatus(namespace, queuedRuns[0].Name, tektonv1.PipelineRunSpecStatusCancelledRunFinally)
		expectSpecStatus(namespace, queuedRuns[1].Name, tektonv1.PipelineRunSpecStatusCancelledRunFinally)
		expectSpecStatus(namespace, queuedRuns[2].Name, tektonv1.PipelineRunSpecStatusPending)
		expectSpecStatus(namespace, running.Name, "")
	})

	It("frees the lane slot as soon as a running run is flagged for cancellation, without waiting for a terminal condition", func() {
		const laneName = "cancel-flag"

		Expect(k8sClient.Create(ctx, newQueue(laneName, namespace, laneLabels(laneName), nil, 1,
			edpv1alpha1.QueueStrategyQueue))).To(Succeed())

		running := newPipelineRun("running", namespace, laneLabels(laneName), false)
		Expect(k8sClient.Create(ctx, running)).To(Succeed())

		pending := newPipelineRun("pending", namespace, laneLabels(laneName), true)
		Expect(k8sClient.Create(ctx, pending)).To(Succeed())

		expectSpecStatus(namespace, pending.Name, tektonv1.PipelineRunSpecStatusPending)

		patchSpecStatus(namespace, running.Name, tektonv1.PipelineRunSpecStatusCancelledRunFinally)

		expectSpecStatus(namespace, pending.Name, "")
	})

	It("admits up to concurrency runs at once", func() {
		Expect(k8sClient.Create(ctx, newQueue("concurrency", namespace, laneLabels("concurrency"), nil, 2,
			edpv1alpha1.QueueStrategyQueue))).To(Succeed())

		runs := createSequentially(namespace, laneLabels("concurrency"), 2)

		expectSpecStatus(namespace, runs[0].Name, "")
		expectSpecStatus(namespace, runs[1].Name, "")
	})
})

func newQueue(
	name, namespace string,
	selector map[string]string,
	queueKey []string,
	concurrency int32,
	strategy edpv1alpha1.QueueStrategy,
) *edpv1alpha1.PipelineRunQueue {
	return &edpv1alpha1.PipelineRunQueue{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: edpv1alpha1.PipelineRunQueueSpec{
			Selector:    metav1.LabelSelector{MatchLabels: selector},
			QueueKey:    queueKey,
			Concurrency: concurrency,
			Strategy:    strategy,
		},
	}
}

func newPipelineRun(name, namespace string, labels map[string]string, pending bool) *tektonv1.PipelineRun {
	pr := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: tektonv1.PipelineRunSpec{
			PipelineRef: &tektonv1.PipelineRef{Name: "dummy-pipeline"},
		},
	}

	if pending {
		pr.Spec.Status = tektonv1.PipelineRunSpecStatusPending
	}

	return pr
}

// createSequentially creates n pending PipelineRuns named run-1..run-n in
// creation order, sleeping creationGap between each so CreationTimestamp
// order matches Name order.
func createSequentially(namespace string, labels map[string]string, n int) []*tektonv1.PipelineRun {
	runs := make([]*tektonv1.PipelineRun, 0, n)

	for i := 1; i <= n; i++ {
		pr := newPipelineRun(fmt.Sprintf("run-%d", i), namespace, labels, true)
		Expect(k8sClient.Create(ctx, pr)).To(Succeed())

		runs = append(runs, pr)

		if i < n {
			time.Sleep(creationGap)
		}
	}

	return runs
}

func expectSpecStatus(namespace, name string, want tektonv1.PipelineRunSpecStatus) {
	GinkgoHelper()

	Eventually(func(g Gomega) tektonv1.PipelineRunSpecStatus {
		pr := &tektonv1.PipelineRun{}
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, pr)).To(Succeed())

		return pr.Spec.Status
	}, testTimeout, testInterval).Should(Equal(want))
}

func patchSpecStatus(namespace, name string, status tektonv1.PipelineRunSpecStatus) {
	GinkgoHelper()

	pr := &tektonv1.PipelineRun{}
	Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, pr)).To(Succeed())

	patch := client.MergeFrom(pr.DeepCopy())
	pr.Spec.Status = status
	Expect(k8sClient.Patch(ctx, pr, patch)).To(Succeed())
}

func markSucceeded(namespace, name string) {
	GinkgoHelper()

	pr := &tektonv1.PipelineRun{}
	Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, pr)).To(Succeed())

	pr.Status.Conditions = duckv1.Conditions{{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	}}
	Expect(k8sClient.Status().Update(ctx, pr)).To(Succeed())
}

// laneStatus returns the lane with the given key from the named
// PipelineRunQueue's status, or nil if the queue or lane does not (yet)
// exist.
func laneStatus(namespace, queueName, key string) *edpv1alpha1.LaneStatus {
	queue := &edpv1alpha1.PipelineRunQueue{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: queueName}, queue); err != nil {
		return nil
	}

	for i := range queue.Status.Lanes {
		if queue.Status.Lanes[i].Key == key {
			return &queue.Status.Lanes[i]
		}
	}

	return nil
}
