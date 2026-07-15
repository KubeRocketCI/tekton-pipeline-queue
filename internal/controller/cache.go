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
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CacheOptions returns the manager cache configuration shared by the
// operator binary and the test suite. The PipelineRun informer caches every
// run in the cluster, and full objects are large (managedFields plus the
// fully resolved pipeline spec Tekton inlines into status routinely reach
// tens of kilobytes); stripping the fields the reconciler never reads keeps
// the operator's memory proportional to run count, not run size.
func CacheOptions() cache.Options {
	return cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&tektonv1.PipelineRun{}: {Transform: stripPipelineRun},
		},
	}
}

// stripPipelineRun drops the PipelineRun fields the controller never reads
// before the object enters the cache.
//
// The reconciler relies on: metadata (name, namespace, labels, annotations,
// creation/deletion timestamps), spec.status, and status.conditions. If a
// future change reads any other field, it MUST be removed from this strip
// list — the field is empty in the cache (and in envtest, which uses the
// same options), not just occasionally missing.
func stripPipelineRun(obj any) (any, error) {
	pr, ok := obj.(*tektonv1.PipelineRun)
	if !ok {
		return obj, nil
	}

	pr.ManagedFields = nil

	pr.Spec.PipelineSpec = nil
	pr.Spec.Params = nil
	pr.Spec.Workspaces = nil
	pr.Spec.TaskRunSpecs = nil

	pr.Status.PipelineSpec = nil
	pr.Status.ChildReferences = nil
	pr.Status.Provenance = nil
	pr.Status.Results = nil
	pr.Status.SkippedTasks = nil
	pr.Status.SpanContext = nil

	return pr, nil
}
