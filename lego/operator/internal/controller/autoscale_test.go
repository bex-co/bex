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
	"encoding/json"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func ptr32(v int32) *int32 { return &v }

// fakePodMetricsList builds a minimal metrics-server PodMetricsList JSON body.
func fakePodMetricsList(items []struct{ cpu, mem string }) []byte {
	type usage struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	}
	type container struct {
		Name  string `json:"name"`
		Usage usage  `json:"usage"`
	}
	type item struct {
		Metadata   struct{ Name string `json:"name"` }   `json:"metadata"`
		Containers []container                            `json:"containers"`
	}
	type list struct {
		Items []item `json:"items"`
	}
	var l list
	for i, it := range items {
		l.Items = append(l.Items, item{
			Metadata: struct{ Name string `json:"name"` }{Name: "pod-" + string(rune('a'+i))},
			Containers: []container{
				{Name: "app", Usage: usage{CPU: it.cpu, Memory: it.mem}},
			},
		})
	}
	b, _ := json.Marshal(l)
	return b
}

func TestParsePodUsage(t *testing.T) {
	raw := fakePodMetricsList([]struct{ cpu, mem string }{
		{"100m", "128Mi"},
		{"250m", "256Mi"},
	})
	usage, err := parsePodUsage(raw)
	if err != nil {
		t.Fatalf("parsePodUsage: %v", err)
	}
	if len(usage) != 2 {
		t.Fatalf("len = %d, want 2", len(usage))
	}
	// pod-a: 0.1 CPU, 128 MiB
	if got := usage[0].CPUCores; got < 0.099 || got > 0.101 {
		t.Errorf("pod-a CPU = %.4f, want ~0.1", got)
	}
	if got := usage[0].MemoryBytes; got != float64(128*1024*1024) {
		t.Errorf("pod-a mem = %v, want 128 MiB", got)
	}
	// pod-b: 0.25 CPU, 256 MiB
	if got := usage[1].CPUCores; got < 0.249 || got > 0.251 {
		t.Errorf("pod-b CPU = %.4f, want ~0.25", got)
	}
}

func TestAutoscaleDesiredNilSpec(t *testing.T) {
	_, skip := autoscaleDesired(nil, []PodUsage{{Pod: "p", CPUCores: 0.5}}, "starter")
	if !skip {
		t.Fatal("nil spec should skip")
	}
}

func TestAutoscaleDesiredNoUsage(t *testing.T) {
	as := &appv1alpha1.AutoscalingSpec{
		Enabled:          true,
		MinReplicas:      1,
		MaxReplicas:      5,
		TargetCPUPercent: ptr32(80),
	}
	_, skip := autoscaleDesired(as, nil, "starter")
	if !skip {
		t.Fatal("empty usage should skip")
	}
}

func TestAutoscaleDesiredBestEffortTier(t *testing.T) {
	// Best-effort tier has no limits — should hold steady.
	as := &appv1alpha1.AutoscalingSpec{
		Enabled:          true,
		MinReplicas:      1,
		MaxReplicas:      5,
		TargetCPUPercent: ptr32(80),
	}
	usage := []PodUsage{{Pod: "p", CPUCores: 999, MemoryBytes: 999e9}}
	_, skip := autoscaleDesired(as, usage, "") // empty tier = best-effort
	if !skip {
		t.Fatal("best-effort tier should skip (no limits)")
	}
}

func TestAutoscaleDesiredScaleUp(t *testing.T) {
	// starter tier: 0.5 CPU, 512 MiB
	// target: 80% CPU → 0.5 * 0.8 = 0.4 CPU per pod target
	// current: 1 pod using 0.6 CPU → desired = ceil(1 * 0.6/0.4) = 2
	as := &appv1alpha1.AutoscalingSpec{
		Enabled:          true,
		MinReplicas:      1,
		MaxReplicas:      5,
		TargetCPUPercent: ptr32(80),
	}
	usage := []PodUsage{{Pod: "p", CPUCores: 0.6, MemoryBytes: 100 * 1024 * 1024}}
	got, skip := autoscaleDesired(as, usage, "starter")
	if skip {
		t.Fatal("should not skip for starter tier")
	}
	if got != 2 {
		t.Errorf("desired = %d, want 2", got)
	}
}

func TestAutoscaleDesiredScaleDown(t *testing.T) {
	// 3 pods, each using 0.05 CPU (very low).
	// starter: 0.5 CPU limit, target 80% → 0.4 CPU/pod target
	// avg usage per pod = 0.05 CPU
	// desired = ceil(3 * 0.05/0.4) = ceil(0.375) = 1
	as := &appv1alpha1.AutoscalingSpec{
		Enabled:          true,
		MinReplicas:      1,
		MaxReplicas:      5,
		TargetCPUPercent: ptr32(80),
	}
	usage := []PodUsage{
		{Pod: "p1", CPUCores: 0.05},
		{Pod: "p2", CPUCores: 0.05},
		{Pod: "p3", CPUCores: 0.05},
	}
	got, skip := autoscaleDesired(as, usage, "starter")
	if skip {
		t.Fatal("should not skip")
	}
	if got != 1 {
		t.Errorf("desired = %d, want 1", got)
	}
}

func TestAutoscaleDesiredClampToMax(t *testing.T) {
	// 1 pod, 10× over target → would be 10, but max is 3
	as := &appv1alpha1.AutoscalingSpec{
		Enabled:          true,
		MinReplicas:      1,
		MaxReplicas:      3,
		TargetCPUPercent: ptr32(10),
	}
	usage := []PodUsage{{Pod: "p", CPUCores: 0.5}} // starter limit 0.5; target 10% = 0.05
	got, skip := autoscaleDesired(as, usage, "starter")
	if skip {
		t.Fatal("should not skip")
	}
	if got != 3 {
		t.Errorf("desired = %d, want 3 (clamped to max)", got)
	}
}

func TestAutoscaleDesiredClampToMin(t *testing.T) {
	// Min replicas = 2; computed would be 1
	as := &appv1alpha1.AutoscalingSpec{
		Enabled:          true,
		MinReplicas:      2,
		MaxReplicas:      5,
		TargetCPUPercent: ptr32(80),
	}
	usage := []PodUsage{{Pod: "p", CPUCores: 0.05}}
	got, skip := autoscaleDesired(as, usage, "starter")
	if skip {
		t.Fatal("should not skip")
	}
	if got != 2 {
		t.Errorf("desired = %d, want 2 (clamped to min)", got)
	}
}

func TestAutoscaleDesiredMemoryMetric(t *testing.T) {
	// standard: 1 CPU, 2 Gi — target mem 50% → 1 Gi per pod
	// 1 pod using 1.5 Gi → desired = ceil(1 * 1.5/1.0) = 2
	as := &appv1alpha1.AutoscalingSpec{
		Enabled:             true,
		MinReplicas:         1,
		MaxReplicas:         5,
		TargetMemoryPercent: ptr32(50),
	}
	usage := []PodUsage{{Pod: "p", CPUCores: 0.1, MemoryBytes: 1.5 * 1024 * 1024 * 1024}}
	got, skip := autoscaleDesired(as, usage, "standard")
	if skip {
		t.Fatal("should not skip")
	}
	if got != 2 {
		t.Errorf("desired = %d, want 2", got)
	}
}

func TestAutoscaleDesiredBothMetrics(t *testing.T) {
	// CPU says scale to 2, memory says scale to 3 → max wins → 3
	as := &appv1alpha1.AutoscalingSpec{
		Enabled:             true,
		MinReplicas:         1,
		MaxReplicas:         5,
		TargetCPUPercent:    ptr32(80),
		TargetMemoryPercent: ptr32(50),
	}
	// starter: 0.5 CPU, 512 MiB
	// CPU: target 0.4, avg usage 0.6 → desired = ceil(1*0.6/0.4) = 2
	// mem: target 256 MiB (50% of 512), avg usage 768 MiB → desired = ceil(1*768/256) = 3
	usage := []PodUsage{{Pod: "p", CPUCores: 0.6, MemoryBytes: 768 * 1024 * 1024}}
	got, skip := autoscaleDesired(as, usage, "starter")
	if skip {
		t.Fatal("should not skip")
	}
	if got != 3 {
		t.Errorf("desired = %d, want 3 (max of cpu=2, mem=3)", got)
	}
}
