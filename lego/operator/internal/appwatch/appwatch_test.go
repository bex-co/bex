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

package appwatch

import (
	"sync/atomic"
	"testing"
	"time"

	toolscache "k8s.io/client-go/tools/cache"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func app(t string) *appv1alpha1.App {
	return &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Type: t}}
}

// TestSignallerSkipsIrrelevantApps: the static-server rebuilds only for static
// sites, so a web service's status writes cost it nothing; an App that leaves
// the set (old state relevant) and a deleted tombstone still signal.
func TestSignallerSkipsIrrelevantApps(t *testing.T) {
	static := func(a *appv1alpha1.App) bool { return a.Spec.Type == appv1alpha1.TypeStaticSite }
	for _, tc := range []struct {
		name string
		fire func(toolscache.ResourceEventHandlerFuncs)
		want bool
	}{
		{"web add", func(h toolscache.ResourceEventHandlerFuncs) { h.OnAdd(app(appv1alpha1.TypeWebService), false) }, false},
		{"static add", func(h toolscache.ResourceEventHandlerFuncs) { h.OnAdd(app(appv1alpha1.TypeStaticSite), false) }, true},
		{"web update", func(h toolscache.ResourceEventHandlerFuncs) {
			h.OnUpdate(app(appv1alpha1.TypeWebService), app(appv1alpha1.TypeWebService))
		}, false},
		{"leaves static", func(h toolscache.ResourceEventHandlerFuncs) {
			h.OnUpdate(app(appv1alpha1.TypeStaticSite), app(appv1alpha1.TypeWebService))
		}, true},
		{"static tombstone", func(h toolscache.ResourceEventHandlerFuncs) {
			h.OnDelete(toolscache.DeletedFinalStateUnknown{Obj: app(appv1alpha1.TypeStaticSite)})
		}, true},
	} {
		changed := make(chan struct{}, 1)
		tc.fire(signaller(changed, static))
		if got := len(changed) == 1; got != tc.want {
			t.Errorf("%s: signalled %v, want %v", tc.name, got, tc.want)
		}
	}
	changed := make(chan struct{}, 1)
	signaller(changed, nil).OnAdd(app(appv1alpha1.TypeWebService), false)
	if len(changed) != 1 {
		t.Error("with no filter every App signals")
	}
}

// TestLoopRebuildsOnChangeAtABoundedRate: a signal rebuilds without waiting
// for the backstop, and a burst of signals rebuilds at most once per
// rebuildInterval.
func TestLoopRebuildsOnChangeAtABoundedRate(t *testing.T) {
	ctx := t.Context()
	changed := make(chan struct{}, 1)
	var rebuilds atomic.Int64
	go Loop(ctx, time.Hour, changed, func() { rebuilds.Add(1) })
	deadline := time.Now().Add(2 * time.Second)
	for rebuilds.Load() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("no first rebuild")
		}
		time.Sleep(time.Millisecond)
	}
	stop := time.Now().Add(4 * rebuildInterval)
	for time.Now().Before(stop) {
		select {
		case changed <- struct{}{}:
		default:
		}
		time.Sleep(time.Millisecond)
	}
	if got := rebuilds.Load(); got < 2 || got > 6 {
		t.Fatalf("rebuilds over %v of constant change = %d, want a few, bounded by %v", 4*rebuildInterval, got, rebuildInterval)
	}
}
