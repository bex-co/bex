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

package deploys

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Rollback selects historical runtime inputs while saved settings remain ready
// for the next standard deploy (w5/m107).

// rollbackFixture: D1 (generation 1) live and served, D2 (generation 2) the current
// release. The operator recorded D1's release as the snapshots and record below.
func rollbackFixture(t *testing.T, withRecord bool) (*Service, client.Client, store.Deploy) {
	t.Helper()
	ctx := context.Background()
	ds := newFakeStore()
	d1, _ := ds.CreateDeploy(ctx, "srv-1", "create", "web:v1", 1, store.CommitInfo{}, "")
	if won, err := ds.CloseDeploy(ctx, d1.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close d1: %v %v", won, err)
	}
	d2, _ := ds.CreateDeploy(ctx, "srv-1", "config_change", "web:v2", 2, store.CommitInfo{}, "")
	if won, err := ds.CloseDeploy(ctx, d2.ID, store.DeployLive, "web:v2"); err != nil || !won {
		t.Fatalf("close d2: %v %v", won, err)
	}

	app := sampleApp("web", "srv-1")
	app.Generation = 2
	app.Spec.Image = "web:v2"
	app.Spec.EnvFromSecret = "web-env"
	app.Spec.StartCommand = "./server --v2"
	app.Spec.AutoDeploy = true
	app.Spec.Tier = "standard" // plan must NOT roll back
	objs := []client.Object{app,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "web-env", Namespace: "default"}, Data: map[string][]byte{"KEEP": []byte("saved-new"), "ADDED": []byte("B")}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "web-files", Namespace: "default"}, Data: map[string][]byte{"config.txt": []byte("file-B")}},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ReleaseSnapshotName("web-env", 1), Namespace: "default"},
			Data:       map[string][]byte{"KEEP": []byte("old")},
		},
	}
	if withRecord {
		spec, _ := json.Marshal(appv1alpha1.ReleaseRecordSpec{StartCommand: "./server --v1"})
		objs = append(objs, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ReleaseRecordName("web", 1), Namespace: "default"},
			Data:       map[string][]byte{appv1alpha1.ReleaseRecordSpecKey: spec},
		})
	}
	svc, cl := newService(ds, objs...)
	return svc, cl, d1
}

func TestRollbackPreservesSavedSettingsAndSelectsTargetRuntime(t *testing.T) {
	svc, cl, d1 := rollbackFixture(t, true)
	if _, err := svc.Rollback(context.Background(), "web", d1.ID); err != nil {
		t.Fatal(err)
	}
	assertSavedRollbackSettings(t, cl)
	got := getApp(t, cl, "web")
	selected := got.Spec.ReleaseConfig
	if selected == nil || selected.Generation != 3 || selected.SourceGeneration != 1 || selected.Image != "web:v1" || selected.PreserveGroupValues {
		t.Fatalf("runtime selection = %+v, want rollback of generation1 in release3", selected)
	}
	if len(svc.Store.(*fakeStore).setImage) != 0 {
		t.Fatal("rollback changed the saved row image")
	}
	if !got.Spec.AutoDeploy {
		t.Fatal("API rollback disabled auto deploy")
	}
}

func assertSavedRollbackSettings(t *testing.T, cl client.Client) {
	t.Helper()
	got := getApp(t, cl, "web")
	if got.Spec.StartCommand != "./server --v2" || got.Spec.Image != "web:v2" || got.Spec.Tier != "standard" {
		t.Fatalf("saved configuration was changed: command %q image %q tier %q", got.Spec.StartCommand, got.Spec.Image, got.Spec.Tier)
	}
	for _, tc := range []struct{ name, key, value string }{{"web-env", "KEEP", "saved-new"}, {"web-env", "ADDED", "B"}, {"web-files", "config.txt", "file-B"}} {
		secret := &corev1.Secret{}
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: tc.name}, secret); err != nil {
			t.Fatal(err)
		}
		if string(secret.Data[tc.key]) != tc.value {
			t.Fatalf("saved %s/%s changed", tc.name, tc.key)
		}
	}
}

func TestRollbackRestartAndNextDeployKeepSeparateConfigurations(t *testing.T) {
	ctx := context.Background()
	svc, cl, d1 := rollbackFixture(t, true)
	rolled, err := svc.Rollback(ctx, "web", d1.ID)
	if err != nil {
		t.Fatal(err)
	}
	ds := svc.Store.(*fakeStore)
	if _, err := ds.CloseDeploy(ctx, rolled.ID, store.DeployLive, "web:v1"); err != nil {
		t.Fatal(err)
	}
	app := getApp(t, cl, "web")
	app.Generation = 3
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(appv1alpha1.ReleaseRecordSpec{StartCommand: "./server --v1"})
	if err := cl.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: appv1alpha1.ReleaseRecordName("web", 3)}, Data: map[string][]byte{appv1alpha1.ReleaseRecordSpecKey: raw}}); err != nil {
		t.Fatal(err)
	}
	restarted, err := svc.Restart(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	selected := getApp(t, cl, "web").Spec.ReleaseConfig
	if selected == nil || selected.SourceGeneration != 3 || selected.Generation != 4 || selected.Image != "web:v1" || !selected.PreserveGroupValues || restarted.Image != "web:v1" {
		t.Fatalf("restart did not keep rollback runtime: %+v, %+v", selected, restarted)
	}
	assertSavedRollbackSettings(t, cl)
	next, err := svc.Trigger(ctx, "web", TriggerParams{})
	if err != nil {
		t.Fatal(err)
	}
	if getApp(t, cl, "web").Spec.ReleaseConfig != nil || next.Image != "web:v2" {
		t.Fatalf("normal deploy did not return to saved B: %+v", next)
	}
	assertSavedRollbackSettings(t, cl)
}

func TestDashboardRollbackTurnsAutoDeployOff(t *testing.T) {
	svc, cl, d1 := rollbackFixture(t, true)
	if _, err := svc.Rollback(context.Background(), "web", d1.ID, RollbackOptions{DisableAutoDeploy: true}); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if getApp(t, cl, "web").Spec.AutoDeploy {
		t.Error("a dashboard rollback must turn auto-deploy off so the next push does not undo it")
	}
}

// No record: the target predates snapshots or was reclaimed. The rollback still
// restores the image — it must not fail — but it must not invent a configuration.
func TestRollbackWithoutARecordSelectsOnlyTheImage(t *testing.T) {
	svc, cl, d1 := rollbackFixture(t, false)
	if _, err := svc.Rollback(context.Background(), "web", d1.ID); err != nil {
		t.Fatal(err)
	}
	assertSavedRollbackSettings(t, cl)
	selected := getApp(t, cl, "web").Spec.ReleaseConfig
	if selected == nil || selected.SourceGeneration != 0 || selected.Image != "web:v1" {
		t.Fatalf("legacy selection = %+v", selected)
	}
}

func TestRollbackRejectsCorruptRecordWithoutChangingSavedState(t *testing.T) {
	ctx := context.Background()
	svc, cl, d1 := rollbackFixture(t, true)
	rec := &corev1.Secret{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: appv1alpha1.ReleaseRecordName("web", 1)}, rec); err != nil {
		t.Fatal(err)
	}
	rec.Data[appv1alpha1.ReleaseRecordSpecKey] = []byte("invalid json")
	if err := cl.Update(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rollback(ctx, "web", d1.ID); err == nil {
		t.Fatal("corrupt configuration record was accepted")
	}
	assertSavedRollbackSettings(t, cl)
	if got := getApp(t, cl, "web"); got.Spec.ReleaseConfig != nil || got.Spec.RestartedAt != "" {
		t.Fatal("failed rollback opened a release")
	}
	if len(svc.Store.(*fakeStore).byApp["srv-1"]) != 2 {
		t.Fatal("failed rollback opened a deploy row")
	}
}

// GraphQL is both the dashboard's surface and a public API. The argument defaults
// to Render's API behaviour; only a caller that asks — the dashboard — turns
// auto-deploy off.
func TestGraphQLRollbackDisablesAutoDeployOnlyWhenAsked(t *testing.T) {
	for _, tc := range []struct {
		name string
		arg  string
		want bool
	}{
		{"default leaves it on", "", true},
		{"dashboard turns it off", ", disableAutoDeploy: true", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl, d1 := rollbackFixture(t, true)
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
				Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
			})
			if err != nil {
				t.Fatal(err)
			}
			res := graphql.Do(graphql.Params{
				Schema:  schema,
				Context: context.Background(),
				RequestString: `mutation { rollbackService(serviceId: "web", deployId: "` + d1.ID + `"` +
					tc.arg + `) { id } }`,
			})
			if len(res.Errors) > 0 {
				t.Fatalf("errors: %v", res.Errors)
			}
			if got := getApp(t, cl, "web").Spec.AutoDeploy; got != tc.want {
				t.Errorf("autoDeploy = %v, want %v", got, tc.want)
			}
		})
	}
}

// A failed configuration-only release can share the previous live image.
// Recovery remains actionable, while repeating the selected live release does not.
func TestRollbackCapabilitiesFollowSelectedReleaseAndConfigGeneration(t *testing.T) {
	ctx := context.Background()
	svc, cl, d1 := rollbackFixture(t, true)
	app := getApp(t, cl, "web")
	app.Spec.Image = "web:v1"
	app.Annotations = map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"}
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	ds := svc.Store.(*fakeStore)
	rows := ds.byApp["srv-1"]
	for i := range rows {
		if rows[i].ID == d1.ID {
			rows[i].Status = store.DeployLive
		} else {
			rows[i].Status = store.DeployUpdateFailed
		}
	}
	ds.byApp["srv-1"] = rows
	live, _ := ds.GetDeploy(ctx, "srv-1", d1.ID)
	if !RollbackActionable(app, live) {
		t.Fatal("same-image configuration recovery was refused")
	}
	rolled, err := svc.Rollback(ctx, "web", d1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ds.CloseDeploy(ctx, rolled.ID, store.DeployLive, "web:v1"); err != nil {
		t.Fatal(err)
	}
	app = getApp(t, cl, "web")
	app.Spec.Image = "web:v2"
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	current, _ := ds.GetDeploy(ctx, "srv-1", rolled.ID)
	if RollbackActionable(app, current) {
		t.Fatal("already-selected live rollback was offered")
	}
	if _, err := svc.Rollback(ctx, "web", rolled.ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("no-op rollback = %v, want conflict", err)
	}
}
