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
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testLabelBranch = "branch"
	testLabelRepo   = "repo"

	testBranchMain    = "main"
	testBranchFeature = "feature"
)

func withCreation(pr *tektonv1.PipelineRun, offset time.Duration) *tektonv1.PipelineRun {
	pr.CreationTimestamp = metav1.NewTime(time.Unix(1_700_000_000, 0).Add(offset))
	return pr
}

func pendingRun(name string, labels map[string]string, offset time.Duration) *tektonv1.PipelineRun {
	return withCreation(&tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       tektonv1.PipelineRunSpec{Status: tektonv1.PipelineRunSpecStatusPending},
	}, offset)
}

func runningRun(name string, labels map[string]string, offset time.Duration) *tektonv1.PipelineRun {
	return withCreation(&tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
	}, offset)
}

func cancellingRun(name string, status tektonv1.PipelineRunSpecStatus) *tektonv1.PipelineRun {
	return withCreation(&tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       tektonv1.PipelineRunSpec{Status: status},
	}, 0)
}

func doneRun(name string, succeeded corev1.ConditionStatus, offset time.Duration) *tektonv1.PipelineRun {
	pr := withCreation(&tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}, offset)
	pr.Status.Conditions = duckv1.Conditions{{Type: apis.ConditionSucceeded, Status: succeeded}}

	return pr
}

func TestLaneKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		labels   map[string]string
		queueKey []string
		want     string
	}{
		{name: "empty queue key yields single lane", labels: map[string]string{testLabelBranch: testBranchMain}, queueKey: nil, want: ""},
		{name: "single key", labels: map[string]string{testLabelBranch: testBranchMain}, queueKey: []string{testLabelBranch}, want: testBranchMain},
		{
			name:     "multiple keys joined with slash",
			labels:   map[string]string{testLabelRepo: "svc", testLabelBranch: testBranchMain},
			queueKey: []string{testLabelRepo, testLabelBranch},
			want:     "svc/main",
		},
		{
			name:     "missing label contributes empty segment",
			labels:   map[string]string{testLabelRepo: "svc"},
			queueKey: []string{testLabelRepo, testLabelBranch},
			want:     "svc/",
		},
		{name: "all labels missing", labels: nil, queueKey: []string{testLabelRepo, testLabelBranch}, want: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pr := &tektonv1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Labels: tt.labels}}
			if got := laneKey(pr, tt.queueKey); got != tt.want {
				t.Errorf("laneKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		pr            *tektonv1.PipelineRun
		wantQueued    bool
		wantOccupying bool
	}{
		{name: "pending is queued", pr: pendingRun("a", nil, 0), wantQueued: true, wantOccupying: false},
		{name: "no status, no conditions is occupying", pr: runningRun("a", nil, 0), wantQueued: false, wantOccupying: true},
		{
			name:          "cancelled is neither",
			pr:            cancellingRun("a", tektonv1.PipelineRunSpecStatusCancelled),
			wantQueued:    false,
			wantOccupying: false,
		},
		{
			name:          "gracefully cancelled frees the slot immediately",
			pr:            cancellingRun("a", tektonv1.PipelineRunSpecStatusCancelledRunFinally),
			wantQueued:    false,
			wantOccupying: false,
		},
		{
			name:          "gracefully stopped frees the slot immediately",
			pr:            cancellingRun("a", tektonv1.PipelineRunSpecStatusStoppedRunFinally),
			wantQueued:    false,
			wantOccupying: false,
		},
		{
			name:          "succeeded is done, neither queued nor occupying",
			pr:            doneRun("a", corev1.ConditionTrue, 0),
			wantQueued:    false,
			wantOccupying: false,
		},
		{
			name:          "failed is done, neither queued nor occupying",
			pr:            doneRun("a", corev1.ConditionFalse, 0),
			wantQueued:    false,
			wantOccupying: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isQueued(tt.pr); got != tt.wantQueued {
				t.Errorf("isQueued() = %v, want %v", got, tt.wantQueued)
			}

			if got := isOccupying(tt.pr); got != tt.wantOccupying {
				t.Errorf("isOccupying() = %v, want %v", got, tt.wantOccupying)
			}
		})
	}
}

func TestBucketLanes(t *testing.T) {
	t.Parallel()

	runs := []tektonv1.PipelineRun{
		*pendingRun("run-c", map[string]string{testLabelBranch: testBranchMain}, 3*time.Second),
		*pendingRun("run-a", map[string]string{testLabelBranch: testBranchMain}, 1*time.Second),
		*pendingRun("run-b", map[string]string{testLabelBranch: testBranchMain}, 1*time.Second), // same timestamp as run-a
		*runningRun("run-running", map[string]string{testLabelBranch: testBranchMain}, 0),
		*doneRun("run-done", corev1.ConditionTrue, 0),
		*cancellingRun("run-cancelled", tektonv1.PipelineRunSpecStatusCancelledRunFinally),
		*pendingRun("run-other-lane", map[string]string{testLabelBranch: testBranchFeature}, 0),
	}

	lanes := bucketLanes(runs, []string{testLabelBranch})

	if len(lanes) != 2 {
		t.Fatalf("expected 2 lanes, got %d: %v", len(lanes), lanes)
	}

	main, ok := lanes[testBranchMain]
	if !ok {
		t.Fatal("expected a main lane")
	}

	if len(main.occupying) != 1 || main.occupying[0].Name != "run-running" {
		t.Errorf("unexpected occupying set in main lane: %v", main.occupying)
	}

	// run-a and run-b share a timestamp; tiebreak by Name puts run-a first.
	gotOrder := []string{main.queued[0].Name, main.queued[1].Name, main.queued[2].Name}
	wantOrder := []string{"run-a", "run-b", "run-c"}

	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("queued order = %v, want %v", gotOrder, wantOrder)
			break
		}
	}

	feature, ok := lanes[testBranchFeature]
	if !ok {
		t.Fatal("expected a feature lane")
	}

	if len(feature.queued) != 1 || feature.queued[0].Name != "run-other-lane" {
		t.Errorf("unexpected feature lane queued set: %v", feature.queued)
	}
}
