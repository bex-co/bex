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
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type stubGetter struct {
	err error
}

func (s stubGetter) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return s.err
}

func clusterBuilderScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	scheme.AddKnownTypeWithName(build.ClusterBuilderGVK, &unstructured.Unstructured{})
	return scheme
}

func clusterBuilderWithReady(status corev1.ConditionStatus) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kpack.io/v1alpha2",
		"kind":       "ClusterBuilder",
		"metadata":   map[string]any{"name": build.ClusterBuilderName},
	}}
	if status != "" {
		obj.Object["status"] = map[string]any{
			"conditions": []any{map[string]any{
				"type": kpackReadyType, "status": string(status),
			}},
		}
	}
	return obj
}

func gatherClusterBuilder(t *testing.T, get clusterBuilderGetter, resolvedAt time.Time) map[string]float64 {
	t.Helper()
	reg := prometheus.NewPedanticRegistry()
	if err := reg.Register(NewClusterBuilderCollector(get, resolvedAt)); err != nil {
		t.Fatal(err)
	}
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, mf := range mfs {
		if len(mf.Metric) != 1 || mf.Metric[0].Gauge == nil || mf.Metric[0].Gauge.Value == nil {
			t.Fatalf("%s: want one unlabeled gauge, got %#v", mf.GetName(), mf.Metric)
		}
		if len(mf.Metric[0].Label) != 0 {
			t.Fatalf("%s: labels must stay empty, got %v", mf.GetName(), mf.Metric[0].Label)
		}
		out[mf.GetName()] = mf.Metric[0].Gauge.GetValue()
	}
	return out
}

func TestClusterBuilderMetricsReadyUnreadyMissingUnknown(t *testing.T) {
	scheme := clusterBuilderScheme(t)
	resolved := time.Unix(1_700_000_000, 0).UTC()

	readyClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterBuilderWithReady(corev1.ConditionTrue)).Build()
	got := gatherClusterBuilder(t, readyClient, resolved)
	if got["bex_build_clusterbuilder_present"] != clusterBuilderPresentYes {
		t.Errorf("present(ready) = %v", got["bex_build_clusterbuilder_present"])
	}
	if got["bex_build_clusterbuilder_ready"] != clusterBuilderReadyTrue {
		t.Errorf("ready(true) = %v", got["bex_build_clusterbuilder_ready"])
	}

	unreadyClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterBuilderWithReady(corev1.ConditionFalse)).Build()
	got = gatherClusterBuilder(t, unreadyClient, resolved)
	if got["bex_build_clusterbuilder_ready"] != clusterBuilderReadyFalse {
		t.Errorf("ready(false) = %v", got["bex_build_clusterbuilder_ready"])
	}

	unknownClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterBuilderWithReady(corev1.ConditionUnknown)).Build()
	got = gatherClusterBuilder(t, unknownClient, resolved)
	if got["bex_build_clusterbuilder_ready"] != clusterBuilderReadyUnknown {
		t.Errorf("ready(unknown) = %v", got["bex_build_clusterbuilder_ready"])
	}

	missingClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	got = gatherClusterBuilder(t, missingClient, resolved)
	if got["bex_build_clusterbuilder_present"] != clusterBuilderPresentNo {
		t.Errorf("present(missing) = %v", got["bex_build_clusterbuilder_present"])
	}
	if got["bex_build_clusterbuilder_ready"] != clusterBuilderReadyUnknown {
		t.Errorf("ready(missing) = %v", got["bex_build_clusterbuilder_ready"])
	}

	got = gatherClusterBuilder(t, stubGetter{err: errors.New("apiserver down")}, resolved)
	if got["bex_build_clusterbuilder_present"] != clusterBuilderPresentErr {
		t.Errorf("present(error) = %v", got["bex_build_clusterbuilder_present"])
	}
	if got["bex_build_clusterbuilder_ready"] != clusterBuilderReadyUnknown {
		t.Errorf("ready(error) = %v", got["bex_build_clusterbuilder_ready"])
	}

	noCondition := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterBuilderWithReady("")).Build()
	got = gatherClusterBuilder(t, noCondition, resolved)
	if got["bex_build_clusterbuilder_present"] != clusterBuilderPresentYes {
		t.Errorf("present(no condition) = %v", got["bex_build_clusterbuilder_present"])
	}
	if got["bex_build_clusterbuilder_ready"] != clusterBuilderReadyUnknown {
		t.Errorf("ready(no condition) = %v", got["bex_build_clusterbuilder_ready"])
	}
}

func TestClusterBuilderImageAgeUsesCommittedMetadata(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(clusterBuilderScheme(t)).WithObjects(clusterBuilderWithReady(corev1.ConditionTrue)).Build()

	resolved := time.Unix(1_700_000_000, 0).UTC()
	got := gatherClusterBuilder(t, cl, resolved)
	if got["bex_build_clusterbuilder_image_resolved_timestamp_seconds"] != float64(resolved.Unix()) {
		t.Fatalf("resolved timestamp = %v, want %d", got["bex_build_clusterbuilder_image_resolved_timestamp_seconds"], resolved.Unix())
	}

	got = gatherClusterBuilder(t, cl, time.Time{})
	if got["bex_build_clusterbuilder_image_resolved_timestamp_seconds"] != 0 {
		t.Fatalf("malformed metadata must export timestamp 0, got %v", got["bex_build_clusterbuilder_image_resolved_timestamp_seconds"])
	}

	later := resolved.Add(30 * 24 * time.Hour)
	if later.Unix() <= resolved.Unix() {
		t.Fatal("age must advance as wall time moves past committed resolved_at")
	}
}
