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
	"path/filepath"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// w6/m469: the manager hashes the proxy binary its image carries; one it
// cannot read holds no proxy's image, rather than a hash of nothing.
func TestFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv-internal-proxy")
	if err := os.WriteFile(path, []byte("abc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := fileSHA256(path), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"; got != want {
		t.Fatalf("fileSHA256 = %q, want %q", got, want)
	}
	if got := fileSHA256(filepath.Join(t.TempDir(), "missing")); got != "" {
		t.Fatalf("a missing binary hashed to %q", got)
	}
}

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

// w9/m492: an s3 build cache missing a coordinate refuses to start rather than
// running every build uncached, and any other mode needs no store at all.
func TestBuildCacheStoreRefusesAnIncompleteS3Store(t *testing.T) {
	full := map[string]string{
		"BEX_BUILD_CACHE_S3_ENDPOINT": "https://s3.example",
		"BEX_BUILD_CACHE_S3_BUCKET":   "bex-build-cache",
		"BEX_BUILD_CACHE_S3_REGION":   "eu-central-2",
		"BEX_BUILD_CACHE_S3_SECRET":   "bex-build-cache-s3",
	}
	set := func(t *testing.T, without string) {
		for k, v := range full {
			if k == without {
				v = ""
			}
			t.Setenv(k, v)
		}
	}
	set(t, "")
	store, err := buildCacheStore("s3", "bex-build")
	if err != nil || store.Bucket != "bex-build-cache" || store.Region != "eu-central-2" || !store.Configured() {
		t.Fatalf("complete s3 store = %+v, %v", store, err)
	}
	// Region is optional; the rest, and a separate build namespace, are not.
	required := []string{"BEX_BUILD_CACHE_S3_ENDPOINT", "BEX_BUILD_CACHE_S3_BUCKET", "BEX_BUILD_CACHE_S3_SECRET"}
	for _, missing := range required {
		set(t, missing)
		if _, err := buildCacheStore("s3", "bex-build"); err == nil || !strings.Contains(err.Error(), missing) {
			t.Errorf("s3 store without %s: err = %v", missing, err)
		}
	}
	set(t, "BEX_BUILD_CACHE_S3_REGION")
	if _, err := buildCacheStore("s3", "bex-build"); err != nil {
		t.Errorf("an s3 store without a region was refused: %v", err)
	}
	if _, err := buildCacheStore("s3", ""); err == nil || !strings.Contains(err.Error(), "BEX_BUILD_NAMESPACE") {
		t.Errorf("an s3 store with co-located builds: err = %v", err)
	}
	for _, mode := range []string{"", "registry", "off"} {
		if store, err := buildCacheStore(mode, ""); err != nil || store.Configured() {
			t.Errorf("mode %q = %+v, %v; want no store", mode, store, err)
		}
	}
}
