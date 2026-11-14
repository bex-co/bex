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
	"math/rand/v2"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w21/m394 t011 (R25-01): the host index let a host an App no longer claimed
// (still in its Status.URLs, written at its last Running pass) compete with the
// App that claims it in its spec, and chose by the informer cache's map order,
// so a later claimant's requests could be woken, held and proxied to the former
// holder's app.

func indexApp(name string, age time.Duration, spec appv1alpha1.AppSpec, status appv1alpha1.AppStatus) appv1alpha1.App {
	return appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "bex-system",
			CreationTimestamp: metav1.NewTime(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).Add(-age)),
		},
		Spec:   spec,
		Status: status,
	}
}

func appName(app *appv1alpha1.App) string {
	if app == nil {
		return "no App"
	}
	return app.Name
}

// shuffled returns items in a random order, as an informer cache's map does.
func shuffled(items []appv1alpha1.App) []appv1alpha1.App {
	out := append([]appv1alpha1.App(nil), items...)
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func TestHostIndexSpecClaimBeatsStaleStatus(t *testing.T) {
	// "former" dropped shared.example.com while parked: only its status still
	// names it. "claimant" has it in its spec.
	former := indexApp("former", 48*time.Hour,
		appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Hosts: []string{"other.example.com"}},
		appv1alpha1.AppStatus{URL: "https://shared.example.com", URLs: []string{"https://shared.example.com"}})
	claimant := indexApp("claimant", time.Hour,
		appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Hosts: []string{"shared.example.com"}},
		appv1alpha1.AppStatus{})
	items := []appv1alpha1.App{former, claimant}

	for range 200 {
		index := hostIndex(shuffled(items), "onbex.co")
		if got := index["shared.example.com"]; got == nil || got.Name != "claimant" {
			t.Fatalf("shared.example.com resolves to %s, want the App whose spec claims it", appName(got))
		}
		if got := index["other.example.com"]; got == nil || got.Name != "former" {
			t.Fatalf("other.example.com resolves to %s, want former", appName(got))
		}
	}
}

// Status still routes a host nobody's spec claims (the platform host of an App
// whose first status write is pending, a host just dropped from a Running App):
// the preference is only for spec claims.
func TestHostIndexKeepsStatusOnlyHost(t *testing.T) {
	app := indexApp("only", time.Hour,
		appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService},
		appv1alpha1.AppStatus{URL: "https://status-only.example.com"})
	if got := hostIndex([]appv1alpha1.App{app}, "onbex.co")["status-only.example.com"]; got == nil || got.Name != "only" {
		t.Fatalf("status-only host resolves to %s, want only", appName(got))
	}
}

// Two specs claiming one host resolve the same way on every rebuild: the older
// App, then the lower namespace/name.
func TestHostIndexDuplicateSpecClaimIsDeterministic(t *testing.T) {
	spec := appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Hosts: []string{"dup.example.com"}}
	older := indexApp("zzz-older", 72*time.Hour, spec, appv1alpha1.AppStatus{})
	newerA := indexApp("aaa-newer", time.Hour, spec, appv1alpha1.AppStatus{})
	newerB := indexApp("bbb-newer", time.Hour, spec, appv1alpha1.AppStatus{})
	for range 200 {
		index := hostIndex(shuffled([]appv1alpha1.App{newerB, older, newerA}), "onbex.co")
		if got := index["dup.example.com"]; got == nil || got.Name != "zzz-older" {
			t.Fatalf("dup.example.com resolves to %s, want the oldest claimant", appName(got))
		}
	}
	index := hostIndex(shuffled([]appv1alpha1.App{newerB, newerA}), "onbex.co")
	if got := index["dup.example.com"]; got == nil || got.Name != "aaa-newer" {
		t.Fatalf("equal-age claimants resolve to %s, want the lowest name", appName(got))
	}
}

func TestHostIndexSkipsDeletedAppsAndNamesPlatformHost(t *testing.T) {
	gone := indexApp("gone", time.Hour,
		appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Hosts: []string{"gone.example.com"}}, appv1alpha1.AppStatus{})
	now := metav1.Now()
	gone.DeletionTimestamp = &now
	live := indexApp("live", time.Hour,
		appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Expose: true, Subdomain: "live"}, appv1alpha1.AppStatus{})
	index := hostIndex([]appv1alpha1.App{gone, live}, "onbex.co")
	if _, ok := index["gone.example.com"]; ok {
		t.Fatal("a deleted App's host is still indexed")
	}
	if got := index["live.onbex.co"]; got == nil || got.Name != "live" {
		t.Fatalf("live.onbex.co resolves to %s, want live (named from its spec before any status)", appName(got))
	}
}
