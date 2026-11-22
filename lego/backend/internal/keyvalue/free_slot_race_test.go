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

package keyvalue

import (
	"context"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A plan change onto free re-checks under the tenant lock with its write, so
// two moves that both pass the pre-check admit exactly one — the Key Value
// sibling of the Postgres race the tenant lock closes. Both stores below run
// a tier the catalog no longer names: a paid-to-free move never reaches the
// race (the downgrade gate refuses it first), while a move from an
// uncomparable plan reaches the one-free check.

// kvLockingWorkspace is fakeWorkspace with the tenant lock production's
// resolver offers (core.TenantAdvisoryLocker): an in-process mutex. Each
// caller is counted on the barrier as it reaches the lock, before it waits
// there. The lock is not re-entrant, so production would deadlock where the
// test helper errors.
type kvLockingWorkspace struct {
	fakeWorkspace
	lock func(ctx context.Context, tenantID string, fn func() error) error
	// barrier counts the callers that reached the lock, before they wait
	// there; held reports a caller running under it.
	barrier *kvArrivals
	held    bool
	mu      sync.Mutex
}

func (w *kvLockingWorkspace) WithTenantAdvisoryLock(_ context.Context, _ string, fn func() error) error {
	if w.barrier != nil {
		w.barrier.arrive()
	}
	w.mu.Lock()
	w.held = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.held = false
		w.mu.Unlock()
	}()
	return w.lock(context.Background(), "", fn)
}

// kvArrivals releases its waiters once want callers have arrived.
type kvArrivals struct {
	mu   sync.Mutex
	want int
	all  chan struct{}
}

func (a *kvArrivals) arrive() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.want--; a.want == 0 {
		close(a.all)
	}
}

func (a *kvArrivals) wait() {
	select {
	case <-a.all:
	case <-time.After(10 * time.Second): // a caller that never arrives must not hang the suite
	}
}

func kvMutexLock() func(context.Context, string, func() error) error {
	var mu sync.Mutex
	return func(_ context.Context, _ string, fn func() error) error {
		mu.Lock()
		defer mu.Unlock()
		return fn()
	}
}

// raceKVPlanChangesOntoFree moves two retired-tier stores onto the free plan
// at once, lined up the way their writes can meet: neither patch is written
// until both requests have passed their pre-check. It returns the two
// requests' errors and how many stores end up free.
func raceKVPlanChangesOntoFree(t *testing.T, tenantID string, change func(svc *Service, ctx context.Context, name string) error) ([]error, int) {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	retired := func(name string) *appv1alpha1.KeyValue {
		return &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: core.TenantLabels(tenantID)},
			Spec:       appv1alpha1.KeyValueSpec{Name: name, Plan: "retired-tier"},
		}
	}
	barrier := &kvArrivals{want: 2, all: make(chan struct{})}
	ws := &kvLockingWorkspace{fakeWorkspace: fakeWorkspace{"user-a": tenantID}, lock: kvMutexLock(), barrier: barrier}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(retired("red-one"), retired("red-two")).WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
			ws.mu.Lock()
			held := ws.held
			ws.mu.Unlock()
			if !held {
				barrier.arrive()
			}
			barrier.wait()
			return c.Patch(ctx, obj, p, opts...)
		},
	}).Build()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Workspace: ws}}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, name := range []string{"red-one", "red-two"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = change(svc, ctxAs("user-a"), name)
		}()
	}
	wg.Wait()
	var list appv1alpha1.KeyValueList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	free := 0
	for _, kv := range list.Items {
		if kv.Spec.Plan == core.FreePlanID {
			free++
		}
	}
	return errs, free
}

// kvPlanChangeVerbs are the two plan changes the surfaces share: SetPlan
// (GraphQL updateKeyValuePlan) and UpdateKeyValue (REST PATCH, MCP
// update_key_value).
var kvPlanChangeVerbs = map[string]func(svc *Service, ctx context.Context, name string) error{
	"SetPlan": func(svc *Service, ctx context.Context, name string) error {
		_, err := svc.SetPlan(ctx, name, core.FreePlanID)
		return err
	},
	"UpdateKeyValue": func(svc *Service, ctx context.Context, name string) error {
		free := core.FreePlanID
		_, err := svc.UpdateKeyValue(ctx, name, KeyValuePatch{Plan: &free})
		return err
	},
}

// Two plan changes onto free that both pass their pre-check before either
// writes admit one: each re-checks under the tenant lock with its patch.
func TestConcurrentKVPlanChangesOntoFreeAdmitOne(t *testing.T) {
	for verb, change := range kvPlanChangeVerbs {
		t.Run(verb, func(t *testing.T) {
			errs, free := raceKVPlanChangesOntoFree(t, "tea-a", change)
			admitted, refused := 0, 0
			for _, err := range errs {
				switch {
				case err == nil:
					admitted++
				case errorCode(err) == core.FreeInstanceLimitCode:
					refused++
				}
			}
			if free != 1 || admitted != 1 || refused != 1 {
				t.Fatalf("two concurrent moves onto free left %d free, errs %v; want one admitted and one refused with %s", free, errs, core.FreeInstanceLimitCode)
			}
		})
	}
}
