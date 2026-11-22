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

package postgres

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// lockingWorkspace is fakeWorkspace with the tenant lock production's resolver
// offers (core.TenantAdvisoryLocker), run by lock: an in-process mutex, or the
// control-plane store's advisory lock. Each caller is counted on the barrier
// as it reaches the lock, before it waits there.
type lockingWorkspace struct {
	fakeWorkspace
	lock    func(ctx context.Context, tenantID string, fn func() error) error
	barrier *arrivals
	// held reports a caller running under the lock.
	held atomic.Bool
}

func (w *lockingWorkspace) WithTenantAdvisoryLock(ctx context.Context, tenantID string, fn func() error) error {
	if w.barrier != nil {
		w.barrier.arrive()
	}
	return w.lock(ctx, tenantID, func() error {
		w.held.Store(true)
		defer w.held.Store(false)
		return fn()
	})
}

// mutexLock is an in-process tenant lock: one holder at a time.
func mutexLock() func(context.Context, string, func() error) error {
	var mu sync.Mutex
	return func(_ context.Context, _ string, fn func() error) error {
		mu.Lock()
		defer mu.Unlock()
		return fn()
	}
}

// arrivals releases its waiters once want callers have arrived.
type arrivals struct {
	mu   sync.Mutex
	want int
	all  chan struct{}
}

func newArrivals(want int) *arrivals { return &arrivals{want: want, all: make(chan struct{})} }

func (a *arrivals) arrive() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.want--; a.want == 0 {
		close(a.all)
	}
}

func (a *arrivals) wait() {
	select {
	case <-a.all:
	case <-time.After(10 * time.Second): // a caller that never arrives must not hang the suite
	}
}

// racePlanChangesOntoFree moves two paid databases onto the free plan at once,
// lined up the way their writes can meet: neither patch is written until both
// requests have passed their pre-check, which each has once it reaches the
// tenant lock or, writing without it, its patch. It returns the two requests'
// errors and how many databases end up free.
func racePlanChangesOntoFree(t *testing.T, lock func(context.Context, string, func() error) error, tenantID string, change func(svc *Service, ctx context.Context, name string) error) ([]error, int) {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	paid := func(name string) *appv1alpha1.Database {
		return &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: core.TenantLabels(tenantID)},
			Spec:       appv1alpha1.DatabaseSpec{Name: name, Plan: "basic-256mb"},
		}
	}
	barrier := newArrivals(2)
	ws := &lockingWorkspace{fakeWorkspace: fakeWorkspace{"user-a": tenantID}, lock: lock, barrier: barrier}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(paid("dpg-one"), paid("dpg-two")).WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
			if !ws.held.Load() {
				barrier.arrive()
			}
			barrier.wait()
			return c.Patch(ctx, obj, p, opts...)
		},
	}).Build()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Workspace: ws}}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, name := range []string{"dpg-one", "dpg-two"} {
		wg.Go(func() { errs[i] = change(svc, ctxAs("user-a"), name) })
	}
	wg.Wait()
	var list appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	free := 0
	for _, d := range list.Items {
		if d.Spec.Plan == core.FreePlanID {
			free++
		}
	}
	return errs, free
}

// planChangeVerbs are the two plan changes the surfaces share: SetPlan
// (GraphQL updateDatabasePlan) and UpdatePostgres (REST PATCH, MCP
// update_postgres).
var planChangeVerbs = map[string]func(svc *Service, ctx context.Context, name string) error{
	"SetPlan": func(svc *Service, ctx context.Context, name string) error {
		_, err := svc.SetPlan(ctx, name, core.FreePlanID)
		return err
	},
	"UpdatePostgres": func(svc *Service, ctx context.Context, name string) error {
		free := core.FreePlanID
		_, err := svc.UpdatePostgres(ctx, name, PostgresPatch{Plan: &free})
		return err
	},
}

// assertOneFreeAdmitted checks that exactly one of two concurrent moves onto
// free was admitted and the other refused with FREE_INSTANCE_LIMIT.
func assertOneFreeAdmitted(t *testing.T, errs []error, free int) {
	t.Helper()
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
}

// Two plan changes onto free that both pass their pre-check before either
// writes admit one: each re-checks under the tenant lock with its patch.
func TestConcurrentPlanChangesOntoFreeAdmitOne(t *testing.T) {
	for verb, change := range planChangeVerbs {
		t.Run(verb, func(t *testing.T) {
			errs, free := racePlanChangesOntoFree(t, mutexLock(), "tea-a", change)
			assertOneFreeAdmitted(t, errs, free)
		})
	}
}

// The same race under the control-plane store's own advisory lock, the lock
// production's workspace resolver holds.
func TestConcurrentPlanChangesOntoFreeAdmitOnePG(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), testDBURI(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	pg := store.NewPGStore(pool)
	for verb, change := range planChangeVerbs {
		t.Run(verb, func(t *testing.T) {
			tenantID := fmt.Sprintf("tea-free-slot-%d", time.Now().UnixNano())
			errs, free := racePlanChangesOntoFree(t, pg.WithTenantAdvisoryLock, tenantID, change)
			assertOneFreeAdmitted(t, errs, free)
		})
	}
}

// Without the tenant lock (a resolver that offers none), the create's
// rollback is the guard. B passes its pre-check first; A then runs its whole
// create, and B's create lands within the same second, which is all a
// creationTimestamp records, under a name that sorts before A's. A listed
// before B landed and keeps the slot, so B, seeing the tie, yields.
func TestFreeCreatesWithinOneSecondLeaveOne(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	second := metav1.NewTime(time.Now().Truncate(time.Second))
	bPassed, aDone := make(chan struct{}), make(chan struct{})
	cl := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			obj.SetCreationTimestamp(second)
			if obj.GetName() == "dpg-a-later" {
				close(bPassed)
				<-aDone
			}
			return c.Create(ctx, obj, opts...)
		},
	}).Build()
	b := &core.Base{Client: cl, Namespace: "default"}
	free := func(name string) *appv1alpha1.Database {
		return &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.DatabaseSpec{Name: name, Plan: core.FreePlanID},
		}
	}
	errB := make(chan error, 1)
	go func() { errB <- CreateResource(context.Background(), b, "", free("dpg-a-later")) }()
	<-bPassed
	errA := CreateResource(context.Background(), b, "", free("dpg-z-earlier"))
	close(aDone)
	if err := <-errB; errorCode(err) != core.FreeInstanceLimitCode {
		t.Fatalf("the later create = %v, want %s", err, core.FreeInstanceLimitCode)
	}
	if errA != nil {
		t.Fatalf("the create that saw only itself = %v, want it kept", errA)
	}
	var list appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].Name != "dpg-z-earlier" {
		t.Fatalf("free databases left = %+v, want dpg-z-earlier alone", list.Items)
	}
}
