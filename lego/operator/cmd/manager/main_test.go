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

package main

import (
	"net"
	"os"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestPositiveEnvInt(t *testing.T) {
	const key = "BEX_TEST_POSITIVE_ENV_INT"
	for _, tc := range []struct {
		name  string
		value string
		want  int
	}{
		{name: "unset", value: "", want: 1},
		{name: "positive", value: "2", want: 2},
		{name: "zero", value: "0", want: 1},
		{name: "negative", value: "-3", want: 1},
		{name: "malformed", value: "many", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(key, tc.value)
			if got := positiveEnvInt(key, 1); got != tc.want {
				t.Fatalf("positiveEnvInt(%q, 1) = %d, want %d", key, got, tc.want)
			}
		})
	}
}

// managerContainer reads the manager container from the shipped manifest, the
// one production runs.
func managerContainer(t *testing.T) corev1.Container {
	t.Helper()
	raw, err := os.ReadFile("../../config/manager/manager.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for doc := range strings.SplitSeq(string(raw), "\n---\n") {
		var dep appsv1.Deployment
		if yaml.Unmarshal([]byte(doc), &dep) != nil || dep.Kind != "Deployment" {
			continue
		}
		for _, c := range dep.Spec.Template.Spec.Containers {
			if c.Name == "manager" {
				return c
			}
		}
	}
	t.Fatal("config/manager/manager.yaml has no manager container")
	return corev1.Container{}
}

// The pprof endpoint is unauthenticated: it may only listen on the pod's own
// loopback, where `kubectl port-forward` reaches it and no other pod does.
func TestManagerServesPprofOnLoopbackOnly(t *testing.T) {
	var addr string
	for _, a := range managerContainer(t).Args {
		if v, ok := strings.CutPrefix(a, "--pprof-bind-address="); ok {
			addr = v
		}
	}
	if addr == "" {
		t.Fatal("the manager does not enable pprof")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("pprof address %q: %v", addr, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		t.Fatalf("pprof binds %q: it must listen on a loopback address, never the pod network", addr)
	}
}

// minManagerCPUMilli leaves room for the manager's reconcile bursts: at 500m it
// was CFS-throttled in 92-99% of its periods and its App queue waited minutes
// (w21/m421 t001).
const minManagerCPUMilli = 1000

func TestManagerCPULimitLeavesRoomForBursts(t *testing.T) {
	limits := managerContainer(t).Resources.Limits
	if got := limits.Cpu().MilliValue(); got < minManagerCPUMilli {
		t.Fatalf("the manager's CPU limit is %dm, below %dm: its bursts are throttled and the App queue waits (w21/m421)",
			got, minManagerCPUMilli)
	}
}
