package main

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNamespaceMutator(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta1",
		"kind":       "Cluster",
		"metadata": map[string]any{
			"name":      "bex",
			"namespace": "default",
		},
		"spec": map[string]any{
			"controlPlaneRef": map[string]any{"name": "bex-control-plane", "namespace": "default"},
			"infrastructureRef": map[string]any{
				"name":      "bex",
				"namespace": "external-infra",
			},
		},
	}}

	if err := namespaceMutator("default", "bex-capi")(obj); err != nil {
		t.Fatalf("namespaceMutator: %v", err)
	}
	if got := obj.GetNamespace(); got != "bex-capi" {
		t.Fatalf("metadata.namespace = %q, want bex-capi", got)
	}
	if got, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneRef", "namespace"); got != "bex-capi" {
		t.Errorf("controlPlaneRef.namespace = %q, want bex-capi", got)
	}
	if got, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "namespace"); got != "external-infra" {
		t.Errorf("external infrastructure namespace changed to %q", got)
	}
}

func TestNamespaceMutatorLeavesOtherNamespacesAndMissingRefsAlone(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta1",
		"kind":       "MachineDeployment",
		"metadata": map[string]any{
			"name":      "other",
			"namespace": "another-cluster",
		},
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{}}},
	}}

	if err := namespaceMutator("default", "bex-capi")(obj); err != nil {
		t.Fatalf("namespaceMutator: %v", err)
	}
	if got := obj.GetNamespace(); got != "another-cluster" {
		t.Fatalf("metadata.namespace = %q, want another-cluster", got)
	}
}

func TestNamespaceMutatorRejectsMalformedReference(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cluster.x-k8s.io/v1beta1",
		"kind":       "Machine",
		"metadata":   map[string]any{"name": "bex-worker", "namespace": "default"},
		"spec": map[string]any{
			"infrastructureRef": map[string]any{"namespace": float64(1)},
		},
	}}

	if err := namespaceMutator("default", "bex-capi")(obj); err == nil {
		t.Fatal("namespaceMutator accepted a non-string namespace reference")
	}
}
