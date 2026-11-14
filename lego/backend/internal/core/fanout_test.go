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

package core

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// countingClient counts reads and patches and the most patches in flight,
// and can run a hook before a named object's first patch.
type countingClient struct {
	client.Client
	gets, patches    atomic.Int32
	inFlight, peak   atomic.Int32
	mu               sync.Mutex
	beforeFirstPatch map[string]func()
}

func (c *countingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.gets.Add(1)
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *countingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patches.Add(1)
	n := c.inFlight.Add(1)
	defer c.inFlight.Add(-1)
	for {
		peak := c.peak.Load()
		if n <= peak || c.peak.CompareAndSwap(peak, n) {
			break
		}
	}
	c.mu.Lock()
	hook := c.beforeFirstPatch[obj.GetName()]
	delete(c.beforeFirstPatch, obj.GetName())
	c.mu.Unlock()
	if hook != nil {
		hook()
	}
	time.Sleep(time.Millisecond)
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func fanoutApps(n int) []client.Object {
	objs := make([]client.Object, n)
	for i := range n {
		objs[i] = &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("app-%02d", i), Namespace: "tea-a"}}
	}
	return objs
}

// w9/m388: PatchEach patches from the listed objects (no read on the happy
// path), skips those mutate leaves alone, re-reads and re-applies only after a
// conflict — keeping the concurrent write — and runs at most fanOutLimit
// patches at a time.
func TestPatchEach(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fanoutApps(20)...).Build()
	cl := &countingClient{Client: base, beforeFirstPatch: map[string]func(){
		"app-03": func() {
			var a appv1alpha1.App
			_ = base.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: "app-03"}, &a)
			a.Annotations = map[string]string{"unrelated": "write"}
			if err := base.Update(context.Background(), &a); err != nil {
				t.Error(err)
			}
		},
	}}
	var list appv1alpha1.AppList
	if err := base.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	// app-00 already carries the layer: no write.
	list.Items[0].Spec.WorkspaceIPAllowList = []string{"203.0.113.0/24"}
	if err := base.Update(context.Background(), &list.Items[0]); err != nil {
		t.Fatal(err)
	}
	layer := []string{"203.0.113.0/24"}
	err := PatchEach(context.Background(), cl, Pointers(list.Items), func(a *appv1alpha1.App) bool {
		if slices.Equal(a.Spec.WorkspaceIPAllowList, layer) {
			return false
		}
		a.Spec.WorkspaceIPAllowList = slices.Clone(layer)
		return true
	})
	if err != nil {
		t.Fatalf("PatchEach: %v", err)
	}
	if got := cl.patches.Load(); got != 20 { // 19 changed + 1 retried conflict
		t.Errorf("patches = %d, want 20", got)
	}
	if got := cl.gets.Load(); got != 1 {
		t.Errorf("reads = %d, want only the conflicted object's re-read", got)
	}
	if peak := cl.peak.Load(); peak > fanOutLimit || peak < 2 {
		t.Errorf("peak concurrent patches = %d, want 2..%d", peak, fanOutLimit)
	}
	var after appv1alpha1.AppList
	if err := base.List(context.Background(), &after); err != nil {
		t.Fatal(err)
	}
	for _, a := range after.Items {
		if !slices.Equal(a.Spec.WorkspaceIPAllowList, layer) {
			t.Errorf("%s layer = %v", a.Name, a.Spec.WorkspaceIPAllowList)
		}
		if a.Name == "app-03" && a.Annotations["unrelated"] != "write" {
			t.Error("the retried patch lost the concurrent write")
		}
	}
}

// An object deleted between the list and its conflicted retry is skipped, not
// an error; a non-conflict failure is returned.
func TestPatchEachSkipsAnObjectGoneOnRetry(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fanoutApps(1)...).Build()
	var list appv1alpha1.AppList
	if err := base.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	cl := &countingClient{Client: base, beforeFirstPatch: map[string]func(){
		"app-00": func() {
			if err := base.Delete(context.Background(), &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "app-00", Namespace: "tea-a"}}); err != nil {
				t.Error(err)
			}
		},
	}}
	mutate := func(a *appv1alpha1.App) bool { a.Spec.WorkspaceIPAllowList = []string{"10.0.0.0/8"}; return true }
	if err := PatchEach(context.Background(), cl, Pointers(list.Items), mutate); err != nil {
		t.Fatalf("a deleted object failed the fan-out: %v", err)
	}
}
