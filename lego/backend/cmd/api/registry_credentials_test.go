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
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestParseSecretRef(t *testing.T) {
	if key, err := parseSecretRef("bex-registry/bex-sandbox-registry"); err != nil || key.Namespace != "bex-registry" || key.Name != "bex-sandbox-registry" {
		t.Errorf("valid ref = %v, %v", key, err)
	}
	for _, bad := range []string{"", "no-slash", "/name", "ns/", "a/b/c"} {
		if _, err := parseSecretRef(bad); err == nil {
			t.Errorf("parseSecretRef(%q) must fail", bad)
		}
	}
}

func meterSecret(password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "bex-registry", Name: "bex-sandbox-registry"},
		Data: map[string][]byte{
			registrySecretReaderUserKey: []byte("bex-sandbox-meter"), registrySecretReaderPasswordKey: []byte(password),
			registrySecretReaperUserKey: []byte("bex-sandbox-reaper"), registrySecretReaperPasswordKey: []byte("reaper-pw"),
		},
	}
}

// The provider reads the right key pair, caches it for the TTL, picks up a
// rotated password after it, serves the last good value through an API outage,
// and refuses a Secret that has no password yet instead of sending an empty one.
func TestSecretCredentialsReadsCachesRotatesAndSurvivesAnOutage(t *testing.T) {
	ctx := context.Background()
	key := client.ObjectKey{Namespace: "bex-registry", Name: "bex-sandbox-registry"}
	reads, fail := 0, false
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(meterSecret("meter-pw")).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, k client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			reads++
			if fail {
				return errors.New("apiserver down")
			}
			return cl.Get(ctx, k, obj, opts...)
		},
	}).Build()

	clock := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	now := func() time.Time { return clock }
	reader := secretCredentials(c, key, registrySecretReaderUserKey, registrySecretReaderPasswordKey, now)
	reaper := secretCredentials(c, key, registrySecretReaperUserKey, registrySecretReaperPasswordKey, now)

	if u, p, err := reader(ctx); err != nil || u != "bex-sandbox-meter" || p != "meter-pw" {
		t.Fatalf("reader = %q/%q err %v", u, p, err)
	}
	if u, p, err := reaper(ctx); err != nil || u != "bex-sandbox-reaper" || p != "reaper-pw" {
		t.Fatalf("reaper = %q/%q err %v: it must read its own key pair", u, p, err)
	}
	before := reads
	for range 5 {
		if _, _, err := reader(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if reads != before {
		t.Errorf("%d extra reads inside the TTL, want none", reads-before)
	}

	// Past the TTL a rotated password is picked up.
	if err := c.Update(ctx, meterSecret("rotated")); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(registrySecretTTL + time.Second)
	if _, p, _ := reader(ctx); p != "rotated" {
		t.Errorf("password after the TTL = %q, want the rotated one", p)
	}

	// An API outage serves the last good value.
	fail = true
	clock = clock.Add(registrySecretTTL + time.Second)
	if u, p, err := reader(ctx); err != nil || u != "bex-sandbox-meter" || p != "rotated" {
		t.Errorf("during an outage = %q/%q err %v, want the last good credential", u, p, err)
	}

	// A fresh provider with no good value and a Secret with no password refuses.
	fail = false
	empty := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "bex-registry", Name: "bex-sandbox-registry"}}
	c2 := fake.NewClientBuilder().WithScheme(scheme).WithObjects(empty).Build()
	if _, _, err := secretCredentials(c2, key, registrySecretReaderUserKey, registrySecretReaderPasswordKey, now)(ctx); err == nil {
		t.Error("a Secret with no password yet must be refused, not sent as an empty credential")
	}
	if _, _, err := secretCredentials(fake.NewClientBuilder().WithScheme(scheme).Build(), key, registrySecretReaderUserKey, registrySecretReaderPasswordKey, now)(ctx); err == nil {
		t.Error("a missing Secret with no good value must be an error")
	}
}
