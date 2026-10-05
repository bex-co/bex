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

package apps

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/jackc/pgx/v5/pgxpool"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Preserve the real env-group store's workspace/legacy locator separation.
type initialGroupKV struct{ *memKV }

func (k initialGroupKV) Get(ctx context.Context, path string) (map[string]string, error) {
	return k.memKV.Get(ctx, secrets.TenantFromContext(ctx)+"/"+path)
}
func (k initialGroupKV) Put(ctx context.Context, path string, data map[string]string) error {
	return k.memKV.Put(ctx, secrets.TenantFromContext(ctx)+"/"+path, data)
}
func (k initialGroupKV) Delete(ctx context.Context, path string) error {
	return k.memKV.Delete(ctx, secrets.TenantFromContext(ctx)+"/"+path)
}
func (k initialGroupKV) List(ctx context.Context, path string) ([]string, error) {
	return k.memKV.List(ctx, secrets.TenantFromContext(ctx)+"/"+path)
}
func (k initialGroupKV) GetVersioned(ctx context.Context, path string) (core.SecretKVSnapshot, error) {
	return k.memKV.GetVersioned(ctx, secrets.TenantFromContext(ctx)+"/"+path)
}
func (k initialGroupKV) PutCAS(ctx context.Context, path string, data map[string]string, version uint64) (uint64, error) {
	return k.memKV.PutCAS(ctx, secrets.TenantFromContext(ctx)+"/"+path, data, version)
}

// This controlled API server increments generation only for spec changes and
// observes every published spec before the next Blueprint stage. It deliberately
// models the adverse interleaving from production: the operator adopts the first
// release before initial group attachment. The actual Postgres deploy state and
// projector run below; no backend test imports the operator module.
type initialReleaseClient struct {
	client.Client
	observed         map[string][]*appv1alpha1.App
	afterObservation func(*appv1alpha1.App)
	beforeDelete     func(context.Context, *appv1alpha1.App) error
}

func (c *initialReleaseClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	a, ok := obj.(*appv1alpha1.App)
	if ok {
		a.Generation = 1
	}
	if err := c.Client.Create(ctx, obj, opts...); err != nil {
		return err
	}
	if ok {
		return c.observe(ctx, a)
	}
	return nil
}

func (c *initialReleaseClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if a, ok := obj.(*appv1alpha1.App); ok && c.beforeDelete != nil {
		callback := c.beforeDelete
		c.beforeDelete = nil
		if err := callback(ctx, a); err != nil {
			return err
		}
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func (c *initialReleaseClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	a, ok := obj.(*appv1alpha1.App)
	if !ok {
		return c.Client.Patch(ctx, obj, patch, opts...)
	}
	before := &appv1alpha1.App{}
	if err := c.Client.Get(ctx, client.ObjectKeyFromObject(a), before); err != nil {
		return err
	}
	if err := c.Client.Patch(ctx, obj, patch, opts...); err != nil {
		return err
	}
	if reflect.DeepEqual(before.Spec, a.Spec) {
		return nil
	}
	a.Generation = before.Generation + 1
	if err := c.Client.Update(ctx, a); err != nil {
		return err
	}
	return c.observe(ctx, a)
}

func (c *initialReleaseClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	a, ok := obj.(*appv1alpha1.App)
	if !ok {
		return c.Client.Update(ctx, obj, opts...)
	}
	before := &appv1alpha1.App{}
	if err := c.Client.Get(ctx, client.ObjectKeyFromObject(a), before); err != nil {
		return err
	}
	changed := !reflect.DeepEqual(before.Spec, a.Spec)
	if changed {
		a.Generation = before.Generation + 1
	}
	if err := c.Client.Update(ctx, obj, opts...); err != nil {
		return err
	}
	if changed {
		return c.observe(ctx, a)
	}
	return nil
}

func (c *initialReleaseClient) observe(ctx context.Context, a *appv1alpha1.App) error {
	c.observed[a.Labels[core.LabelServiceName]] = append(c.observed[a.Labels[core.LabelServiceName]], a.DeepCopy())
	a.Status.Phase = appv1alpha1.PhaseBuilding
	a.Status.ReleaseGeneration = a.Generation
	a.Status.ObservedGeneration = a.Generation
	a.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "BuildRunning", ObservedGeneration: a.Generation}}
	if err := c.Client.Status().Update(ctx, a); err != nil {
		return err
	}
	if c.afterObservation != nil {
		c.afterObservation(a)
	}
	return nil
}

// Run the projector in the interval after SQL identity allocation but before
// App publication, when it otherwise reconstructs a partial spec from its row.
type initialGroupIntentStore struct {
	*store.PGStore
	afterPending  func(context.Context, store.App) error
	afterComplete func(context.Context, string) error
}

func (s initialGroupIntentStore) CreateApp(ctx context.Context, desired store.App) (store.App, error) {
	row, err := s.PGStore.CreateApp(ctx, desired)
	if err == nil && desired.CreationPending && s.afterPending != nil {
		err = s.afterPending(ctx, row)
	}
	return row, err
}

func (s *initialGroupIntentStore) CompleteAppCreation(ctx context.Context, appID string) error {
	if err := s.PGStore.CompleteAppCreation(ctx, appID); err != nil {
		return err
	}
	if s.afterComplete != nil {
		return s.afterComplete(ctx, appID)
	}
	return nil
}

func TestPGBlueprintInitialGroupDeployLifecycle(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "initial-group-owner", Method: "session"})
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	workspace, err := st.CreateWorkspace(ctx, "initial-group-"+id.New(id.Owner), store.PlanHobby, "initial-group-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, workspace.ID) })
	scheme := fakeClient().Scheme()
	cl := &initialReleaseClient{Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appv1alpha1.App{}).Build(), observed: map[string][]*appv1alpha1.App{}}
	base := &core.Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"initial-group-owner": workspace.ID}}
	groups := &envgroups.Service{Base: base, Store: initialGroupKV{newMemKV()}}
	projector := &store.Reconciler{Store: st, Client: cl, DeployGateTimeout: time.Hour, BuildGateTimeout: time.Hour, PreDeployGateTimeout: time.Hour}
	pendingObservations := 0
	intents := &initialGroupIntentStore{PGStore: st, afterPending: func(ctx context.Context, row store.App) error {
		pendingObservations++
		if err := projector.ReconcileOnce(ctx); err != nil {
			return err
		}
		if len(cl.observed[row.Name]) != 0 {
			return fmt.Errorf("projector published incomplete pending App %s", row.Name)
		}
		return nil
	}}
	svc := &Service{Base: base, Store: intents, EnvGroups: groups}

	for _, scenario := range []string{"new-group", "existing-group", "literal"} {
		t.Run(scenario, func(t *testing.T) {
			declaration := "      - fromGroup: shared\n"
			prefix := ""
			if scenario == "new-group" {
				prefix = "envVarGroups:\n  - name: shared\n    envVars: [{key: MESSAGE, value: group-value}]\n"
			}
			if scenario == "literal" {
				declaration = "      - {key: MESSAGE, value: literal-value}\n"
			}
			manifest := prefix + fmt.Sprintf("services:\n  - name: %s\n    type: web\n    runtime: go\n    repo: https://github.com/bex-co/bex\n    buildCommand: go build -o app .\n    startCommand: ./app\n    autoDeployTrigger: off\n    envVars:\n%s", scenario, declaration)
			result, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Services) != 1 {
				t.Fatalf("services=%+v", result.Services)
			}
			returned := result.Services[0]
			a := &appv1alpha1.App{}
			if err := cl.Get(ctx, client.ObjectKey{Namespace: workspace.ID, Name: core.CRName(workspace.ID, scenario)}, a); err != nil {
				t.Fatal(err)
			}
			// The served revision is the last observed complete spec. This is an explicit
			// status input to the real projector, not a fake deploy-store verdict.
			a.Status.Phase = appv1alpha1.PhaseRunning
			a.Status.ActiveRevision = fmt.Sprintf("rev-%d", a.Status.ReleaseGeneration)
			a.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Running", ObservedGeneration: a.Generation}}
			if err := cl.Status().Update(ctx, a); err != nil {
				t.Fatal(err)
			}
			if err := projector.ReconcileOnce(ctx); err != nil {
				t.Fatal(err)
			}
			deploys, err := st.ListDeploys(ctx, returned.ID, store.DeployFilter{})
			if err != nil {
				t.Fatal(err)
			}
			if len(deploys) != 1 {
				t.Fatalf("deploys=%+v", deploys)
			}
			first := deploys[0]
			if first.ID != returned.LatestDeployID || first.Generation != store.FirstDeployGeneration || first.Status != store.DeployLive || first.FinishedAt == nil {
				t.Errorf("returned initial deploy=%s; stored id=%s generation=%d status=%s cancel=%q; observed versions=%d", returned.LatestDeployID, first.ID, first.Generation, first.Status, first.CancelReason, len(cl.observed[scenario]))
			}
			snapshots := cl.observed[scenario]
			if len(snapshots) != 1 {
				t.Errorf("published %d initial release specs, want one complete intent", len(snapshots))
			}
			if scenario == "literal" {
				if len(snapshots[0].Spec.Env) != 1 || snapshots[0].Spec.Env[0].Value != "literal-value" {
					t.Fatal("literal missing from initial release")
				}
			} else {
				if len(snapshots[0].Spec.EnvFromSecrets) == 0 {
					t.Error("initial release was observable before group attachment")
				}
				if len(a.Spec.EnvFromSecrets) == 0 {
					t.Fatal("final group reference missing")
				}
				secret := &corev1.Secret{}
				if err := cl.Get(ctx, client.ObjectKey{Namespace: workspace.ID, Name: a.Spec.EnvFromSecrets[0]}, secret); err != nil {
					t.Fatal(err)
				}
				if string(secret.Data["MESSAGE"]) != "group-value" {
					t.Errorf("projected group MESSAGE=%q", secret.Data["MESSAGE"])
				}
			}
		})
	}
	for _, cause := range []string{"superseded", "user-canceled"} {
		t.Run(cause, func(t *testing.T) {
			row, err := st.CreateApp(ctx, store.App{TenantID: workspace.ID, Name: cause, Type: appv1alpha1.TypeWebService, Image: "nginx", Port: 80, Replicas: 1, Tier: "free"})
			if err != nil {
				t.Fatal(err)
			}
			if err := projector.ReconcileOnce(ctx); err != nil {
				t.Fatal(err)
			}
			a := &appv1alpha1.App{}
			if err := cl.Get(ctx, client.ObjectKey{Namespace: workspace.ID, Name: core.CRName(workspace.ID, cause)}, a); err != nil {
				t.Fatal(err)
			}
			if cause == "user-canceled" {
				if changed, err := st.CloseDeploy(ctx, row.FirstDeployID, store.DeployCanceled, ""); err != nil || !changed {
					t.Fatalf("cancel changed=%v err=%v", changed, err)
				}
			} else {
				before := a.DeepCopy()
				a.Spec.StartCommand = "echo replacement"
				if err := cl.Patch(ctx, a, client.MergeFrom(before)); err != nil {
					t.Fatal(err)
				}
			}
			a.Status.Phase = appv1alpha1.PhaseRunning
			a.Status.ActiveRevision = fmt.Sprintf("rev-%d", a.Status.ReleaseGeneration)
			if err := cl.Status().Update(ctx, a); err != nil {
				t.Fatal(err)
			}
			if err := projector.ReconcileOnce(ctx); err != nil {
				t.Fatal(err)
			}
			deploy, err := st.GetDeploy(ctx, row.ID, row.FirstDeployID)
			if err != nil {
				t.Fatal(err)
			}
			if deploy.Status != store.DeployCanceled {
				t.Errorf("%s deploy became %s on healthy observation", cause, deploy.Status)
			}
		})
	}
	for _, name := range []string{"canceled-create", "ambiguous-completion"} {
		t.Run(name, func(t *testing.T) {
			canceledCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			cl.afterObservation = func(a *appv1alpha1.App) {
				if name == "canceled-create" && a.Labels[core.LabelServiceName] == name {
					cancel()
				}
			}
			defer func() { cl.afterObservation = nil; cl.beforeDelete = nil; intents.afterComplete = nil }()
			if name == "ambiguous-completion" {
				intents.afterComplete = func(context.Context, string) error { return fmt.Errorf("committed creation but acknowledgment lost") }
				cl.beforeDelete = func(deleteCtx context.Context, a *appv1alpha1.App) error {
					rows, err := st.ListDesiredApps(deleteCtx)
					if err != nil {
						return err
					}
					for _, row := range rows {
						if row.ID == a.Labels[core.LabelAppID] {
							t.Error("App deletion preceded durable row rollback")
						}
					}
					return projector.ReconcileOnce(deleteCtx)
				}
			}
			manifest := fmt.Sprintf("services:\n  - name: %s\n    type: web\n    runtime: go\n    repo: https://github.com/bex-co/bex\n    buildCommand: go build -o app .\n    startCommand: ./app\n    autoDeployTrigger: off\n    envVars: [{fromGroup: shared}]\n", name)
			if _, err := svc.DeployStack(canceledCtx, DeployRequest{Manifest: manifest}); err == nil {
				t.Fatal("canceled creation succeeded")
			}
			if len(cl.observed[name]) != 1 {
				t.Fatal("cancellation did not occur after App publication")
			}
			rows, err := st.ListDesiredApps(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row.Name == name {
					t.Fatal("canceled creation retained SQL row")
				}
			}
			apps := &appv1alpha1.AppList{}
			if err := cl.List(ctx, apps); err != nil {
				t.Fatal(err)
			}
			for _, app := range apps.Items {
				if app.Labels[core.LabelServiceName] == name {
					t.Fatal("canceled creation retained App")
				}
			}
			allGroups, err := groups.ListEnvGroups(ctx, "")
			if err != nil {
				t.Fatal(err)
			}
			for _, group := range allGroups {
				for _, link := range group.ServiceLinks {
					if link == name {
						t.Fatal("canceled creation retained membership")
					}
				}
			}
		})
	}
	if pendingObservations != 4 {
		t.Errorf("observed %d pending creation barriers, want 4", pendingObservations)
	}
}
