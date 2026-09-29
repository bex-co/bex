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

// rollback_config_test.go pins w1/m152 t009. Live on 2026-09-14 a rollback to D1
// (no MESSAGE) kept serving D2's MESSAGE=v2 and left autoDeploy on; Render restores
// the target's env vars and start command, and turns auto-deploy off for a
// dashboard rollback only.

type recordingRestorer struct {
	calls []struct{ env, files map[string]string }
	err   error
}

func (r *recordingRestorer) RestoreEnvironment(_ context.Context, _ string, env, files map[string]string) (bool, error) {
	r.calls = append(r.calls, struct{ env, files map[string]string }{env, files})
	return true, r.err
}

// rollbackFixture: D1 (generation 1) live and served, D2 (generation 2) the current
// release. The operator recorded D1's release as the snapshots and record below.
func rollbackFixture(t *testing.T, withRecord bool) (*Service, client.Client, *recordingRestorer, store.Deploy) {
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
	app.Spec.Image = "web:v2"
	app.Spec.EnvFromSecret = "web-env"
	app.Spec.StartCommand = "./server --v2"
	app.Spec.AutoDeploy = true
	app.Spec.Tier = "standard" // plan must NOT roll back
	objs := []client.Object{app,
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
	restorer := &recordingRestorer{}
	svc.Environment = restorer
	return svc, cl, restorer, d1
}

func TestRollbackRestoresTheTargetsEnvironmentAndStartCommand(t *testing.T) {
	svc, cl, restorer, d1 := rollbackFixture(t, true)
	if _, err := svc.Rollback(context.Background(), "web", d1.ID); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if len(restorer.calls) != 1 {
		t.Fatalf("RestoreEnvironment calls = %d, want 1", len(restorer.calls))
	}
	call := restorer.calls[0]
	if len(call.env) != 1 || call.env["KEEP"] != "old" {
		t.Errorf("restored env = %v, want exactly D1's {KEEP: old}", call.env)
	}
	if len(call.files) != 0 {
		t.Errorf("restored files = %v, want none — D1 had no files snapshot", call.files)
	}
	got := getApp(t, cl, "web")
	if got.Spec.StartCommand != "./server --v1" {
		t.Errorf("start command = %q, want D1's ./server --v1", got.Spec.StartCommand)
	}
	if got.Spec.Image != "web:v1" {
		t.Errorf("image = %q, want web:v1", got.Spec.Image)
	}
	if got.Spec.Tier != "standard" {
		t.Errorf("plan changed to %q — Render keeps the current plan on rollback", got.Spec.Tier)
	}
	// An API rollback (REST, MCP, GraphQL without the flag) leaves auto-deploy alone.
	if !got.Spec.AutoDeploy {
		t.Error("an API rollback turned auto-deploy off; Render does that for dashboard rollbacks only")
	}
}

func TestDashboardRollbackTurnsAutoDeployOff(t *testing.T) {
	svc, cl, _, d1 := rollbackFixture(t, true)
	if _, err := svc.Rollback(context.Background(), "web", d1.ID, RollbackOptions{DisableAutoDeploy: true}); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if getApp(t, cl, "web").Spec.AutoDeploy {
		t.Error("a dashboard rollback must turn auto-deploy off so the next push does not undo it")
	}
}

// No record: the target predates snapshots or was reclaimed. The rollback still
// restores the image — it must not fail — but it must not invent a configuration.
func TestRollbackWithoutARecordRestoresTheImageOnly(t *testing.T) {
	svc, cl, restorer, d1 := rollbackFixture(t, false)
	if _, err := svc.Rollback(context.Background(), "web", d1.ID); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if len(restorer.calls) != 0 {
		t.Errorf("RestoreEnvironment called %d times with no record to restore from", len(restorer.calls))
	}
	got := getApp(t, cl, "web")
	if got.Spec.Image != "web:v1" {
		t.Errorf("image = %q, want web:v1", got.Spec.Image)
	}
	if got.Spec.StartCommand != "./server --v2" {
		t.Errorf("start command = %q, want the current one left alone", got.Spec.StartCommand)
	}
}

// A failed restore fails the rollback before any release opens, rather than
// rolling the old image against a half-restored environment.
func TestRollbackFailsCleanlyWhenTheRestoreFails(t *testing.T) {
	svc, cl, restorer, d1 := rollbackFixture(t, true)
	restorer.err = core.ErrConflict
	if _, err := svc.Rollback(context.Background(), "web", d1.ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("Rollback error = %v, want the restore's error", err)
	}
	if got := getApp(t, cl, "web"); got.Spec.Image != "web:v2" {
		t.Errorf("image = %q after a failed restore; no release should have opened", got.Spec.Image)
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
			svc, cl, _, d1 := rollbackFixture(t, true)
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
