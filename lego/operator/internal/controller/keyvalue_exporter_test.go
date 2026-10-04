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
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// exporterContainer projects a KeyValue pod spec and returns its metrics
// sidecar, failing if the sidecar is missing.
func exporterContainer(t *testing.T, spec *corev1.PodSpec) corev1.Container {
	t.Helper()
	for _, c := range spec.Containers {
		if c.Name == "metrics" {
			return c
		}
	}
	t.Fatalf("no metrics sidecar in %d containers", len(spec.Containers))
	return corev1.Container{}
}

// TestKVExporterBudgetIsGuaranteedAndNotStarved pins w4/181: the exporter
// sidecar's budget is 50m/32Mi with requests == limits, so the projected pod
// stays Guaranteed QoS at every plan, and the CPU limit is far enough above the
// old 10m (which CFS-throttled ~98% and timed scrapes out) to finish a scrape.
func TestKVExporterBudgetIsGuaranteedAndNotStarved(t *testing.T) {
	for _, planName := range append([]string{""}, tiers.Valkey.IDs()...) {
		kv := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: "red-x", Namespace: "ws"},
			Spec:       appv1alpha1.KeyValueSpec{Plan: planName},
		}
		plan, _ := resolveKVPlan(kv.Spec)
		var spec corev1.PodSpec
		applyValkeyPodSpec(&spec, kv, keyValueIntent{plan: plan, authSecretName: "red-x-auth"})

		exp := exporterContainer(t, &spec)
		want := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("32Mi"),
		}
		for name, q := range want {
			if got := exp.Resources.Limits[name]; got.Cmp(q) != 0 {
				t.Errorf("plan %q: exporter limit %s = %s, want %s", planName, name, got.String(), q.String())
			}
		}
		// Guaranteed QoS requires every container's requests == limits for
		// both cpu and memory; one Burstable container demotes the whole pod.
		for _, c := range spec.Containers {
			for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
				req, lim := c.Resources.Requests[name], c.Resources.Limits[name]
				if lim.IsZero() || req.Cmp(lim) != 0 {
					t.Errorf("plan %q: container %q %s request %s != limit %s — pod is not Guaranteed QoS",
						planName, c.Name, name, req.String(), lim.String())
				}
			}
		}
	}
}

// TestKVExporterRunsOnlyConsumedCollectors pins the collector trim (w4/181):
// the sidecar keeps INFO (memory + connected clients) and turns off the
// per-scrape work no consumer reads. Values are the pinned v1.89.0 env names.
func TestKVExporterRunsOnlyConsumedCollectors(t *testing.T) {
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-x", Namespace: "ws"}}
	plan, _ := resolveKVPlan(kv.Spec)
	var spec corev1.PodSpec
	applyValkeyPodSpec(&spec, kv, keyValueIntent{plan: plan, authSecretName: "red-x-auth"})
	exp := exporterContainer(t, &spec)

	env := map[string]corev1.EnvVar{}
	for _, e := range exp.Env {
		env[e.Name] = e
	}
	for name, want := range map[string]string{
		"REDIS_ADDR":                                       "redis://localhost:6379",
		"REDIS_EXPORTER_REDIS_ONLY_METRICS":                "true",
		"REDIS_EXPORTER_EXCLUDE_LATENCY_HISTOGRAM_METRICS": "true",
		"REDIS_EXPORTER_CONFIG_COMMAND":                    "-",
		"REDIS_EXPORTER_INCL_METRICS_FOR_EMPTY_DATABASES":  "false",
	} {
		if got, ok := env[name]; !ok || got.Value != want {
			t.Errorf("exporter env %s = %q (set=%v), want %q", name, got.Value, ok, want)
		}
	}
	pw, ok := env["REDIS_PASSWORD"]
	if !ok || pw.ValueFrom == nil || pw.ValueFrom.SecretKeyRef == nil ||
		pw.ValueFrom.SecretKeyRef.Name != "red-x-auth" || pw.Value != "" {
		t.Errorf("REDIS_PASSWORD must come from the auth Secret, never a literal: %+v", pw)
	}
}

// infoDerivedRedisMetrics are the redis_exporter series that survive the
// collector trim above: they come from INFO, which the exporter always runs.
// A consumer of anything else (redis_config_*, redis_latency_*, …) would read
// an empty series — extend kvExporterEnv deliberately, then this list.
var infoDerivedRedisMetrics = map[string]bool{
	"redis_memory_used_bytes": true,
	"redis_connected_clients": true,
}

var redisMetricRE = regexp.MustCompile(`\bredis_[a-z_]+\b`)

// TestKVExporterTrimKeepsEveryConsumedSeries scans the consumers of the
// valkey-instances job — the backend's Key Value metrics source and the GitOps
// Prometheus config/rules — so a new dashboard query or alert on a series the
// trim disables fails here instead of silently rendering empty.
func TestKVExporterTrimKeepsEveryConsumedSeries(t *testing.T) {
	gitops, err := filepath.Glob("../../../../deploy/gitops/base/*.yaml")
	if err != nil || len(gitops) == 0 {
		t.Fatalf("no gitops manifests found (err=%v)", err)
	}
	files := append([]string{"../../../backend/internal/metrics/source.go"}, gitops...)
	seen := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range redisMetricRE.FindAllString(string(src), -1) {
			if m == "redis_exporter" {
				continue // the image/component name, not a series
			}
			seen++
			if !infoDerivedRedisMetrics[m] {
				t.Errorf("%s consumes %s, which the trimmed exporter (kvExporterEnv) does not emit", f, m)
			}
		}
	}
	if seen == 0 {
		t.Fatal("found no redis_* consumers — the scan paths are stale")
	}
}
